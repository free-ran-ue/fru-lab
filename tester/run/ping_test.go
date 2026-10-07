package run

import (
	"errors"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"tester/profile"
)

// fakePinger answers with replies (and err), records each call, and can
// hold a ping until release is closed.
type fakePinger struct {
	mu      sync.Mutex
	replies []time.Duration
	err     error
	calls   []string
	started chan struct{}
	release chan struct{}
}

func (p *fakePinger) Ping(src, dst netip.Addr, count int, _ time.Duration) ([]time.Duration, error) {
	p.mu.Lock()
	p.calls = append(p.calls, src.String()+" -> "+dst.String())
	p.mu.Unlock()
	if p.started != nil {
		close(p.started)
		<-p.release
	}
	return p.replies, p.err
}

func pingController(addrs *fakeAddrs, pinger *fakePinger) *Controller {
	c := newTestController(addrs, newFakeDialer(nil))
	c.deps.Pinger = pinger
	return c
}

// Each network is tested from the address a run would add, which is put
// on the interface for the test and removed afterwards.
func TestPingAddsTheRunsAddressPingsTheCoreAndRemovesIt(t *testing.T) {
	for plane, want := range map[string]struct{ iface, source, target string }{
		profile.PlaneN2: {"eth-n2", "10.0.1.10/24", "10.0.1.1"},
		profile.PlaneN3: {"eth-n3", "10.0.2.10/24", "10.0.2.1"},
		profile.PlaneN6: {"eth-n6", "10.0.3.2/24", "10.0.3.1"},
	} {
		addrs := &fakeAddrs{}
		pinger := &fakePinger{replies: []time.Duration{time.Millisecond, 2 * time.Millisecond, 3 * time.Millisecond}}
		res, err := pingController(addrs, pinger).Ping(testProfile(), plane)
		require.NoError(t, err, plane)
		require.Equal(t, PingResult{Plane: plane, Interface: want.iface, Source: want.source, Target: want.target,
			Added: true, Sent: 3, Received: 3, RttMs: []float64{1, 2, 3}}, res, plane)
		present, log := addrs.snapshot()
		require.Empty(t, present, "%s: the address is removed again", plane)
		require.Equal(t, []string{"add " + want.iface + " " + want.source, "remove " + want.iface + " " + want.source}, log, plane)
		require.Equal(t, []string{netip.MustParsePrefix(want.source).Addr().String() + " -> " + want.target}, pinger.calls, plane)
	}
}

func TestPingUsesAnAddressTheHostAlreadyHasAsItIs(t *testing.T) {
	addrs := &fakeAddrs{host: []netip.Addr{netip.MustParseAddr("10.0.3.2")}}
	res, err := pingController(addrs, &fakePinger{}).Ping(testProfile(), profile.PlaneN6)
	require.NoError(t, err)
	require.False(t, res.Added)
	require.Zero(t, res.Received)
	_, log := addrs.snapshot()
	require.Empty(t, log, "nothing added or removed")
}

func TestPingReportsAFailedAddOrSend(t *testing.T) {
	res, err := pingController(&fakeAddrs{failAdd: 1}, &fakePinger{}).Ping(testProfile(), profile.PlaneN2)
	require.NoError(t, err)
	require.Equal(t, "could not add 10.0.1.10/24 to eth-n2: operation not permitted", res.Error)
	require.Zero(t, res.Sent)

	addrs := &fakeAddrs{}
	res, err = pingController(addrs, &fakePinger{err: errors.New("send to 10.0.1.1: no route to host")}).Ping(testProfile(), profile.PlaneN2)
	require.NoError(t, err)
	require.Equal(t, "send to 10.0.1.1: no route to host", res.Error)
	present, _ := addrs.snapshot()
	require.Empty(t, present, "removed even when the ping failed")
}

func TestPingChecksOnlyThatNetworksFields(t *testing.T) {
	p := testProfile()
	p.Ue.Key = "broken"
	_, err := pingController(&fakeAddrs{}, &fakePinger{}).Ping(p, profile.PlaneN2)
	require.NoError(t, err, "another card's error does not stop the test")

	p.Network.N2.Interface = "ens199"
	_, err = pingController(&fakeAddrs{}, &fakePinger{}).Ping(p, profile.PlaneN2)
	var verr *profile.ValidationError
	require.ErrorAs(t, err, &verr)
	require.Equal(t, []profile.FieldError{{Field: "network.n2.interface", Message: `no interface named "ens199" on this host`}}, verr.Errors)

	_, err = pingController(&fakeAddrs{}, &fakePinger{}).Ping(p, "n4")
	require.ErrorIs(t, err, profile.ErrUnknownPlane)
}

// A test and a run both add addresses to the host, so they exclude each
// other.
func TestPingAndRunExcludeEachOther(t *testing.T) {
	pinger := &fakePinger{started: make(chan struct{}), release: make(chan struct{})}
	c := pingController(&fakeAddrs{}, pinger)
	done := make(chan struct{})
	go func() {
		_, _ = c.Ping(testProfile(), profile.PlaneN2)
		close(done)
	}()
	<-pinger.started
	_, err := c.Start(testProfile())
	require.ErrorIs(t, err, ErrPingActive)
	_, err = c.Ping(testProfile(), profile.PlaneN3)
	require.ErrorIs(t, err, ErrPingActive)
	close(pinger.release)
	<-done

	_, err = c.Start(testProfile())
	require.NoError(t, err)
	_, err = c.Ping(testProfile(), profile.PlaneN2)
	require.ErrorIs(t, err, ErrRunActive)
	c.Shutdown()
}
