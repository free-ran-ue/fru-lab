package dataplane

import (
	"context"
	"time"

	"tester/metrics"
)

// Point is one second of throughput, in bits per second of inner IP
// packets. T is seconds since the engine started.
type Point struct {
	T       float64 `json:"t"`
	UlTxBps float64 `json:"ulTxBps"`
	UlRxBps float64 `json:"ulRxBps"`
	DlTxBps float64 `json:"dlTxBps"`
	DlRxBps float64 `json:"dlRxBps"`
}

// DirSnapshot is one direction. Tx is what the tester sent; Rx is what
// came back through the UPF, so Rx is the UPF's real forwarding rate.
type DirSnapshot struct {
	TxPackets  uint64                  `json:"txPackets"`
	TxBytes    uint64                  `json:"txBytes"`
	RxPackets  uint64                  `json:"rxPackets"`
	RxBytes    uint64                  `json:"rxBytes"`
	TxBps      float64                 `json:"txBps"` // last second
	RxBps      float64                 `json:"rxBps"`
	TxPps      float64                 `json:"txPps"`
	RxPps      float64                 `json:"rxPps"`
	LossRate   float64                 `json:"lossRate"` // 1 - rx/tx so far
	OutOfOrder uint64                  `json:"outOfOrder"`
	Misrouted  uint64                  `json:"misrouted"` // arrived in another UE's tunnel or at another gNB
	SendErrors uint64                  `json:"sendErrors"`
	Latency    metrics.LatencySnapshot `json:"latency"` // one-way, same host clock
}

// GnbTraffic is one gNB's bytes so far, for the per-gNB table.
type GnbTraffic struct {
	UlTxBytes uint64 `json:"ulTxBytes"`
	UlRxBytes uint64 `json:"ulRxBytes"`
	DlTxBytes uint64 `json:"dlTxBytes"`
	DlRxBytes uint64 `json:"dlRxBytes"`
}

type Snapshot struct {
	ActiveUes int          `json:"activeUes"`
	Ul        DirSnapshot  `json:"ul"`
	Dl        DirSnapshot  `json:"dl"`
	Gnbs      []GnbTraffic `json:"gnbs"`
	Series    []Point      `json:"series"`
}

type counters struct {
	at                         time.Time
	ulTx, ulRx, dlTx, dlRx     uint64 // bytes
	ulTxP, ulRxP, dlTxP, dlRxP uint64 // packets
}

func (e *Engine) read() counters {
	return counters{
		at:   e.cfg.Now(),
		ulTx: e.ul.txBytes.Load(), ulRx: e.ul.rxBytes.Load(), dlTx: e.dl.txBytes.Load(), dlRx: e.dl.rxBytes.Load(),
		ulTxP: e.ul.txPackets.Load(), ulRxP: e.ul.rxPackets.Load(), dlTxP: e.dl.txPackets.Load(), dlRxP: e.dl.rxPackets.Load(),
	}
}

func (e *Engine) sample(ctx context.Context) {
	defer e.sampler.Done()
	e.mu.Lock()
	e.prev = e.read()
	e.mu.Unlock()
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			e.takeSample()
		}
	}
}

func (e *Engine) takeSample() {
	now := e.read()
	e.mu.Lock()
	defer e.mu.Unlock()
	dt := now.at.Sub(e.prev.at).Seconds()
	if dt <= 0 {
		return
	}
	bps := func(cur, old uint64) float64 { return float64(cur-old) * 8 / dt }
	p := Point{
		T:       now.at.Sub(e.started).Seconds(),
		UlTxBps: bps(now.ulTx, e.prev.ulTx), UlRxBps: bps(now.ulRx, e.prev.ulRx),
		DlTxBps: bps(now.dlTx, e.prev.dlTx), DlRxBps: bps(now.dlRx, e.prev.dlRx),
	}
	e.last = p
	e.lastPps = [4]float64{
		float64(now.ulTxP-e.prev.ulTxP) / dt, float64(now.ulRxP-e.prev.ulRxP) / dt,
		float64(now.dlTxP-e.prev.dlTxP) / dt, float64(now.dlRxP-e.prev.dlRxP) / dt,
	}
	e.prev = now
	e.history.add(p)
}

func (e *Engine) Snapshot() Snapshot {
	e.mu.Lock()
	last, pps := e.last, e.lastPps
	series := e.history.points()
	e.mu.Unlock()
	s := Snapshot{
		ActiveUes: int(e.active.Load()),
		Ul:        dirSnapshot(&e.ul, last.UlTxBps, last.UlRxBps, pps[0], pps[1]),
		Dl:        dirSnapshot(&e.dl, last.DlTxBps, last.DlRxBps, pps[2], pps[3]),
		Gnbs:      make([]GnbTraffic, len(e.gnbs)),
		Series:    series,
	}
	for i := range e.gnbs {
		g := &e.gnbs[i]
		s.Gnbs[i] = GnbTraffic{UlTxBytes: g.ulTx.Load(), UlRxBytes: g.ulRx.Load(), DlTxBytes: g.dlTx.Load(), DlRxBytes: g.dlRx.Load()}
	}
	return s
}

func dirSnapshot(c *dirCounters, txBps, rxBps, txPps, rxPps float64) DirSnapshot {
	d := DirSnapshot{
		TxPackets: c.txPackets.Load(), TxBytes: c.txBytes.Load(),
		RxPackets: c.rxPackets.Load(), RxBytes: c.rxBytes.Load(),
		TxBps: txBps, RxBps: rxBps, TxPps: txPps, RxPps: rxPps,
		OutOfOrder: c.outOfOrder.Load(), SendErrors: c.sendErrors.Load(), Misrouted: c.misrouted.Load(),
		Latency: c.latency.Snapshot(),
	}
	if d.TxPackets > 0 && d.RxPackets < d.TxPackets {
		d.LossRate = 1 - float64(d.RxPackets)/float64(d.TxPackets)
	}
	return d
}
