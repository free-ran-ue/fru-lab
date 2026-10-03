package metrics

import (
	"sync"
	"time"
)

// Latency is a concurrency-safe latency histogram, for the data plane's
// per-packet one-way delays. The data plane gives every reader its own
// and records a whole batch under one lock, so readers never wait on
// each other; MergedSnapshot combines them.
type Latency struct {
	mu sync.Mutex
	h  histogram
}

func (l *Latency) Record(d time.Duration) {
	l.mu.Lock()
	l.h.record(d)
	l.mu.Unlock()
}

// RecordAll records ds under one lock.
func (l *Latency) RecordAll(ds []time.Duration) {
	if len(ds) == 0 {
		return
	}
	l.mu.Lock()
	for _, d := range ds {
		l.h.record(d)
	}
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
	return l.h.snapshot()
}

// MergedSnapshot is the snapshot of all of ls recorded into one histogram.
func MergedSnapshot(ls []*Latency) LatencySnapshot {
	var h histogram
	for _, l := range ls {
		l.mu.Lock()
		h.merge(&l.h)
		l.mu.Unlock()
	}
	return h.snapshot()
}

func (h *histogram) snapshot() LatencySnapshot {
	return LatencySnapshot{
		Count: h.total, AvgMs: ms(h.mean()),
		P50Ms: ms(h.quantile(0.50)), P99Ms: ms(h.quantile(0.99)), MaxMs: ms(h.max),
	}
}
