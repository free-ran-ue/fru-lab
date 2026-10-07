package dataplane

import (
	"context"
	"sync"
	"sync/atomic"
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
	// Engine is how packets are sent and received: "socket" (UDP sockets)
	// or "af_xdp" followed by each interface's mode.
	Engine    string       `json:"engine"`
	ActiveUes int          `json:"activeUes"`
	Ul        DirSnapshot  `json:"ul"`
	Dl        DirSnapshot  `json:"dl"`
	Gnbs      []GnbTraffic `json:"gnbs"`
	Series    []Point      `json:"series"`
}

// workerStats is one sender's or one reader's counters. Only its own
// goroutine writes them, once per batch, so senders and readers never
// wait on each other's counters (or one latency lock); Snapshot sums them.
type workerStats struct {
	packets, bytes        atomic.Uint64 // sent by a sender, received by a reader
	sendErrors            atomic.Uint64
	outOfOrder, misrouted atomic.Uint64
	gnbBytes              []atomic.Uint64 // the same bytes, by gNB
	latency               metrics.Latency // readers only
	_                     [64]byte        // keep the next worker's counters off this cache line
}

// dirStats is one direction's workers.
type dirStats struct {
	mu               sync.Mutex
	senders, readers []*workerStats
}

func (d *dirStats) addSender(gnbs int) *workerStats { return d.add(&d.senders, gnbs) }
func (d *dirStats) addReader(gnbs int) *workerStats { return d.add(&d.readers, gnbs) }

func (d *dirStats) add(to *[]*workerStats, gnbs int) *workerStats {
	w := &workerStats{gnbBytes: make([]atomic.Uint64, gnbs)}
	d.mu.Lock()
	*to = append(*to, w)
	d.mu.Unlock()
	return w
}

func (d *dirStats) workers() (senders, readers []*workerStats) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.senders, d.readers
}

// totals sums workers' packets and bytes.
func totals(ws []*workerStats) (packets, bytes uint64) {
	for _, w := range ws {
		packets += w.packets.Load()
		bytes += w.bytes.Load()
	}
	return packets, bytes
}

// accumulator is what a worker counts during one batch, published to its
// workerStats in one go when the batch is done.
type accumulator struct {
	packets, bytes                uint64
	errors, outOfOrder, misrouted uint64
	gnb                           []uint64 // bytes by gNB
}

func (a *accumulator) add(gnb int, bytes uint64) {
	a.packets++
	a.bytes += bytes
	a.gnb[gnb] += bytes
}

func (a *accumulator) publish(w *workerStats) {
	if a.packets > 0 {
		w.packets.Add(a.packets)
		w.bytes.Add(a.bytes)
		for g, b := range a.gnb {
			if b > 0 {
				w.gnbBytes[g].Add(b)
				a.gnb[g] = 0
			}
		}
	}
	for _, c := range []struct {
		n *uint64
		w *atomic.Uint64
	}{{&a.errors, &w.sendErrors}, {&a.outOfOrder, &w.outOfOrder}, {&a.misrouted, &w.misrouted}} {
		if *c.n > 0 {
			c.w.Add(*c.n)
			*c.n = 0
		}
	}
	a.packets, a.bytes = 0, 0
}

type counters struct {
	at                         time.Time
	ulTx, ulRx, dlTx, dlRx     uint64 // bytes
	ulTxP, ulRxP, dlTxP, dlRxP uint64 // packets
}

func (e *core) read() counters {
	c := counters{at: e.cfg.Now()}
	ulS, ulR := e.ul.workers()
	dlS, dlR := e.dl.workers()
	c.ulTxP, c.ulTx = totals(ulS)
	c.ulRxP, c.ulRx = totals(ulR)
	c.dlTxP, c.dlTx = totals(dlS)
	c.dlRxP, c.dlRx = totals(dlR)
	return c
}

func (e *core) sample(ctx context.Context) {
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

func (e *core) takeSample() {
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

func (e *core) Snapshot() Snapshot {
	e.mu.Lock()
	last, pps := e.last, e.lastPps
	series := e.history.points()
	e.mu.Unlock()
	s := Snapshot{
		ActiveUes: int(e.active.Load()),
		Ul:        dirSnapshot(&e.ul, last.UlTxBps, last.UlRxBps, pps[0], pps[1]),
		Dl:        dirSnapshot(&e.dl, last.DlTxBps, last.DlRxBps, pps[2], pps[3]),
		Gnbs:      make([]GnbTraffic, len(e.cfg.GnbN3IPs)),
		Series:    series,
		Engine:    e.engine,
	}
	for _, f := range []struct {
		d  *dirStats
		tx func(*GnbTraffic) *uint64
		rx func(*GnbTraffic) *uint64
	}{
		{&e.ul, func(g *GnbTraffic) *uint64 { return &g.UlTxBytes }, func(g *GnbTraffic) *uint64 { return &g.UlRxBytes }},
		{&e.dl, func(g *GnbTraffic) *uint64 { return &g.DlTxBytes }, func(g *GnbTraffic) *uint64 { return &g.DlRxBytes }},
	} {
		senders, readers := f.d.workers()
		addGnbBytes(s.Gnbs, senders, f.tx)
		addGnbBytes(s.Gnbs, readers, f.rx)
	}
	return s
}

func addGnbBytes(gnbs []GnbTraffic, ws []*workerStats, field func(*GnbTraffic) *uint64) {
	for _, w := range ws {
		for g := range w.gnbBytes {
			*field(&gnbs[g]) += w.gnbBytes[g].Load()
		}
	}
}

func dirSnapshot(ds *dirStats, txBps, rxBps, txPps, rxPps float64) DirSnapshot {
	senders, readers := ds.workers()
	d := DirSnapshot{TxBps: txBps, RxBps: rxBps, TxPps: txPps, RxPps: rxPps}
	d.TxPackets, d.TxBytes = totals(senders)
	d.RxPackets, d.RxBytes = totals(readers)
	lat := make([]*metrics.Latency, 0, len(readers))
	for _, w := range senders {
		d.SendErrors += w.sendErrors.Load()
	}
	for _, w := range readers {
		d.OutOfOrder += w.outOfOrder.Load()
		d.Misrouted += w.misrouted.Load()
		lat = append(lat, &w.latency)
	}
	d.Latency = metrics.MergedSnapshot(lat)
	if d.TxPackets > 0 && d.RxPackets < d.TxPackets {
		d.LossRate = 1 - float64(d.RxPackets)/float64(d.TxPackets)
	}
	return d
}
