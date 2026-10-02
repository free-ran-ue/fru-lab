package run

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"tester/profile"
)

func runStage(t *testing.T, p *procStage, n int) (cancel func()) {
	t.Helper()
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { p.run(ctx); close(done) }()
	for i := range n {
		p.enqueue(i)
	}
	return func() { stop(); <-done }
}

func TestProcStageHonoursRate(t *testing.T) {
	var mu sync.Mutex
	var starts []time.Time
	var wg sync.WaitGroup
	wg.Add(10)
	p := newProcStage(profile.ProcedureRate{RatePerSec: 50, MaxInFlight: 100}, 10, func(int) bool {
		mu.Lock()
		starts = append(starts, time.Now())
		mu.Unlock()
		wg.Done()
		return false
	})
	stop := runStage(t, p, 10)
	defer stop()
	wg.Wait()
	elapsed := starts[9].Sub(starts[0])
	require.GreaterOrEqual(t, elapsed, 170*time.Millisecond, "10 starts at 50/s span ~180ms")
	require.Less(t, elapsed, 400*time.Millisecond)
}

func TestProcStageCapsInFlight(t *testing.T) {
	var cur, peak atomic.Int32
	var wg sync.WaitGroup
	wg.Add(8)
	p := newProcStage(profile.ProcedureRate{RatePerSec: 100000, MaxInFlight: 2}, 8, func(int) bool {
		n := cur.Add(1)
		for {
			old := peak.Load()
			if n <= old || peak.CompareAndSwap(old, n) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		cur.Add(-1)
		wg.Done()
		return false
	})
	stop := runStage(t, p, 8)
	defer stop()
	wg.Wait()
	require.Equal(t, int32(2), peak.Load())
}

func TestProcStageRequeuesRetriesAtTheBack(t *testing.T) {
	var mu sync.Mutex
	var order []int
	tries := map[int]int{}
	var wg sync.WaitGroup
	wg.Add(6)
	p := newProcStage(profile.ProcedureRate{RatePerSec: 100000, MaxInFlight: 1}, 3, func(ue int) bool {
		mu.Lock()
		defer mu.Unlock()
		order = append(order, ue)
		tries[ue]++
		wg.Done()
		return tries[ue] == 1 // every UE fails once
	})
	stop := runStage(t, p, 3)
	defer stop()
	wg.Wait()
	require.Equal(t, []int{0, 1, 2, 0, 1, 2}, order)
}

func TestProcStageStopLeavesQueuedUesForDrain(t *testing.T) {
	started := make(chan int, 10)
	release := make(chan struct{})
	p := newProcStage(profile.ProcedureRate{RatePerSec: 100000, MaxInFlight: 1}, 5, func(ue int) bool {
		started <- ue
		<-release
		return false
	})
	stop := runStage(t, p, 5)
	<-started // UE 0 is in flight, 1-4 wait for the single slot
	go func() { time.Sleep(20 * time.Millisecond); close(release) }()
	stop() // returns only after UE 0's attempt finished
	require.ElementsMatch(t, []int{1, 2, 3, 4}, p.drain())
}
