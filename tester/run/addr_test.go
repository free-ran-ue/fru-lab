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
