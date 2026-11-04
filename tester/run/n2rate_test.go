package run

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"tester/profile"
)

// Rates.N2.MaxInFlight caps how many gNBs connect at once.
func TestN2MaxInFlightCapsConcurrentSetups(t *testing.T) {
	var mu sync.Mutex
	cur, peak := 0, 0
	dialer := newFakeDialer(func(string, int) (func() ([]byte, error), error) {
		mu.Lock()
		cur++
		peak = max(peak, cur)
		mu.Unlock()
		time.Sleep(30 * time.Millisecond)
		mu.Lock()
		cur--
		mu.Unlock()
		return accept(t), nil
	})
	c := newTestController(&fakeAddrs{}, dialer)
	p := testProfile()
	p.Scale = profile.Scale{GnbCount: 6, UeCount: 12}
	p.Rates.N2.MaxInFlight = 2
	_, err := c.Start(p)
	require.NoError(t, err)
	snap := waitState(t, c, StateRunning)
	require.Equal(t, int64(6), snap.N2.Accepted)
	mu.Lock()
	require.Equal(t, 2, peak)
	mu.Unlock()
	_, _ = c.Stop()
	waitState(t, c, StateStopped)
}

// Rates.N2.RatePerSec spaces the gNBs' starts: 5 at 20/s start over at
// least 4 × 50 ms.
func TestN2RatePerSecSpacesStarts(t *testing.T) {
	var mu sync.Mutex
	var starts []time.Time
	dialer := newFakeDialer(func(string, int) (func() ([]byte, error), error) {
		mu.Lock()
		starts = append(starts, time.Now())
		mu.Unlock()
		return accept(t), nil
	})
	c := newTestController(&fakeAddrs{}, dialer)
	p := testProfile()
	p.Scale = profile.Scale{GnbCount: 5, UeCount: 10}
	p.Rates.N2.RatePerSec = 20
	_, err := c.Start(p)
	require.NoError(t, err)
	snap := waitState(t, c, StateRunning)
	require.Equal(t, int64(5), snap.N2.Accepted)
	mu.Lock()
	require.Len(t, starts, 5)
	require.GreaterOrEqual(t, starts[4].Sub(starts[0]), 190*time.Millisecond)
	mu.Unlock()
	_, _ = c.Stop()
	waitState(t, c, StateStopped)
}
