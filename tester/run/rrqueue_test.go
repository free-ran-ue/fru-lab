package run

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"tester/profile"
)

// UEs 0..11: gNB = ue / 4, so gNB 0 has 0–3, gNB 1 has 4–7, gNB 2 has 8–11.
func byFour(ue int) int { return ue / 4 }

func TestRRQueueTakesGnbsInTurn(t *testing.T) {
	q := newRRQueue(byFour)
	for ue := range 12 {
		q.push(ue) // queued gNB by gNB, as startGnb does
	}
	require.Equal(t, []int{0, 4, 8, 1, 5, 9, 2, 6, 10, 3, 7, 11}, q.drain())
}

func TestRRQueueLateGnbJoinsTheNextRound(t *testing.T) {
	q := newRRQueue(byFour)
	for ue := range 4 {
		q.push(ue) // gNB 0 up first
	}
	first, _ := q.tryPop()
	require.Equal(t, 0, first)
	for ue := 4; ue < 8; ue++ {
		q.push(ue) // gNB 1 comes up while gNB 0 is under way
	}
	require.Equal(t, []int{1, 4, 2, 5, 3, 6, 7}, q.drain(), "gNB 1 does not wait for gNB 0 to finish")
}

func TestRRQueueRetryGoesToTheBackOfItsGnb(t *testing.T) {
	q := newRRQueue(byFour)
	for _, ue := range []int{0, 1, 4, 5} {
		q.push(ue)
	}
	ue, _ := q.tryPop()
	require.Equal(t, 0, ue)
	q.push(0) // UE 0 failed and is retried
	require.Equal(t, []int{4, 1, 5, 0}, q.drain())
}

func TestRRQueuePopWaitsForAPush(t *testing.T) {
	q := newRRQueue(nil)
	go func() { time.Sleep(20 * time.Millisecond); q.push(7) }()
	ue, ok := q.pop(context.Background())
	require.True(t, ok)
	require.Equal(t, 7, ue)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, ok = q.pop(ctx)
	require.False(t, ok)
}

// With MaxInFlight 4 and 4 gNBs of 25 UEs queued gNB by gNB, the 4
// procedures in flight are on 4 different gNBs, not all on gNB 0.
func TestProcStageSpreadsInFlightAcrossGnbs(t *testing.T) {
	const gnbs, perGnb = 4, 25
	gnbOf := func(ue int) int { return ue / perGnb }
	var mu sync.Mutex
	inFlight := map[int]int{} // gNB → procedures in flight
	worst := 0                // most in flight on one gNB at once
	done := make(chan struct{})
	finished := 0
	p := newProcStage(profile.ProcedureRate{RatePerSec: 100000, MaxInFlight: gnbs}, gnbOf, func(ue int) bool {
		g := gnbOf(ue)
		mu.Lock()
		inFlight[g]++
		worst = max(worst, inFlight[g])
		mu.Unlock()
		time.Sleep(5 * time.Millisecond)
		mu.Lock()
		inFlight[g]--
		finished++
		if finished == gnbs*perGnb {
			close(done)
		}
		mu.Unlock()
		return false
	})
	for ue := range gnbs * perGnb {
		p.enqueue(ue)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go p.run(ctx)
	<-done
	cancel()
	require.LessOrEqual(t, worst, 2, "in flight should spread over the gNBs (FIFO order put all 4 on one)")
}
