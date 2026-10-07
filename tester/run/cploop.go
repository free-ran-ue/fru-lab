package run

import (
	"sync"
	"time"

	"tester/metrics"
	"tester/procedure"
	"tester/ue"
)

// The control-plane test: UEs past Scale.ThroughputUeCount never get a
// PDU session. Once every throughput UE has a final PDU outcome, each of
// them registers, stays registered Rates.CpLoop.HoldMs, deregisters, and
// goes to the back of the queue for another cycle, until Stop. A UE still
// registered at Stop is deregistered by the normal cleanup.

type CpLoopState string

const (
	CpOff     CpLoopState = "off"     // the profile has no control-plane UEs
	CpWaiting CpLoopState = "waiting" // throughput UEs are still being set up
	CpRunning CpLoopState = "running"
	CpStopped CpLoopState = "stopped"
)

// CpLoopSnapshot is the control-plane test's card. Registration and
// Deregistration count every attempt of every cycle (Expected is 0: a
// loop has no end); latencies are of accepted procedures.
type CpLoopSnapshot struct {
	Ues            int                   `json:"ues"`
	State          CpLoopState           `json:"state"`
	StartedAt      *time.Time            `json:"startedAt"`
	DurationSec    float64               `json:"durationSec"` // loop running time so far
	Registered     int                   `json:"registered"`  // UEs registered right now
	Cycles         int64                 `json:"cycles"`      // registration and deregistration both accepted
	Registration   metrics.StageSnapshot `json:"registration"`
	Deregistration metrics.StageSnapshot `json:"deregistration"`
	// Cleanup: after Stop, the control-plane UEs still registered
	// deregister; the others are counted as not started.
	Cleanup metrics.StageSnapshot `json:"cleanup"`
	Series  []CpPoint             `json:"series"`
}

// CpPoint is one point of the loop's rate curve, per second.
type CpPoint struct {
	T           float64 `json:"t"` // seconds since the loop started
	RegPerSec   float64 `json:"regPerSec"`
	DeregPerSec float64 `json:"deregPerSec"`
	FailPerSec  float64 `json:"failPerSec"` // procedures that did not succeed
}

// cpLoop is the run's control-plane test; nil stage when it has no UEs.
type cpLoop struct {
	ues        []int // UE indexes, in order
	stage      *procStage
	reg, dereg *metrics.Stage
	// cleanup counts the deregistrations Stop's cleanup does for the
	// control-plane UEs still registered (the throughput UEs' are
	// Snapshot.Deregistration).
	cleanup *metrics.Stage

	mu         sync.Mutex
	state      CpLoopState
	startedAt  time.Time
	stoppedAt  time.Time
	registered int
	cycles     int64
	history    cpHistory
}

// cpHistoryMax bounds the curve like the data plane's (600 points).
const cpHistoryMax = 600

func newCpLoop(r *run) *cpLoop {
	l := &cpLoop{
		state: CpOff,
		reg:   metrics.NewStage("cpRegistration", 0),
		dereg: metrics.NewStage("cpDeregistration", 0),
	}
	l.history.max, l.history.width = cpHistoryMax, 1
	for i, u := range r.plan.Ues {
		if u.Cp {
			l.ues = append(l.ues, i)
		}
	}
	l.cleanup = metrics.NewStage("cpCleanupDeregistration", len(l.ues))
	if len(l.ues) > 0 {
		l.state = CpWaiting
		l.stage = newProcStage(r.profile.Rates.CpLoop.ProcedureRate(), len(r.plan.Ues), r.cpCycle)
	}
	return l
}

func (l *cpLoop) snapshot(now time.Time) CpLoopSnapshot {
	l.mu.Lock()
	defer l.mu.Unlock()
	s := CpLoopSnapshot{
		Ues: len(l.ues), State: l.state, Registered: l.registered, Cycles: l.cycles,
		Registration: l.reg.Snapshot(), Deregistration: l.dereg.Snapshot(), Cleanup: l.cleanup.Snapshot(),
		Series: l.history.points(),
	}
	if !l.startedAt.IsZero() {
		started := l.startedAt
		s.StartedAt = &started
		end := now
		if !l.stoppedAt.IsZero() {
			end = l.stoppedAt
		}
		s.DurationSec = end.Sub(l.startedAt).Seconds()
	}
	return s
}

