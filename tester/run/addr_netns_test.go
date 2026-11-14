package run

import (
	"net/netip"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"tester/netcfg"
)

// On a real kernel (FRU_NETNS_IFACE names an interface with
// promote_secondaries off in a throwaway network namespace, as root), two
// testers share a subnet: stopping the first, whose sink is the subnet's
// primary address, leaves the second's sink, gNB IPs and route in place.
func TestNetnsStoppingOneTesterKeepsTheOthersAddresses(t *testing.T) {
	iface := os.Getenv("FRU_NETNS_IFACE")
	if iface == "" {
		t.Skip("set FRU_NETNS_IFACE in a throwaway network namespace")
	}
	m := netcfg.Netlink{}
	sink, pool, gw := netip.MustParsePrefix("10.0.3.100/24"), netip.MustParsePrefix("10.60.0.0/16"), netip.MustParseAddr("10.0.3.1")
	logf := func(f string, a ...any) { t.Logf(f, a...) }

	// tester A: sink first (the primary), then a gNB IP, then the route
	require.NoError(t, ensurePromote(m, logf, iface))
	aSink, err := placeAddr(m, iface, sink)
	require.NoError(t, err)
	require.NoError(t, m.Add(iface, netip.MustParsePrefix("10.0.3.11/24")))
	aRoute, ok, err := placeRoute(m, iface, pool, gw)
	require.NoError(t, err)
	require.True(t, ok)

	// tester B: the same sink and route, its own gNB IPs
	require.NoError(t, ensurePromote(m, logf, iface))
	bSink, err := placeAddr(m, iface, sink)
	require.NoError(t, err)
	require.NotNil(t, bSink.release, "B shares A's sink")
	for _, ip := range []string{"10.0.3.21/24", "10.0.3.22/24"} {
		require.NoError(t, m.Add(iface, netip.MustParsePrefix(ip)))
	}
	bRoute, ok, err := placeRoute(m, iface, pool, gw)
	require.NoError(t, err)
	require.True(t, ok)

	// A stops, in removeIPs' order
	require.NoError(t, aRoute.undo(m))
	require.NoError(t, (addrChange{iface: iface, added: netip.MustParsePrefix("10.0.3.11/24")}).undo(m))
	require.NoError(t, aSink.undo(m))
	for _, ip := range []string{"10.0.3.100", "10.0.3.21", "10.0.3.22"} {
		_, _, found, err := m.FindAddr(netip.MustParseAddr(ip))
		require.NoError(t, err)
		require.True(t, found, "%s must stay for B", ip)
	}
	_, _, found, _ := m.FindAddr(netip.MustParseAddr("10.0.3.11"))
	require.False(t, found, "A's gNB IP goes")
	added, err := m.EnsureRoute(iface, pool, gw)
	require.NoError(t, err)
	require.False(t, added, "the route stays for B")

	// B stops: everything goes
	require.NoError(t, bRoute.undo(m))
	added, err = m.EnsureRoute(iface, pool, gw)
	require.NoError(t, err)
	require.True(t, added, "the last tester took the route off")
	require.NoError(t, m.RemoveRoute(iface, pool, gw))
	for _, ip := range []string{"10.0.3.22/24", "10.0.3.21/24"} {
		require.NoError(t, (addrChange{iface: iface, added: netip.MustParsePrefix(ip)}).undo(m))
	}
	require.NoError(t, bSink.undo(m))
	_, _, found, _ = m.FindAddr(netip.MustParseAddr("10.0.3.100"))
	require.False(t, found, "the last tester takes the sink off")
}
