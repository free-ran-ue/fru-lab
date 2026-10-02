package dataplane

// history keeps a run's whole rate curve, from its first second, in at
// most max points: each time it fills up, adjacent pairs are averaged and
// every later point covers twice as many seconds.
type history struct {
	max    int // even
	width  int // 1-second samples per point
	done   []Point
	sum    Point // the point being filled; T is its latest sample
	filled int
}

func newHistory(max int) *history { return &history{max: max, width: 1} }

func (h *history) add(p Point) {
	h.sum = Point{T: p.T, UlTxBps: h.sum.UlTxBps + p.UlTxBps, UlRxBps: h.sum.UlRxBps + p.UlRxBps,
		DlTxBps: h.sum.DlTxBps + p.DlTxBps, DlRxBps: h.sum.DlRxBps + p.DlRxBps}
	h.filled++
	if h.filled < h.width {
		return
	}
	h.done = append(h.done, h.average())
	h.sum, h.filled = Point{}, 0
	if len(h.done) < h.max {
		return
	}
	for i := range len(h.done) / 2 {
		a, b := h.done[2*i], h.done[2*i+1]
		h.done[i] = Point{T: b.T, UlTxBps: (a.UlTxBps + b.UlTxBps) / 2, UlRxBps: (a.UlRxBps + b.UlRxBps) / 2,
			DlTxBps: (a.DlTxBps + b.DlTxBps) / 2, DlRxBps: (a.DlRxBps + b.DlRxBps) / 2}
	}
	h.done = h.done[:len(h.done)/2]
	h.width *= 2
}

func (h *history) average() Point {
	n := float64(h.filled)
	return Point{T: h.sum.T, UlTxBps: h.sum.UlTxBps / n, UlRxBps: h.sum.UlRxBps / n,
		DlTxBps: h.sum.DlTxBps / n, DlRxBps: h.sum.DlRxBps / n}
}

// points is a copy of the curve; a point still being filled is included
// so the curve always reaches the latest second.
func (h *history) points() []Point {
	out := append(make([]Point, 0, len(h.done)+1), h.done...)
	if h.filled > 0 {
		out = append(out, h.average())
	}
	return out
}