// runCpLoop waits for the throughput UEs, then cycles the control-plane
// UEs until Stop. It returns once in-flight cycles have finished.
func (r *run) runCpLoop() {
	l := r.cp
	if l.stage == nil {
		return
	}
	select {
	case <-r.ctx.Done():
		l.setState(CpStopped, time.Time{})
		return
	case <-r.tpDone:
	}
	l.mu.Lock()
	l.state, l.startedAt = CpRunning, r.deps.Now()
	l.mu.Unlock()
	r.notify()

	r.mu.Lock()
	var start []int
	for _, i := range l.ues {
		if a := r.assocs[r.ues[i].spec.Gnb]; a != nil && a.Err() == nil {
			start = append(start, i)
		}
	}
	r.mu.Unlock()
	for _, i := range start {
		l.stage.enqueue(i)
	}

	sampled := make(chan struct{})
	go func() { defer close(sampled); r.sampleCpLoop() }()
	l.stage.run(r.ctx)
	<-sampled
	l.setState(CpStopped, r.deps.Now())
	r.notify()
}

func (l *cpLoop) setState(s CpLoopState, stoppedAt time.Time) {
	l.mu.Lock()
	l.state = s
	if !stoppedAt.IsZero() {
		l.stoppedAt = stoppedAt
	}
	l.mu.Unlock()
}

// sampleCpLoop adds one point a second to the loop's curve until Stop.
func (r *run) sampleCpLoop() {
	l := r.cp
	t := time.NewTicker(time.Second)
	defer t.Stop()
	var prevReg, prevDereg, prevFail int64
	for {
		select {
		case <-r.ctx.Done():
			return
		case <-t.C:
		}
		reg, dereg := l.reg.Snapshot(), l.dereg.Snapshot()
		fail := reg.Rejected + reg.TimedOut + reg.Failed + dereg.Rejected + dereg.TimedOut + dereg.Failed
		l.mu.Lock()
		l.history.add(CpPoint{T: r.deps.Now().Sub(l.startedAt).Seconds(),
			RegPerSec: float64(reg.Accepted - prevReg), DeregPerSec: float64(dereg.Accepted - prevDereg),
			FailPerSec: float64(fail - prevFail)})
		l.mu.Unlock()
		prevReg, prevDereg, prevFail = reg.Accepted, dereg.Accepted, fail
		r.notify()
	}
}

