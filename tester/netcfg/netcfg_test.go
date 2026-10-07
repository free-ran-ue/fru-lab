package netcfg

import (
	"net/netip"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
)

// Needs root: sudo FRU_TESTER_NETLINK=1 go test ./netcfg/
func TestNetlinkAddRemoveOnDummyLink(t *testing.T) {
	if os.Getenv("FRU_TESTER_NETLINK") != "1" {
		t.Skip("set FRU_TESTER_NETLINK=1 and run as root to exercise netlink")
	}
	link := &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: "frutest0"}}
	require.NoError(t, netlink.LinkAdd(link))
	t.Cleanup(func() { _ = netlink.LinkDel(link) })
	require.NoError(t, netlink.LinkSetUp(link)) // routes need an up link

	m := Netlink{}
	names, err := m.Interfaces()
	require.NoError(t, err)
	require.Contains(t, names, "frutest0")

	primary := netip.MustParsePrefix("10.250.0.2/24")
	secondary := netip.MustParsePrefix("10.250.0.3/24")
	require.NoError(t, m.Add("frutest0", primary))
	require.NoError(t, m.Add("frutest0", secondary))

	ips, err := m.HostIPv4s()
	require.NoError(t, err)
	require.Contains(t, ips, primary.Addr())
	require.Contains(t, ips, secondary.Addr())

	// removing the primary may take the secondary with it; the second
	// Remove must still succeed.
	require.NoError(t, m.Remove("frutest0", primary))
	require.NoError(t, m.Remove("frutest0", secondary))
	require.NoError(t, m.Remove("frutest0", secondary))

	require.ErrorContains(t, m.Add("no-such-if0", primary), `interface "no-such-if0"`)

	// routes: add once, recognise our own route, refuse a conflicting one
	require.NoError(t, m.Add("frutest0", primary))
	pool := netip.MustParsePrefix("10.251.0.0/16")
	gw := netip.MustParseAddr("10.250.0.9")
	added, err := m.EnsureRoute("frutest0", pool, gw)
	require.NoError(t, err)
	require.True(t, added)
	added, err = m.EnsureRoute("frutest0", pool, gw)
	require.NoError(t, err)
	require.False(t, added, "an identical route is left alone")
	_, err = m.EnsureRoute("frutest0", pool, netip.MustParseAddr("10.250.0.8"))
	require.ErrorContains(t, err, "already routed via 10.250.0.9")
	require.NoError(t, m.RemoveRoute("frutest0", pool, gw))
	require.NoError(t, m.RemoveRoute("frutest0", pool, gw), "removing twice is fine")
}

// Every host has a loopback, up, with 127.0.0.1/8.
func TestInterfaceDetailsListsLoopback(t *testing.T) {
	ifaces, err := Netlink{}.InterfaceDetails()
	require.NoError(t, err)
	var lo *InterfaceInfo
	for i := range ifaces {
		if ifaces[i].Name == "lo" {
			lo = &ifaces[i]
		}
	}
	require.NotNil(t, lo)
	require.Contains(t, lo.Addresses, "127.0.0.1/8")
	require.Equal(t, 65536, lo.Mtu)
}
