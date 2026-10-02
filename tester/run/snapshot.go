// Package run owns the lifecycle of the single active test run: configure
// gNB IPs, bring up N2 for every gNB, hold the associations until Stop,
// then tear everything down. One run at a time (design Q17).
package run

import (
	"time"

	"tester/metrics"
	"tester/profile"
)

type State string

const (
	StateIdle        State = "idle"        // no run since the process started
	StateConfiguring State = "configuring" // adding gNB IPs to host interfaces
	StateN2          State = "n2"          // SCTP + NG Setup in progress
	StateRunning     State = "running"     // N2 finished; holding associations
	StateStopping    State = "stopping"    // closing associations, removing IPs
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
	State     GnbState `json:"state"`
	Attempts  int      `json:"attempts"`
	LatencyMs float64  `json:"latencyMs"`
	Cause     string   `json:"cause"`
}

// Snapshot is everything the run page shows; it is what GET /api/run and
// every WebSocket frame carry.
type Snapshot struct {
	RunID       string                `json:"runId"`
	ProfileName string                `json:"profileName"`
	State       State                 `json:"state"`
	Error       string                `json:"error"`
	StartedAt   *time.Time            `json:"startedAt"`
	StoppedAt   *time.Time            `json:"stoppedAt"`
	N2          metrics.StageSnapshot `json:"n2"`
	Gnbs        []GnbStatus           `json:"gnbs"`
}
