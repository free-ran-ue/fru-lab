package run

import (
	"sync"
	"time"
)

// SetupPoint is one point of the run's setup curve: registrations and PDU
// session establishments accepted, and those that finally failed, in one
// second. The Run page's live rate is the last point; the stage's
// accepted count over its total time is the average.
type SetupPoint struct {
	T          float64 `json:"t"` // seconds since the run started
	RegPerSec  float64 `json:"regPerSec"`
	PduPerSec  float64 `json:"pduPerSec"`
	FailPerSec float64 `json:"failPerSec"`
}

func (p SetupPoint) plus(o SetupPoint) SetupPoint {
	return SetupPoint{T: o.T, RegPerSec: p.RegPerSec + o.RegPerSec, PduPerSec: p.PduPerSec + o.PduPerSec, FailPerSec: p.FailPerSec + o.FailPerSec}
}

func (p SetupPoint) div(n float64) SetupPoint {
	return SetupPoint{T: p.T, RegPerSec: p.RegPerSec / n, PduPerSec: p.PduPerSec / n, FailPerSec: p.FailPerSec / n}
}

// setupHistoryMax bounds the curve like the data plane's (600 points).
const setupHistoryMax = 600

type setupCurve struct {
	mu    sync.Mutex
	curve rateCurve[SetupPoint]
}

func (c *setupCurve) points() []SetupPoint {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.curve.points()
}

// sampleSetup adds one point a second to the setup curve until every
// registration and PDU session has its final outcome, or Stop.
func (r *run) sampleSetup() {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	var prevReg, prevPdu, prevFail int64
	for {
		select {
		case <-r.ctx.Done():
			return
		case <-t.C:
		}
		reg, pdu := r.reg.Snapshot(), r.pdu.Snapshot()
		fail := reg.Rejected + reg.TimedOut + reg.Failed + pdu.Rejected + pdu.TimedOut + pdu.Failed
		r.setup.mu.Lock()
		r.setup.curve.add(SetupPoint{T: r.deps.Now().Sub(r.startedAt).Seconds(),
			RegPerSec: float64(reg.Accepted - prevReg), PduPerSec: float64(pdu.Accepted - prevPdu),
			FailPerSec: float64(fail - prevFail)})
		r.setup.mu.Unlock()
		prevReg, prevPdu, prevFail = reg.Accepted, pdu.Accepted, fail
		r.notify()
		if reg.Done && pdu.Done {
			return
		}
	}
}
