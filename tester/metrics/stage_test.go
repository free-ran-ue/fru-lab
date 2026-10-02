package metrics

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time          { return c.t }
func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

func TestStageCountsOutcomesAndTotalTime(t *testing.T) {
	clk := &fakeClock{t: time.Unix(1000, 0)}
	s := NewStage("n2", 4)
	s.now = clk.now

	s.Begin(false)
	s.Begin(false)
	s.Begin(false)
	s.Begin(false)
	clk.advance(10 * time.Millisecond)
	s.Finish(Accepted, 10*time.Millisecond, "")
	s.Finish(Rejected, 0, "misc(4)")
	s.Retrying()
	s.Begin(true)

	snap := s.Snapshot()
	require.Equal(t, int64(4), snap.Attempted)
	require.Equal(t, int64(1), snap.Retries)
	require.Equal(t, int64(2), snap.InFlight)
	require.False(t, snap.Done)
	require.InDelta(t, 10.0, snap.TotalTimeMs, 0.001)

	clk.advance(20 * time.Millisecond)
	s.Finish(TimedOut, 0, "")
	s.Finish(Failed, 0, "sctp: connection refused")
	clk.advance(time.Hour) // after Done, total time must stop growing

	snap = s.Snapshot()
	require.True(t, snap.Done)
	require.Equal(t, int64(0), snap.InFlight)
	require.Equal(t, []int64{1, 1, 1, 1}, []int64{snap.Accepted, snap.Rejected, snap.TimedOut, snap.Failed})
	require.InDelta(t, 30.0, snap.TotalTimeMs, 0.001)
	require.InDelta(t, 10.0, snap.MaxMs, 0.001)
	require.Equal(t, []CauseCount{{"misc(4)", 1}, {"sctp: connection refused", 1}, {"unknown", 1}}, snap.Causes)
}

func TestStageEmptySnapshot(t *testing.T) {
	snap := NewStage("n2", 3).Snapshot()
	require.Equal(t, 0.0, snap.TotalTimeMs)
	require.False(t, snap.Done)
	require.NotNil(t, snap.Causes)
}

func TestStageConcurrentUse(t *testing.T) {
	s := NewStage("n2", 1000)
	var wg sync.WaitGroup
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Begin(false)
			_ = s.Snapshot()
			s.Finish(Accepted, time.Millisecond, "")
		}()
	}
	wg.Wait()
	snap := s.Snapshot()
	require.Equal(t, int64(1000), snap.Accepted)
	require.True(t, snap.Done)
}

func TestStageSkippedItemsCompleteTheStage(t *testing.T) {
	clk := &fakeClock{t: time.Unix(1000, 0)}
	s := NewStage("registration", 3)
	s.now = clk.now
	s.Begin(false)
	clk.advance(5 * time.Millisecond)
	s.Finish(Accepted, 5*time.Millisecond, "")
	s.Skip()
	require.False(t, s.Snapshot().Done)
	s.Skip()
	clk.advance(time.Hour) // a stopped run's stage must not keep counting
	snap := s.Snapshot()
	require.True(t, snap.Done)
	require.Equal(t, int64(2), snap.Skipped)
	require.InDelta(t, 5.0, snap.TotalTimeMs, 0.001)
}

func TestStageAllStartedItemsSkippedAfterRetry(t *testing.T) {
	clk := &fakeClock{t: time.Unix(1000, 0)}
	s := NewStage("pdu", 1)
	s.now = clk.now
	s.Begin(false)
	s.Retrying()
	s.Skip() // stopped while waiting for its retry
	clk.advance(time.Hour)
	snap := s.Snapshot()
	require.True(t, snap.Done)
	require.Equal(t, 0.0, snap.TotalTimeMs)
}
