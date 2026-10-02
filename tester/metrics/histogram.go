// Package metrics counts stage outcomes and latencies with fixed memory,
// so the same code serves 10 gNBs today and 100k UEs later.
package metrics

import (
	"math"
	"time"
)

// bucketsPerOctave sets resolution: each power of two is split into 16
// buckets, so a reported percentile is within ~4.4% of the true value.
const (
	bucketsPerOctave = 16
	maxOctaves       = 40 // 2^40 µs ≈ 12.7 days, far beyond any timeout
	numBuckets       = bucketsPerOctave*maxOctaves + 1
)

// histogram is not safe for concurrent use; Stage guards it.
type histogram struct {
	counts [numBuckets]uint64
	total  uint64
	sum    time.Duration
	max    time.Duration
}

func bucketOf(d time.Duration) int {
	us := float64(d) / float64(time.Microsecond)
	if us < 1 {
		return 0
	}
	i := int(math.Log2(us)*bucketsPerOctave) + 1
	return min(i, numBuckets-1)
}

// upperBound is the largest duration that lands in bucket i.
func upperBound(i int) time.Duration {
	if i == 0 {
		return time.Microsecond
	}
	return time.Duration(math.Exp2(float64(i)/bucketsPerOctave) * float64(time.Microsecond))
}

func (h *histogram) record(d time.Duration) {
	h.counts[bucketOf(d)]++
	h.total++
	h.sum += d
	h.max = max(h.max, d)
}

// quantile returns the bucket upper bound below which q of the samples
// fall, capped at the true max so p100 is exact.
func (h *histogram) quantile(q float64) time.Duration {
	if h.total == 0 {
		return 0
	}
	rank := uint64(math.Ceil(q * float64(h.total)))
	rank = max(rank, 1)
	var seen uint64
	for i, c := range h.counts {
		seen += c
		if seen >= rank {
			return min(upperBound(i), h.max)
		}
	}
	return h.max
}

func (h *histogram) mean() time.Duration {
	if h.total == 0 {
		return 0
	}
	return h.sum / time.Duration(h.total)
}
