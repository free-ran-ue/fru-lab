package run

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
)

// The sink already on the N6 interface with the same prefix is used as it
// is: not added, and not removed at Stop.
func TestRunKeepsASinkTheHostAlreadyHas(t *testing.T) {
	addrs := &fakeAddrs{hostAddrs: []hostAddr{{iface: "eth-n6", prefix: netip.MustParsePrefix("10.0.3.2/24")}}}
	log := runAndStop(t, addrs)
	require.NotContains(t, log, "add eth-n6 10.0.3.2/24")
	require.NotContains(t, log, "remove eth-n6 10.0.3.2/24")
}

// The same IP with another prefix on the N6 interface is replaced for the
// run, before any gNB IP goes on, and put back at Stop.
func TestRunReplacesASinkWithAnotherPrefixAndPutsItBack(t *testing.T) {
	addrs := &fakeAddrs{hostAddrs: []hostAddr{{iface: "eth-n6", prefix: netip.MustParsePrefix("10.0.3.2/32")}}}
	log := runAndStop(t, addrs)
	require.Equal(t, []string{"remove eth-n6 10.0.3.2/32", "add eth-n6 10.0.3.2/24"}, log[:2], "replaced first")
	require.Equal(t, []string{"remove eth-n6 10.0.3.2/24", "add eth-n6 10.0.3.2/32"}, log[len(log)-2:], "put back last")
}

// The IP with another prefix on another interface is not taken away.
func TestRunRefusesASinkOnAnotherInterface(t *testing.T) {
	addrs := &fakeAddrs{hostAddrs: []hostAddr{{iface: "eth0", prefix: netip.MustParsePrefix("10.0.3.2/16")}}}
	dialer := newFakeDialer(func(string, int) (func() ([]byte, error), error) { return accept(t), nil })
	c := newTestController(addrs, dialer)
	_, err := c.Start(testProfile())
	require.NoError(t, err)
	snap := waitState(t, c, StateFailed)
	require.Contains(t, snap.Error, "configure N6 sink: 10.0.3.2 is already on eth0 as 10.0.3.2/16")
	_, log := addrs.snapshot()
	require.Empty(t, log, "nothing changed on the host")
}

// A ping test follows the same rule and restores the host's address.
func TestPingReplacesAndRestoresAnAddressWithAnotherPrefix(t *testing.T) {
	addrs := &fakeAddrs{hostAddrs: []hostAddr{{iface: "eth-n6", prefix: netip.MustParsePrefix("10.0.3.2/32")}}}
	res, err := pingController(addrs, &fakePinger{}).Ping(testProfile(), "n6")
	require.NoError(t, err)
	require.True(t, res.Added)
	_, log := addrs.snapshot()
	require.Equal(t, []string{"remove eth-n6 10.0.3.2/32", "add eth-n6 10.0.3.2/24", "remove eth-n6 10.0.3.2/24", "add eth-n6 10.0.3.2/32"}, log)
}

// runAndStop runs testProfile until every gNB is up, stops it and returns
// the address log.
func runAndStop(t *testing.T, addrs *fakeAddrs) []string {
	t.Helper()
	dialer := newFakeDialer(func(string, int) (func() ([]byte, error), error) { return accept(t), nil })
	c := newTestController(addrs, dialer)
	_, err := c.Start(testProfile())
	require.NoError(t, err)
	waitState(t, c, StateRunning)
	_, err = c.Stop()
	require.NoError(t, err)
	waitState(t, c, StateStopped)
	_, log := addrs.snapshot()
	return log
}

// Two testers on one host share the N6 sink and the UE pool route and use
// their own gNB IPs on the same interfaces. Stopping one takes off only
// its own gNB IPs: the sink and the route stay for the other, and go with
// the last tester to stop.
func TestTwoTestersOnOneHostEachRemoveOnlyTheirOwn(t *testing.T) {
	host := &fakeAddrs{shared: true}
	dialer := newFakeDialer(func(string, int) (func() ([]byte, error), error) { return accept(t), nil })
	a, b := newTestController(host, dialer), newTestController(host, dialer)
	pa, pb := testProfile(), testProfile()
	pb.Network.N2.StartIP, pb.Network.N3.StartIP = "10.0.1.50", "10.0.2.50"

	_, err := a.Start(pa)
	require.NoError(t, err)
	waitState(t, a, StateRunning)
	_, err = b.Start(pb)
	require.NoError(t, err)
	waitState(t, b, StateRunning)

	_, err = a.Stop()
	require.NoError(t, err)
	snap := waitState(t, a, StateStopped)
	require.Empty(t, snap.Error)
	present, log := host.snapshot()
	require.Contains(t, present, netip.MustParsePrefix("10.0.3.2/24"), "the sink stays for B")
	for _, ip := range []string{"10.0.1.50/24", "10.0.1.51/24", "10.0.1.52/24", "10.0.2.50/24", "10.0.2.51/24", "10.0.2.52/24"} {
		require.Contains(t, present, netip.MustParsePrefix(ip), "B's gNB IPs stay")
	}
	for _, ip := range []string{"10.0.1.10/24", "10.0.2.10/24"} {
		require.NotContains(t, present, netip.MustParsePrefix(ip), "A's gNB IPs go")
	}
	require.NotContains(t, log, "unroute 10.60.0.0/16", "the route stays for B")
	require.Equal(t, 1, countOf(log, "add eth-n6 10.0.3.2/24"), "B shares A's sink instead of adding it again")

	_, err = b.Stop()
	require.NoError(t, err)
	snap = waitState(t, b, StateStopped)
	require.Empty(t, snap.Error)
	present, log = host.snapshot()
	require.Empty(t, present, "the last tester takes everything off")
	require.Equal(t, 1, countOf(log, "remove eth-n6 10.0.3.2/24"))
	require.Equal(t, 1, countOf(log, "unroute 10.60.0.0/16"))
	require.Contains(t, host.promoted, "eth-n2", "promote_secondaries is turned on before any address goes on")
}

// A sink another tester uses with another prefix length is not replaced.
func TestSinkInUseByAnotherTesterWithAnotherPrefixIsRefused(t *testing.T) {
	host := &fakeAddrs{shared: true}
	release, _ := host.Claim(addrKey("eth-n6", netip.MustParsePrefix("10.0.3.2/32")), "-")
	defer release()
	host.hostAddrs = []hostAddr{{iface: "eth-n6", prefix: netip.MustParsePrefix("10.0.3.2/32")}}
	_, err := placeAddr(host, "eth-n6", netip.MustParsePrefix("10.0.3.2/24"))
	require.ErrorContains(t, err, "in use by another tester on eth-n6 as 10.0.3.2/32")
	_, log := host.snapshot()
	require.Empty(t, log)
}

// The route the host had before any tester is used and left there.
func TestHostRouteIsLeftAlone(t *testing.T) {
	host := &fakeAddrs{shared: true, routes: map[string]bool{"eth-n6 10.60.0.0/16 10.0.3.1": true}}
	_, claimed, err := placeRoute(host, "eth-n6", netip.MustParsePrefix("10.60.0.0/16"), netip.MustParseAddr("10.0.3.1"))
	require.NoError(t, err)
	require.False(t, claimed)
}

func countOf(log []string, line string) int {
	n := 0
	for _, l := range log {
		if l == line {
			n++
		}
	}
	return n
}
