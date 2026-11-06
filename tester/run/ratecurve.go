package run

// curvePoint is one point of a per-second rate curve: plus sums two
// points' rates and takes the later one's time, div divides the rates.
type curvePoint[P any] interface {
	plus(P) P
	div(float64) P
}

// rateCurve keeps a whole run's per-second curve in at most max points:
// once full, neighbours are averaged in pairs and each point covers twice
// as many seconds (width) as before. The data plane's curve does the same.
type rateCurve[P curvePoint[P]] struct {
	max, width int
	done       []P
	sum        P
	filled     int
}

func newRateCurve[P curvePoint[P]](max int) rateCurve[P] {
	return rateCurve[P]{max: max, width: 1}
}

func (h *rateCurve[P]) add(p P) {
	h.sum = h.sum.plus(p)
	h.filled++
	if h.filled < h.width {
		return
	}
	h.done = append(h.done, h.average())
	var zero P
	h.sum, h.filled = zero, 0
	if len(h.done) < h.max {
		return
	}
	for i := range len(h.done) / 2 {
		h.done[i] = h.done[2*i].plus(h.done[2*i+1]).div(2)
	}
	h.done = h.done[:len(h.done)/2]
	h.width *= 2
}

func (h *rateCurve[P]) average() P { return h.sum.div(float64(h.filled)) }

// points is the curve so far, a partly filled last point included.
func (h *rateCurve[P]) points() []P {
	out := append(make([]P, 0, len(h.done)+1), h.done...)
	if h.filled > 0 {
		out = append(out, h.average())
	}
	return out
}
