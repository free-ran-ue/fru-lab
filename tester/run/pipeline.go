package run

import (
	"context"
	"sync"
	"time"

	"tester/profile"
)

// procStage paces one per-UE stage (registration or PDU): it starts at
// most RatePerSec attempts per second and keeps at most MaxInFlight
// running at once. Separate knobs, because a semaphore alone lets a fast
// core run far above the configured rate, and a rate alone lets a slow
// core pile up unbounded in-flight requests (design section 5).
type procStage struct {
	interval    time.Duration
	maxInFlight int
	queue       *rrQueue // UE indexes, taken a gNB at a time in turn
	attempt     func(ue int) (retry bool)
	inFlight    sync.WaitGroup
}

// newProcStage paces attempt; gnbOf is a UE's gNB, which the queue takes
// turns over (nil: one group, plain FIFO order).
func newProcStage(r profile.ProcedureRate, gnbOf func(ue int) int, attempt func(ue int) (retry bool)) *procStage {
	return &procStage{
		interval:    time.Second / time.Duration(r.RatePerSec),
		maxInFlight: r.MaxInFlight,
		queue:       newRRQueue(gnbOf),
		attempt:     attempt,
	}
}

// enqueue never blocks.
func (p *procStage) enqueue(ue int) { p.queue.push(ue) }

// run dispatches until ctx is done, then waits for in-flight attempts.
// A failed attempt the callback wants retried goes to the back of the
// queue (design N3). Whatever is still queued afterwards is returned by
// drain.
func (p *procStage) run(ctx context.Context) {
	sem := make(chan struct{}, p.maxInFlight)
	next := time.Now()
	for {
		ue, ok := p.queue.pop(ctx)
		if !ok {
			p.inFlight.Wait()
			return
		}
		// token bucket of depth 1: one start every interval
		if wait := time.Until(next); wait > 0 {
			t := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				t.Stop()
				p.queue.push(ue)
				p.inFlight.Wait()
				return
			case <-t.C:
			}
		}
		next = maxTime(next, time.Now()).Add(p.interval)
		select {
		case <-ctx.Done():
			p.queue.push(ue)
			p.inFlight.Wait()
			return
		case sem <- struct{}{}:
		}
		p.inFlight.Add(1)
		go func() {
			defer func() { <-sem; p.inFlight.Done() }()
			// requeue a wanted retry; if we are stopping, drain finds it
			if p.attempt(ue) {
				p.queue.push(ue)
			}
		}()
	}
}

// drain returns the UEs still queued; call after run has returned.
func (p *procStage) drain() []int { return p.queue.drain() }

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