// cpCycle is one register → hold → deregister cycle of UE i; it returns
// true to queue the UE for its next cycle.
func (r *run) cpCycle(i int) bool {
	l := r.cp
	rates := r.profile.Rates.CpLoop
	timeout := time.Duration(rates.TimeoutMs) * time.Millisecond
	r.mu.Lock()
	u := &r.ues[i]
	assoc := r.assocs[u.spec.Gnb]
	if assoc == nil || assoc.Err() != nil { // its gNB went away: the UE is done
		r.setUeState(i, UeSkipped)
		r.mu.Unlock()
		r.notify()
		return false
	}
	r.setUeState(i, UeRegistering)
	r.mu.Unlock()
	r.notify()

	// a fresh UE every cycle: initial registration with full authentication
	nas, err := ue.New(ue.ConfigFrom(u.spec, r.profile.Gnb, r.profile.Ue))
	l.reg.Begin(false)
	start := r.deps.Now()
	if err != nil {
		l.reg.Finish(metrics.Failed, 0, err.Error())
		return r.cpIdle(i, false)
	}
	link, out := procedure.Register(assoc, nas, timeout)
	end := r.deps.Now()
	if !out.DoneAt.IsZero() {
		end = out.DoneAt
	}
	if out.Result != metrics.Accepted {
		l.reg.Finish(out.Result, end.Sub(start), out.Cause)
		if link != nil {
			assoc.Detach(link)
		}
		return r.cpIdle(i, false)
	}
	l.reg.Finish(metrics.Accepted, end.Sub(start), "")
	r.mu.Lock()
	u.nas, u.link = nas, link
	r.setUeState(i, UeRegistered)
	r.mu.Unlock()
	l.mu.Lock()
	l.registered++
	l.mu.Unlock()
	r.notify()

	if hold := time.Duration(rates.HoldMs) * time.Millisecond; hold > 0 {
		t := time.NewTimer(hold)
		select {
		case <-r.ctx.Done():
			t.Stop()
		case <-t.C:
		}
	}
	if r.ctx.Err() != nil {
		// still registered: Stop's cleanup deregisters it
		l.mu.Lock()
		l.registered--
		l.mu.Unlock()
		return false
	}

	r.mu.Lock()
	r.setUeState(i, UeDeregistering)
	r.mu.Unlock()
	r.notify()
	l.dereg.Begin(false)
	start = r.deps.Now()
	out = procedure.Deregister(assoc, link, nas, timeout, r.cleanupCtx.Done())
	end = r.deps.Now()
	if !out.DoneAt.IsZero() {
		end = out.DoneAt
	}
	r.mu.Lock()
	r.detach(i)
	r.mu.Unlock()
	l.mu.Lock()
	l.registered--
	if out.Result == metrics.Accepted {
		l.cycles++
	}
	l.mu.Unlock()
	if out.Result == metrics.Accepted {
		l.dereg.Finish(metrics.Accepted, end.Sub(start), "")
	} else {
		l.dereg.Finish(out.Result, end.Sub(start), out.Cause)
	}
	return r.cpIdle(i, true)
}

// cpIdle parks UE i between cycles and says whether to queue it again.
func (r *run) cpIdle(i int, cycled bool) bool {
	r.mu.Lock()
	u := &r.ues[i]
	if cycled {
		u.cpCycled = true
	}
	r.setUeState(i, UePending)
	r.mu.Unlock()
	r.notify()
	return r.ctx.Err() == nil
}

// cpHistory keeps the loop's whole rate curve in at most max points,
// halving the resolution each time it fills, like the data plane's.
type cpHistory struct {
	max, width int
	done       []CpPoint
	sum        CpPoint
	filled     int
}

func (h *cpHistory) add(p CpPoint) {
	h.sum = CpPoint{T: p.T, RegPerSec: h.sum.RegPerSec + p.RegPerSec,
		DeregPerSec: h.sum.DeregPerSec + p.DeregPerSec, FailPerSec: h.sum.FailPerSec + p.FailPerSec}
	h.filled++
	if h.filled < h.width {
		return
	}
	h.done = append(h.done, h.average())
	h.sum, h.filled = CpPoint{}, 0
	if len(h.done) < h.max {
		return
	}
	for i := range len(h.done) / 2 {
		a, b := h.done[2*i], h.done[2*i+1]
		h.done[i] = CpPoint{T: b.T, RegPerSec: (a.RegPerSec + b.RegPerSec) / 2,
			DeregPerSec: (a.DeregPerSec + b.DeregPerSec) / 2, FailPerSec: (a.FailPerSec + b.FailPerSec) / 2}
	}
	h.done = h.done[:len(h.done)/2]
	h.width *= 2
}

func (h *cpHistory) average() CpPoint {
	n := float64(h.filled)
	return CpPoint{T: h.sum.T, RegPerSec: h.sum.RegPerSec / n, DeregPerSec: h.sum.DeregPerSec / n, FailPerSec: h.sum.FailPerSec / n}
}

func (h *cpHistory) points() []CpPoint {
	out := append(make([]CpPoint, 0, len(h.done)+1), h.done...)
	if h.filled > 0 {
		out = append(out, h.average())
	}
	return out
}
