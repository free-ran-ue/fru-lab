package metrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHistogramQuantilesWithinResolution(t *testing.T) {
	var h histogram
	for i := 1; i <= 1000; i++ {
		h.record(time.Duration(i) * time.Millisecond)
	}
	require.InEpsilon(t, 500.0, ms(h.quantile(0.50)), 0.05)
	require.InEpsilon(t, 950.0, ms(h.quantile(0.95)), 0.05)
	require.InEpsilon(t, 990.0, ms(h.quantile(0.99)), 0.05)
	require.Equal(t, time.Second, h.quantile(1.0))
	require.Equal(t, 500500*time.Microsecond, h.mean())
}

func TestHistogramEdges(t *testing.T) {
	var h histogram
	require.Equal(t, time.Duration(0), h.quantile(0.5))
	require.Equal(t, time.Duration(0), h.mean())
	h.record(0)
	h.record(time.Hour * 24 * 365)
	require.Equal(t, time.Hour*24*365, h.max)
	// sub-microsecond samples share bucket 0, reported as its 1µs bound
	require.Equal(t, time.Microsecond, h.quantile(0.5))
}

func TestLatencySnapshot(t *testing.T) {
	var l Latency
	require.Equal(t, LatencySnapshot{}, l.Snapshot())
	l.Record(2 * time.Millisecond)
	l.Record(4 * time.Millisecond)
	s := l.Snapshot()
	require.Equal(t, uint64(2), s.Count)
	require.InDelta(t, 3.0, s.AvgMs, 0.001)
	require.InDelta(t, 4.0, s.MaxMs, 0.001)
}
