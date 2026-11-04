package run

import (
	"context"
	"sync"
)

// rrQueue holds UEs waiting for a stage, one FIFO per gNB, and hands
// them out a gNB at a time in turn: gNB 1's first UE, gNB 2's first, …,
// then each one's second. A plain FIFO would hand out all of gNB 1's UEs
// first, so every procedure in flight would share gNB 1's SCTP
// association and the AMF would serve them one connection at a time. A
// gNB that gets UEs joins the end of the round, so a gNB that comes up
// late waits at most one round, not for the others to finish.
type rrQueue struct {
	mu    sync.Mutex
	group func(ue int) int // the UE's gNB
	fifos map[int][]int
	ring  []int // gNBs with queued UEs, in turn order; ring[head] is next
	head  int
	n     int
	ready chan struct{} // has a value while n > 0 may have changed from 0
}

func newRRQueue(group func(ue int) int) *rrQueue {
	if group == nil {
		group = func(int) int { return 0 }
	}
	return &rrQueue{group: group, fifos: map[int][]int{}, ready: make(chan struct{}, 1)}
}

// push queues ue at the back of its gNB's FIFO; it never blocks.
func (q *rrQueue) push(ue int) {
	g := q.group(ue)
	q.mu.Lock()
	f, queued := q.fifos[g]
	if !queued || len(f) == 0 {
		q.ring = append(q.ring, g)
	}
	q.fifos[g] = append(f, ue)
	q.n++
	q.mu.Unlock()
	select {
	case q.ready <- struct{}{}:
	default:
	}
}

// tryPop takes the next UE in turn, if any.
func (q *rrQueue) tryPop() (int, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.n == 0 {
		return 0, false
	}
	g := q.ring[q.head]
	q.head++
	f := q.fifos[g]
	ue := f[0]
	if len(f) == 1 {
		delete(q.fifos, g) // drop the backing array with the last UE
	} else {
		q.fifos[g] = f[1:]
		q.ring = append(q.ring, g) // its next UE waits for the others' turn
	}
	q.n--
	// reclaim the consumed front of the ring now and then
	if q.head > 1024 && q.head*2 > len(q.ring) {
		q.ring = append([]int(nil), q.ring[q.head:]...)
		q.head = 0
	}
	return ue, true
}

// pop waits for the next UE in turn; ok is false once ctx is done.
func (q *rrQueue) pop(ctx context.Context) (ue int, ok bool) {
	for {
		if ue, ok := q.tryPop(); ok {
			return ue, true
		}
		select {
		case <-ctx.Done():
			return 0, false
		case <-q.ready:
		}
	}
}

// drain empties the queue and returns what was in it.
func (q *rrQueue) drain() []int {
	var out []int
	for {
		ue, ok := q.tryPop()
		if !ok {
			return out
		}
		out = append(out, ue)
	}
}
