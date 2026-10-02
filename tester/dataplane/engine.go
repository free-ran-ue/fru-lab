package dataplane

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"tester/metrics"
)

// Config is one run's traffic setup. Rates are per UE (design Q10).
type Config struct {
	RunID      uint32
	UeCount    int
	GnbN3IPs   []netip.Addr // by gNB index; UL leaves from and DL arrives at <ip>:2152
	SinkIP     netip.Addr   // N6 side: UL is addressed to it, DL is sent from it
	Port       uint16       // UDP port of the sink and of the (simulated) UEs
	PacketSize int          // inner IP packet bytes, MinPacketSize..1400
	UlBps      float64      // per UE; 0 disables uplink
	DlBps      float64      // per UE; 0 disables downlink
	// DlTarget is where a UE's downlink is sent. nil means the UE's own
	// IP:Port, which the host routes to the UPF's N6; tests point it at
	// a fake UPF.
	DlTarget func(ueIP netip.Addr) netip.AddrPort
	Now      func() time.Time
}

// dlSenders shards downlink flows over this many goroutines.
const dlSenders = 4

// historyLen is how many 1-second points Snapshot.Series keeps.
const historyLen = 300

type flow struct {
	ue             uint32
	gnb            int
	upf            netip.AddrPort
	dlTo           netip.AddrPort
	ul             []byte // G-PDU template, written only by its UL sender
	dl             []byte // UDP payload template, written only by its DL sender
	ulSeq, dlSeq   uint32 // sender-owned
	ulLast, dlLast int64  // receiver-owned: last seq seen, -1 = none
}

type shard struct {
	mu      sync.Mutex
	flows   []*flow
	version atomic.Uint64
}

func (s *shard) add(f *flow) {
	s.mu.Lock()
	s.flows = append(s.flows, f)
	s.mu.Unlock()
	s.version.Add(1)
}

func (s *shard) load() []*flow {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*flow(nil), s.flows...)
}

type dirCounters struct {
	txPackets, txBytes, rxPackets, rxBytes atomic.Uint64
	outOfOrder, sendErrors                 atomic.Uint64
	latency                                metrics.Latency
}

type gnbCounters struct {
	ulTx, ulRx, dlTx, dlRx atomic.Uint64 // bytes
}

// Engine runs one run's data plane.
type Engine struct {
	cfg   Config
	n3    []*net.UDPConn
	n6    *net.UDPConn
	flows []atomic.Pointer[flow]

	ulShards []*shard // by gNB
	dlShards []*shard
	ul, dl   dirCounters
	gnbs     []gnbCounters
	active   atomic.Int64

	cancel  context.CancelFunc
	senders sync.WaitGroup
	readers sync.WaitGroup
	sampler sync.WaitGroup

	mu      sync.Mutex
	started time.Time
	series  []Point
	last    Point
	lastPps [4]float64 // ul tx, ul rx, dl tx, dl rx
	prev    counters
}

func New(cfg Config) *Engine {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	e := &Engine{cfg: cfg, flows: make([]atomic.Pointer[flow], cfg.UeCount), gnbs: make([]gnbCounters, len(cfg.GnbN3IPs))}
	for range cfg.GnbN3IPs {
		e.ulShards = append(e.ulShards, &shard{})
	}
	for range dlSenders {
		e.dlShards = append(e.dlShards, &shard{})
	}
	return e
}

// Start binds every socket and starts the receivers, senders and sampler.
// It fails if a gNB N3 IP or the sink IP is not on this host.
func (e *Engine) Start() error {
	for i, ip := range e.cfg.GnbN3IPs {
		c, err := net.ListenUDP("udp4", net.UDPAddrFromAddrPort(netip.AddrPortFrom(ip, GtpPort)))
		if err != nil {
			e.closeSockets()
			return fmt.Errorf("bind gNB-%d N3 %s:%d: %w", i+1, ip, GtpPort, err)
		}
		e.n3 = append(e.n3, c)
	}
	c, err := net.ListenUDP("udp4", net.UDPAddrFromAddrPort(netip.AddrPortFrom(e.cfg.SinkIP, e.cfg.Port)))
	if err != nil {
		e.closeSockets()
		return fmt.Errorf("bind N6 sink %s:%d: %w", e.cfg.SinkIP, e.cfg.Port, err)
	}
	e.n6 = c

	ctx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel
	e.started = e.cfg.Now()
	for g, conn := range e.n3 {
		e.readers.Add(1)
		go e.readN3(g, conn)
	}
	e.readers.Add(1)
	go e.readN6()
	pktBits := float64(e.cfg.PacketSize * 8)
	if e.cfg.UlBps > 0 {
		for g, sh := range e.ulShards {
			e.senders.Add(1)
			go e.pace(ctx, sh, e.cfg.UlBps, pktBits, func(f *flow) { e.sendUl(g, f) })
		}
	}
	if e.cfg.DlBps > 0 {
		for _, sh := range e.dlShards {
			e.senders.Add(1)
			go e.pace(ctx, sh, e.cfg.DlBps, pktBits, e.sendDl)
		}
	}
	e.sampler.Add(1)
	go e.sample(ctx)
	return nil
}

