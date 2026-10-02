package metrics

import (
	"sync"
	"time"
)

// Latency is a concurrency-safe latency histogram, for the data plane's
// per-packet one-way delays.
type Latency struct {
	mu sync.Mutex
	h  histogram
}

func (l *Latency) Record(d time.Duration) {
	l.mu.Lock()
	l.h.record(d)
	l.mu.Unlock()
}

// LatencySnapshot is in milliseconds.
type LatencySnapshot struct {
	Count uint64  `json:"count"`
	AvgMs float64 `json:"avgMs"`
	P50Ms float64 `json:"p50Ms"`
	P99Ms float64 `json:"p99Ms"`
	MaxMs float64 `json:"maxMs"`
}

func (l *Latency) Snapshot() LatencySnapshot {
	l.mu.Lock()
	defer l.mu.Unlock()
	return LatencySnapshot{
		Count: l.h.total, AvgMs: ms(l.h.mean()),
		P50Ms: ms(l.h.quantile(0.50)), P99Ms: ms(l.h.quantile(0.99)), MaxMs: ms(l.h.max),
	}
}
