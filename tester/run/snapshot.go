// Package run owns the lifecycle of the single active test run: configure
// gNB IPs, bring up N2 for every gNB, hold the associations until Stop,
// then tear everything down. One run at a time (design Q17).
package run

import (
	"slices"
	"time"

	"tester/cpuset"
	"tester/dataplane"
	"tester/metrics"
	"tester/profile"
)

type State string

const (
	StateIdle        State = "idle"        // no run since the process started
	StateConfiguring State = "configuring" // adding gNB IPs to host interfaces
	StateN2          State = "n2"          // SCTP + NG Setup in progress
	StateRunning     State = "running"     // N2 finished; UEs register / establish PDU sessions, then hold
	StateStopping    State = "stopping"    // traffic stopped; deregistering UEs, closing associations, removing IPs
	StateStopped     State = "stopped"
	StateFailed      State = "failed" // could not configure the host; nothing was attempted
)

// Finished reports whether a new run may start.
func (s State) Finished() bool {
	return s == StateIdle || s == StateStopped || s == StateFailed
}

type GnbState string

const (
	GnbPending    GnbState = "pending"
	GnbConnecting GnbState = "connecting"
	GnbUp         GnbState = "up"
	GnbFailed     GnbState = "failed"
	GnbLost       GnbState = "lost" // was up, then the AMF side dropped it
	GnbClosed     GnbState = "closed"
)

// GnbStatus is one row of the per-gNB table.
type GnbStatus struct {
	profile.GnbSpec
	State       GnbState `json:"state"`
	Attempts    int      `json:"attempts"`
	LatencyMs   float64  `json:"latencyMs"`
	Cause       string   `json:"cause"`
	Registered  int      `json:"registered"`  // UEs of this gNB that completed registration
	Established int      `json:"established"` // UEs of this gNB with a PDU session
}

// UeState is where one UE is in the pipeline.
type UeState string

const (
	UePending       UeState = "pending" // waiting for its gNB or a registration slot
	UeRegistering   UeState = "registering"
	UeRegistered    UeState = "registered" // waiting for a PDU slot
	UeEstablishing  UeState = "establishing"
	UeEstablished   UeState = "established"
	UeFailed        UeState = "failed"    // registration or PDU failed for good
	UeSkipped       UeState = "skipped"   // its gNB never came up
	UeCancelled     UeState = "cancelled" // the run was stopped before it finished
	UeDeregistering UeState = "deregistering"
	UeDeregistered  UeState = "deregistered" // cleanup after Stop deregistered it
)

// UeSummary counts UEs per state; the fields add up to the UE count.
type UeSummary struct {
	Pending       int `json:"pending"`
	Registering   int `json:"registering"`
	Registered    int `json:"registered"`
	Establishing  int `json:"establishing"`
	Established   int `json:"established"`
	Failed        int `json:"failed"`
	Skipped       int `json:"skipped"`
	Cancelled     int `json:"cancelled"`
	Deregistering int `json:"deregistering"`
	Deregistered  int `json:"deregistered"`
}

func (s *UeSummary) add(st UeState, d int) {
	switch st {
	case UePending:
		s.Pending += d
	case UeRegistering:
		s.Registering += d
	case UeRegistered:
		s.Registered += d
	case UeEstablishing:
		s.Establishing += d
	case UeEstablished:
		s.Established += d
	case UeFailed:
		s.Failed += d
	case UeSkipped:
		s.Skipped += d
	case UeCancelled:
		s.Cancelled += d
	case UeDeregistering:
		s.Deregistering += d
	case UeDeregistered:
		s.Deregistered += d
	}
}

// UeFailure is one row of the failed-UE list.
type UeFailure struct {
	Supi     string `json:"supi"`
	Gnb      string `json:"gnb"`
	Stage    string `json:"stage"` // "registration", "pdu" or "deregistration"
	Cause    string `json:"cause"`
	Attempts int    `json:"attempts"`
}

// maxFailuresListed bounds the snapshot; UeSummary.Failed has the total.
const maxFailuresListed = 200

// Report is a finished run as fru-lab stores it in its history: the
// profile it ran with and its final snapshot (series included).
type Report struct {
	Profile  profile.Profile `json:"profile"`
	Snapshot Snapshot        `json:"snapshot"`
}

// Snapshot is everything the run page shows; it is what GET /api/run and
// every WebSocket frame carry.
type Snapshot struct {
	RunID        string                `json:"runId"`
	ProfileName  string                `json:"profileName"`
	State        State                 `json:"state"`
	Error        string                `json:"error"`
	StartedAt    *time.Time            `json:"startedAt"`
	StoppedAt    *time.Time            `json:"stoppedAt"`
	N2           metrics.StageSnapshot `json:"n2"`
	Registration metrics.StageSnapshot `json:"registration"`
	Pdu          metrics.StageSnapshot `json:"pdu"`
	// Cleanup after Stop: every registered UE deregisters, then every
	// gNB's SCTP association is closed (design Q12; no PDU Session
	// Release, the core releases the session with the deregistration).
	Deregistration metrics.StageSnapshot `json:"deregistration"`
	N2Release      metrics.StageSnapshot `json:"n2Release"`
	// StopReason is "user" (Stop pressed) or "maxDuration"; empty while running.
	StopReason string `json:"stopReason"`
	// Gnbs is the gNBs the request asked for (GnbPage): every gNB by
	// default, or one page of those in the asked-for states. GnbTotal is
	// how many match (the pager's total), GnbOffset where the page
	// starts in them, and GnbCounts every gNB counted by state.
	Gnbs      []GnbStatus      `json:"gnbs"`
	GnbTotal  int              `json:"gnbTotal"`
	GnbOffset int              `json:"gnbOffset"`
	GnbCounts map[GnbState]int `json:"gnbCounts"`
	Ues       UeSummary        `json:"ues"`
	FailedUes []UeFailure      `json:"failedUes"` // first maxFailuresListed failures
	CpLoop    CpLoopSnapshot   `json:"cpLoop"`
	// Setup is registrations and PDU sessions per second from the start
	// of the run until both stages are done (at most 600 points; later
	// points average 2, 4, 8 … seconds).
	Setup []SetupPoint `json:"setup"`
	// Cpus is the CPU allocation the run used (empty lists: not pinned).
	Cpus      cpuset.Allocation  `json:"cpus"`
	Dataplane dataplane.Snapshot `json:"dataplane"`
}

// GnbPage picks the gNBs a Snapshot lists. With 10 000 gNBs the full list
// is about 3 MB, too much to send every second: the run page asks for one
// page. Limit < 0 lists every gNB (AllGnbs); States, when set, keeps
// only gNBs in those states, and Offset counts within them.
type GnbPage struct {
	Offset int
	Limit  int
	States []GnbState
}

// AllGnbs lists every gNB: reports, and requests that ask for no page.
var AllGnbs = GnbPage{Limit: -1}

// MaxGnbPage is the largest page a request may ask for.
const MaxGnbPage = 1000

// pick returns the page of gnbs q asks for, how many gNBs match, and
// every gNB counted by state.
func (q GnbPage) pick(gnbs []GnbStatus) (page []GnbStatus, total int, counts map[GnbState]int) {
	counts = map[GnbState]int{}
	page = []GnbStatus{}
	for _, g := range gnbs {
		counts[g.State]++
		if len(q.States) > 0 && !slices.Contains(q.States, g.State) {
			continue
		}
		if total >= q.Offset && (q.Limit < 0 || total < q.Offset+q.Limit) {
			page = append(page, g)
		}
		total++
	}
	return page, total, counts
}
