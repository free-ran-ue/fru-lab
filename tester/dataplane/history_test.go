package dataplane

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// rate makes a 1-second point at t whose rates are all t, so an averaged
// point shows which seconds went into it.
func rate(t float64) Point { return Point{T: t, UlTxBps: t, UlRxBps: t, DlTxBps: t, DlRxBps: t} }

func TestHistoryKeepsEverySecondWhileItFits(t *testing.T) {
	h := newHistory(8)
	for s := 1; s <= 5; s++ {
		h.add(rate(float64(s)))
	}
	require.Equal(t, []Point{rate(1), rate(2), rate(3), rate(4), rate(5)}, h.points())
}

func TestHistoryHalvesItsResolutionToKeepTheWholeRun(t *testing.T) {
	h := newHistory(8)
	for s := 1; s <= 8; s++ { // the 8th point fills it: pairs merge
		h.add(rate(float64(s)))
	}
	require.Equal(t, []Point{
		{T: 2, UlTxBps: 1.5, UlRxBps: 1.5, DlTxBps: 1.5, DlRxBps: 1.5},
		{T: 4, UlTxBps: 3.5, UlRxBps: 3.5, DlTxBps: 3.5, DlRxBps: 3.5},
		{T: 6, UlTxBps: 5.5, UlRxBps: 5.5, DlTxBps: 5.5, DlRxBps: 5.5},
		{T: 8, UlTxBps: 7.5, UlRxBps: 7.5, DlTxBps: 7.5, DlRxBps: 7.5},
	}, h.points())

	h.add(rate(9)) // now 2 seconds per point: shown as a partial point
	require.Len(t, h.points(), 5)
	require.Equal(t, rate(9), h.points()[4])
	h.add(rate(10))
	require.Equal(t, Point{T: 10, UlTxBps: 9.5, UlRxBps: 9.5, DlTxBps: 9.5, DlRxBps: 9.5}, h.points()[4])
}

func TestHistoryStaysBoundedOverALongRun(t *testing.T) {
	h := newHistory(600)
	for s := 1; s <= 24*3600; s++ {
		h.add(rate(float64(s)))
	}
	p := h.points()
	require.LessOrEqual(t, len(p), 600)
	require.Greater(t, len(p), 300)
	require.Equal(t, float64(24*3600), p[len(p)-1].T, "the curve reaches now")
	require.Less(t, p[0].T, float64(300), "and still starts at the beginning of the run")
}
