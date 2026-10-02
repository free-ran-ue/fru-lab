package fakecore

import (
	"io"
	"sync"
)

// Pipe returns two connected ends that keep SCTP's message boundaries
// and, unlike net.Pipe, buffer writes, so two sides that both write
// before reading (as a gNB and an AMF do) cannot deadlock.
func Pipe() (*PipeEnd, *PipeEnd) {
	ab, ba := newQueue(), newQueue()
	return &PipeEnd{in: ba, out: ab}, &PipeEnd{in: ab, out: ba}
}

type PipeEnd struct {
	in, out *queue
}

func (p *PipeEnd) Read(b []byte) (int, error)  { return p.in.pop(b) }
func (p *PipeEnd) Write(b []byte) (int, error) { return p.out.push(b) }

// Close ends both directions: the peer's reads return io.EOF.
func (p *PipeEnd) Close() error {
	p.in.close()
	p.out.close()
	return nil
}

type queue struct {
	mu     sync.Mutex
	cond   *sync.Cond
	msgs   [][]byte
	closed bool
}

func newQueue() *queue {
	q := &queue{}
	q.cond = sync.NewCond(&q.mu)
	return q
}

func (q *queue) push(b []byte) (int, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return 0, io.ErrClosedPipe
	}
	q.msgs = append(q.msgs, append([]byte(nil), b...))
	q.cond.Signal()
	return len(b), nil
}

func (q *queue) pop(b []byte) (int, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.msgs) == 0 && !q.closed {
		q.cond.Wait()
	}
	if len(q.msgs) == 0 {
		return 0, io.EOF
	}
	m := q.msgs[0]
	q.msgs = q.msgs[1:]
	return copy(b, m), nil
}

func (q *queue) close() {
	q.mu.Lock()
	q.closed = true
	q.cond.Broadcast()
	q.mu.Unlock()
}
