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
	queue       chan int // UE indexes; each UE is queued at most once at a time
	attempt     func(ue int) (retry bool)
	inFlight    sync.WaitGroup
}

func newProcStage(r profile.ProcedureRate, ueCount int, attempt func(ue int) (retry bool)) *procStage {
	return &procStage{
		interval:    time.Second / time.Duration(r.RatePerSec),
		maxInFlight: r.MaxInFlight,
		queue:       make(chan int, ueCount),
		attempt:     attempt,
	}
}

// enqueue never blocks: the queue holds every UE at once.
func (p *procStage) enqueue(ue int) { p.queue <- ue }

// run dispatches until ctx is done, then waits for in-flight attempts.
// A failed attempt the callback wants retried goes to the back of the
// queue (design N3). Whatever is still queued afterwards is returned by
// drain.
func (p *procStage) run(ctx context.Context) {
	sem := make(chan struct{}, p.maxInFlight)
	next := time.Now()
	for {
		var ue int
		select {
		case <-ctx.Done():
			p.inFlight.Wait()
			return
		case ue = <-p.queue:
		}
		// token bucket of depth 1: one start every interval
		if wait := time.Until(next); wait > 0 {
			t := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				t.Stop()
				p.queue <- ue
				p.inFlight.Wait()
				return
			case <-t.C:
			}
		}
		next = maxTime(next, time.Now()).Add(p.interval)
		select {
		case <-ctx.Done():
			p.queue <- ue
			p.inFlight.Wait()
			return
		case sem <- struct{}{}:
		}
		p.inFlight.Add(1)
		go func() {
			defer func() { <-sem; p.inFlight.Done() }()
			// requeue a wanted retry; if we are stopping, drain finds it
			if p.attempt(ue) {
				p.queue <- ue
			}
		}()
	}
}

// drain returns the UEs still queued; call after run has returned.
func (p *procStage) drain() []int {
	var out []int
	for {
		select {
		case ue := <-p.queue:
			out = append(out, ue)
		default:
			return out
		}
	}
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
