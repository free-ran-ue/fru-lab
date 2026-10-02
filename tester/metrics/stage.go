package metrics

import (
	"sort"
	"sync"
	"time"
)

// Outcome is how one item (a gNB, later a UE) finished a stage.
type Outcome int

const (
	Accepted Outcome = iota // core said yes
	Rejected                // core said no (e.g. NGSetupFailure)
	TimedOut                // no answer before the stage timeout
	Failed                  // local or transport error (e.g. SCTP refused)
)

// Stage tracks one pipeline step. Total time is wall clock from the first
// Begin to the last final outcome, per the design's definition.
type Stage struct {
	mu        sync.Mutex
	name      string
	expected  int
	attempted int64
	retries   int64
	inFlight  int64
	outcomes  [4]int64
	causes    map[string]int64
	hist      histogram
	firstAt   time.Time
	lastAt    time.Time
	now       func() time.Time
}

func NewStage(name string, expected int) *Stage {
	return &Stage{name: name, expected: expected, causes: map[string]int64{}, now: time.Now}
}

// Begin marks one attempt starting. retry is true for the 2nd+ attempt of
// the same item, which is counted in Retries instead of Attempted.
func (s *Stage) Begin(retry bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.firstAt.IsZero() {
		s.firstAt = s.now()
	}
	if retry {
		s.retries++
	} else {
		s.attempted++
	}
	s.inFlight++
}

// Retrying closes an attempt that failed but will be tried again; it does
// not count as an outcome.
func (s *Stage) Retrying() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inFlight--
}

// Finish records an item's final outcome. latency is measured from that
// item's first attempt, so retries lengthen it. cause is ignored for
// Accepted.
func (s *Stage) Finish(o Outcome, latency time.Duration, cause string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inFlight--
	s.outcomes[o]++
	s.lastAt = s.now()
	if o == Accepted {
		s.hist.record(latency)
		return
	}
	if cause == "" {
		cause = "unknown"
	}
	s.causes[cause]++
}

type CauseCount struct {
	Cause string `json:"cause"`
	Count int64  `json:"count"`
}

// StageSnapshot is the JSON the UI renders as one stage card. Latencies
// are milliseconds and cover accepted items only.
type StageSnapshot struct {
	Name        string       `json:"name"`
	Expected    int          `json:"expected"`
	Attempted   int64        `json:"attempted"`
	Retries     int64        `json:"retries"`
	InFlight    int64        `json:"inFlight"`
	Accepted    int64        `json:"accepted"`
	Rejected    int64        `json:"rejected"`
	TimedOut    int64        `json:"timedOut"`
	Failed      int64        `json:"failed"`
	Done        bool         `json:"done"`
	TotalTimeMs float64      `json:"totalTimeMs"`
	AvgMs       float64      `json:"avgMs"`
	P50Ms       float64      `json:"p50Ms"`
	P95Ms       float64      `json:"p95Ms"`
	P99Ms       float64      `json:"p99Ms"`
	MaxMs       float64      `json:"maxMs"`
	Causes      []CauseCount `json:"causes"`
}

func ms(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }

func (s *Stage) Snapshot() StageSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	finished := s.outcomes[Accepted] + s.outcomes[Rejected] + s.outcomes[TimedOut] + s.outcomes[Failed]
	snap := StageSnapshot{
		Name:      s.name,
		Expected:  s.expected,
		Attempted: s.attempted,
		Retries:   s.retries,
		InFlight:  s.inFlight,
		Accepted:  s.outcomes[Accepted],
		Rejected:  s.outcomes[Rejected],
		TimedOut:  s.outcomes[TimedOut],
		Failed:    s.outcomes[Failed],
		Done:      int(finished) == s.expected,
		AvgMs:     ms(s.hist.mean()),
		P50Ms:     ms(s.hist.quantile(0.50)),
		P95Ms:     ms(s.hist.quantile(0.95)),
		P99Ms:     ms(s.hist.quantile(0.99)),
		MaxMs:     ms(s.hist.max),
		Causes:    make([]CauseCount, 0, len(s.causes)),
	}
	if !s.firstAt.IsZero() {
		end := s.now()
		if snap.Done {
			end = s.lastAt
		}
		snap.TotalTimeMs = ms(end.Sub(s.firstAt))
	}
	for c, n := range s.causes {
		snap.Causes = append(snap.Causes, CauseCount{Cause: c, Count: n})
	}
	sort.Slice(snap.Causes, func(i, j int) bool {
		if snap.Causes[i].Count != snap.Causes[j].Count {
			return snap.Causes[i].Count > snap.Causes[j].Count
		}
		return snap.Causes[i].Cause < snap.Causes[j].Cause
	})
	return snap
}