// AddUE starts traffic for an established UE (design Q9: as soon as its
// PDU session is up, without waiting for the others).
func (e *Engine) AddUE(ue, gnb int, ueIP netip.Addr, ulTeid, dlTeid uint32, upfN3 netip.AddrPort) {
	_ = dlTeid // downlink is attributed by the payload header, which survives NAT
	f := &flow{ue: uint32(ue), gnb: gnb, upf: upfN3, ulLast: -1, dlLast: -1}
	f.ul = ulTemplate(ulTeid, ueIP, e.cfg.SinkIP, e.cfg.Port, e.cfg.PacketSize)
	f.dl = make([]byte, e.cfg.PacketSize-ipv4HeaderLen-udpHeaderLen)
	f.dlTo = netip.AddrPortFrom(ueIP, e.cfg.Port)
	if e.cfg.DlTarget != nil {
		f.dlTo = e.cfg.DlTarget(ueIP)
	}
	e.flows[ue].Store(f)
	e.ulShards[gnb].add(f)
	e.dlShards[ue%dlSenders].add(f)
	e.active.Add(1)
}

// pace sends round-robin over a shard's flows at bps per flow, using a
// token bucket refilled every millisecond and capped at 10 ms of burst.
func (e *Engine) pace(ctx context.Context, sh *shard, bps, pktBits float64, send func(*flow)) {
	defer e.senders.Done()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	var flows []*flow
	var version uint64
	var budget float64
	rr := 0
	last := time.Now()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-tick.C:
			if v := sh.version.Load(); v != version {
				version, flows = v, sh.load()
			}
			rate := bps * float64(len(flows))
			budget = min(budget+rate*now.Sub(last).Seconds(), rate*0.01+pktBits)
			last = now
			for budget >= pktBits && len(flows) > 0 {
				send(flows[rr%len(flows)])
				rr++
				budget -= pktBits
			}
		}
	}
}

func (e *Engine) sendUl(g int, f *flow) {
	putHeader(f.ul[gtpHeaderLen+ipv4HeaderLen+udpHeaderLen:], header{
		runID: e.cfg.RunID, ue: f.ue, seq: f.ulSeq, txNanos: e.cfg.Now().UnixNano(),
	})
	f.ulSeq++
	if _, err := e.n3[g].WriteToUDPAddrPort(f.ul, f.upf); err != nil {
		e.ul.sendErrors.Add(1)
		return
	}
	e.ul.txPackets.Add(1)
	e.ul.txBytes.Add(uint64(e.cfg.PacketSize))
	e.gnbs[g].ulTx.Add(uint64(e.cfg.PacketSize))
}

func (e *Engine) sendDl(f *flow) {
	putHeader(f.dl, header{runID: e.cfg.RunID, ue: f.ue, dl: true, seq: f.dlSeq, txNanos: e.cfg.Now().UnixNano()})
	f.dlSeq++
	if _, err := e.n6.WriteToUDPAddrPort(f.dl, f.dlTo); err != nil {
		e.dl.sendErrors.Add(1)
		return
	}
	e.dl.txPackets.Add(1)
	e.dl.txBytes.Add(uint64(e.cfg.PacketSize))
	e.gnbs[f.gnb].dlTx.Add(uint64(e.cfg.PacketSize))
}

// readN6 receives uplink after the UPF decapsulated it. The source may be
// NATed (fru-lab's UPF masquerades), so the UE comes from the header.
func (e *Engine) readN6() {
	defer e.readers.Done()
	buf := make([]byte, 65536)
	for {
		n, _, err := e.n6.ReadFromUDPAddrPort(buf)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			continue
		}
		e.receive(&e.ul, buf[:n], false, ipv4HeaderLen+udpHeaderLen+n)
	}
}

// readN3 receives downlink G-PDUs addressed to one gNB's N3 IP.
func (e *Engine) readN3(g int, conn *net.UDPConn) {
	defer e.readers.Done()
	buf := make([]byte, 65536)
	for {
		n, _, err := conn.ReadFromUDPAddrPort(buf)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			continue
		}
		_, inner, err := parseGpdu(buf[:n])
		if err != nil {
			continue
		}
		if payload, ok := udpPayload(inner); ok {
			e.receive(&e.dl, payload, true, len(inner))
		}
	}
}

func (e *Engine) receive(c *dirCounters, payload []byte, dl bool, ipLen int) {
	h, ok := parseHeader(payload)
	if !ok || h.runID != e.cfg.RunID || h.dl != dl || int(h.ue) >= len(e.flows) {
		return // not ours, or left over from an earlier run
	}
	f := e.flows[h.ue].Load()
	if f == nil {
		return
	}
	c.rxPackets.Add(1)
	c.rxBytes.Add(uint64(ipLen))
	c.latency.Record(time.Duration(e.cfg.Now().UnixNano() - h.txNanos))
	last := &f.ulLast
	if dl {
		last = &f.dlLast
		e.gnbs[f.gnb].dlRx.Add(uint64(ipLen))
	} else {
		e.gnbs[f.gnb].ulRx.Add(uint64(ipLen))
	}
	if int64(h.seq) <= *last {
		c.outOfOrder.Add(1)
	} else {
		*last = int64(h.seq)
	}
}

// Stop halts the senders at once (design Q12), lets in-flight packets
// arrive for drain, then closes the sockets.
func (e *Engine) Stop(drain time.Duration) {
	if e.cancel == nil {
		return
	}
	e.cancel()
	e.senders.Wait()
	time.Sleep(drain)
	e.closeSockets()
	e.readers.Wait()
	e.sampler.Wait()
	e.takeSample()
}

func (e *Engine) closeSockets() {
	for _, c := range e.n3 {
		_ = c.Close()
	}
	if e.n6 != nil {
		_ = e.n6.Close()
	}
}
