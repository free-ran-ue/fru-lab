package dataplane

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"golang.org/x/net/ipv4"
	"golang.org/x/sys/unix"
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
	// StartDelay holds a UE's traffic back after AddUE. Right after the PDU
	// session is up the UPF may not have the gNB's downlink tunnel yet;
	// packets sent in that window are lost.
	StartDelay time.Duration
	// Senders is how many sender goroutines (each with its own socket)
	// each direction gets; 0 = one per CPU. Downlink uses that many;
	// uplink splits them over the gNBs, at least one per gNB.
	Senders int
	// Receivers is how many sink sockets (SO_REUSEPORT, one reader each)
	// receive uplink; 0 = one per CPU, at most maxReceivers.
	Receivers int
	Now       func() time.Time
}

// rcvBuf is the receive buffer asked for on every socket: at ~1 Gbps the
// default (~208 KB) holds under 2 ms, and a reader stall longer than that
// would drop packets in this host's kernel and look like UPF loss.
const rcvBuf = 8 << 20

// Every sender has its own socket: Go lets one write at a time through a
// socket, so senders sharing one top out at what a single thread sends.
// Senders and readers move up to batchSize packets per system call
// (sendmmsg / recvmmsg).
const (
	batchSize    = 32
	maxReceivers = 16
	readBufLen   = 2048 // largest packet we read: 1400 inner + GTP-U + extensions
)

// historyPoints bounds Snapshot.Series, which covers the whole run.
const historyPoints = 600

type flow struct {
	ue             uint32
	gnb            int
	dlTeid         uint32 // the tunnel downlink must arrive in
	upf            netip.AddrPort
	dlTo           netip.AddrPort
	ul             []byte       // G-PDU template, written only by its UL sender
	dl             []byte       // UDP payload template, written only by its DL sender
	ulSeq, dlSeq   uint32       // sender-owned
	ulLast, dlLast atomic.Int64 // last seq seen, -1 = none; several readers may see one UE
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

// Engine runs one run's data plane.
type Engine struct {
	cfg   Config
	n3    []*net.UDPConn // per gNB, :2152: receives downlink
	ulOut []*net.UDPConn // per uplink shard, on its gNB's N3 IP
	sinks []*net.UDPConn // SO_REUSEPORT group on the sink IP:port: receives uplink
	dlOut []*net.UDPConn // per downlink shard, on the sink IP
	flows []atomic.Pointer[flow]

	ulShards []*shard // ulPerGnb consecutive shards per gNB
	dlShards []*shard
	ulPerGnb int
	ul, dl   dirStats
	active   atomic.Int64
	stopped  atomic.Bool

	cancel  context.CancelFunc
	senders sync.WaitGroup
	readers sync.WaitGroup
	sampler sync.WaitGroup

	mu      sync.Mutex
	started time.Time
	history *history
	last    Point
	lastPps [4]float64 // ul tx, ul rx, dl tx, dl rx
	prev    counters
}

func New(cfg Config) *Engine {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Senders <= 0 {
		cfg.Senders = runtime.NumCPU()
	}
	if cfg.Receivers <= 0 {
		cfg.Receivers = min(runtime.NumCPU(), maxReceivers)
	}
	e := &Engine{cfg: cfg, flows: make([]atomic.Pointer[flow], cfg.UeCount), history: newHistory(historyPoints)}
	gnbs := max(1, len(cfg.GnbN3IPs))
	e.ulPerGnb = max(1, (cfg.Senders+gnbs-1)/gnbs)
	for range len(cfg.GnbN3IPs) * e.ulPerGnb {
		e.ulShards = append(e.ulShards, &shard{})
	}
	for range cfg.Senders {
		e.dlShards = append(e.dlShards, &shard{})
	}
	return e
}

// listenReusePort binds addr with SO_REUSEPORT, so several sockets share
// it and the kernel spreads incoming flows over them.
func listenReusePort(addr netip.AddrPort) (*net.UDPConn, error) {
	lc := net.ListenConfig{Control: func(_, _ string, c syscall.RawConn) error {
		var serr error
		if err := c.Control(func(fd uintptr) {
			serr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
		}); err != nil {
			return err
		}
		return serr
	}}
	pc, err := lc.ListenPacket(context.Background(), "udp4", addr.String())
	if err != nil {
		return nil, err
	}
	return pc.(*net.UDPConn), nil
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
		growReadBuffer(c)
		e.n3 = append(e.n3, c)
	}
	for i := range e.ulShards {
		ip := e.cfg.GnbN3IPs[i/e.ulPerGnb]
		c, err := net.ListenUDP("udp4", net.UDPAddrFromAddrPort(netip.AddrPortFrom(ip, 0)))
		if err != nil {
			e.closeSockets()
			return fmt.Errorf("bind gNB-%d uplink sender on %s: %w", i/e.ulPerGnb+1, ip, err)
		}
		e.ulOut = append(e.ulOut, c)
	}
	for range e.cfg.Receivers {
		c, err := listenReusePort(netip.AddrPortFrom(e.cfg.SinkIP, e.cfg.Port))
		if err != nil {
			e.closeSockets()
			return fmt.Errorf("bind N6 sink %s:%d: %w", e.cfg.SinkIP, e.cfg.Port, err)
		}
		growReadBuffer(c)
		e.sinks = append(e.sinks, c)
	}
	for range e.dlShards {
		c, err := net.ListenUDP("udp4", net.UDPAddrFromAddrPort(netip.AddrPortFrom(e.cfg.SinkIP, 0)))
		if err != nil {
			e.closeSockets()
			return fmt.Errorf("bind N6 downlink sender on %s: %w", e.cfg.SinkIP, err)
		}
		e.dlOut = append(e.dlOut, c)
	}

	ctx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel
	e.started = e.cfg.Now()
	for g, conn := range e.n3 {
		r := e.newReader(conn, &e.dl)
		e.readers.Add(1)
		go r.readN3(g)
	}
	for _, conn := range e.sinks {
		r := e.newReader(conn, &e.ul)
		e.readers.Add(1)
		go r.readSink()
	}
	if e.cfg.UlBps > 0 {
		for i, sh := range e.ulShards {
			b := e.newBatcher(e.ulOut[i], false, i/e.ulPerGnb)
			e.senders.Add(1)
			go e.pace(ctx, sh, e.cfg.UlBps, b)
		}
	}
	if e.cfg.DlBps > 0 {
		for i, sh := range e.dlShards {
			b := e.newBatcher(e.dlOut[i], true, -1)
			e.senders.Add(1)
			go e.pace(ctx, sh, e.cfg.DlBps, b)
		}
	}
	e.sampler.Add(1)
	go e.sample(ctx)
	return nil
}

// AddUE starts traffic for an established UE (design Q9: as soon as its
// PDU session is up, without waiting for the others).
func (e *Engine) AddUE(ue, gnb int, ueIP netip.Addr, ulTeid, dlTeid uint32, upfN3 netip.AddrPort) {
	if e.stopped.Load() {
		return // a PDU accept that lands while the run is stopping
	}
	f := &flow{ue: uint32(ue), gnb: gnb, dlTeid: dlTeid, upf: upfN3}
	f.ulLast.Store(-1)
	f.dlLast.Store(-1)
	f.ul = ulTemplate(ulTeid, ueIP, e.cfg.SinkIP, e.cfg.Port, e.cfg.PacketSize)
	f.dl = make([]byte, e.cfg.PacketSize-ipv4HeaderLen-udpHeaderLen)
	f.dlTo = netip.AddrPortFrom(ueIP, e.cfg.Port)
	if e.cfg.DlTarget != nil {
		f.dlTo = e.cfg.DlTarget(ueIP)
	}
	if e.cfg.StartDelay <= 0 {
		e.activate(f)
		return
	}
	time.AfterFunc(e.cfg.StartDelay, func() { e.activate(f) })
}

// activate puts a flow into its senders' shards; a no-op once stopped.
func (e *Engine) activate(f *flow) {
	if e.stopped.Load() {
		return
	}
	e.flows[f.ue].Store(f)
	e.ulShards[f.gnb*e.ulPerGnb+int(f.ue)%e.ulPerGnb].add(f)
	e.dlShards[int(f.ue)%len(e.dlShards)].add(f)
	e.active.Add(1)
}

// growReadBuffer asks for rcvBuf, forcing past net.core.rmem_max when the
// process may (fru-tester runs with CAP_NET_ADMIN), else as much as allowed.
func growReadBuffer(c *net.UDPConn) {
	raw, err := c.SyscallConn()
	if err == nil {
		var serr error
		_ = raw.Control(func(fd uintptr) {
			serr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_RCVBUFFORCE, rcvBuf)
		})
		if serr == nil {
			return
		}
	}
	_ = c.SetReadBuffer(rcvBuf)
}

// pace sends round-robin over a shard's flows at bps per flow, using a
// token bucket refilled every millisecond and capped at 10 ms of burst.
// Packets go out batchSize at a time through b (sendmmsg).
func (e *Engine) pace(ctx context.Context, sh *shard, bps float64, b *batcher) {
	defer e.senders.Done()
	pktBits := float64(e.cfg.PacketSize * 8)
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
				b.add(flows[rr%len(flows)])
				rr++
				budget -= pktBits
				if b.full() {
					b.flush()
				}
			}
			b.flush()
		}
	}
}

// batcher fills up to batchSize packets, each in its own buffer (the
// per-UE templates stay read-only), and sends them with one sendmmsg.
type batcher struct {
	e     *Engine
	pc    *ipv4.PacketConn
	dl    bool
	gnb   int // uplink: the gNB it sends for; downlink: -1 (each flow's own)
	msgs  []ipv4.Message
	flows []*flow
	addrs []net.UDPAddr
	n     int
	stats *workerStats
	acc   accumulator
}

func (e *Engine) newBatcher(conn *net.UDPConn, dl bool, gnb int) *batcher {
	d := &e.ul
	if dl {
		d = &e.dl
	}
	b := &batcher{e: e, pc: ipv4.NewPacketConn(conn), dl: dl, gnb: gnb,
		msgs: make([]ipv4.Message, batchSize), flows: make([]*flow, batchSize), addrs: make([]net.UDPAddr, batchSize),
		stats: d.addSender(len(e.cfg.GnbN3IPs))}
	b.acc.gnb = make([]uint64, len(e.cfg.GnbN3IPs))
	size := e.cfg.PacketSize + gtpHeaderLen // uplink carries the G-PDU header
	if dl {
		size = e.cfg.PacketSize - ipv4HeaderLen - udpHeaderLen // the kernel adds IP/UDP
	}
	for i := range b.msgs {
		b.msgs[i].Buffers = [][]byte{make([]byte, size)}
	}
	return b
}

func (b *batcher) full() bool { return b.n == batchSize }

// add copies f's template into the next slot and stamps its header.
func (b *batcher) add(f *flow) {
	buf := b.msgs[b.n].Buffers[0]
	now := b.e.cfg.Now().UnixNano()
	if b.dl {
		copy(buf, f.dl)
		putHeader(buf, header{runID: b.e.cfg.RunID, ue: f.ue, dl: true, seq: f.dlSeq, txNanos: now})
		f.dlSeq++
		b.addrs[b.n] = *net.UDPAddrFromAddrPort(f.dlTo)
	} else {
		copy(buf, f.ul)
		putHeader(buf[gtpHeaderLen+ipv4HeaderLen+udpHeaderLen:], header{runID: b.e.cfg.RunID, ue: f.ue, seq: f.ulSeq, txNanos: now})
		f.ulSeq++
		b.addrs[b.n] = *net.UDPAddrFromAddrPort(f.upf)
	}
	b.msgs[b.n].Addr = &b.addrs[b.n]
	b.flows[b.n] = f
	b.n++
}

// flush sends what is batched; a message the kernel refuses is counted as
// a send error and the rest still go out. The batch's counts are then
// published to the sender's own stats.
func (b *batcher) flush() {
	for off := 0; off < b.n; {
		sent, err := b.pc.WriteBatch(b.msgs[off:b.n], 0)
		for _, f := range b.flows[off : off+sent] {
			b.count(f)
		}
		off += sent
		if err != nil {
			if off < b.n {
				b.acc.errors++
				off++ // skip the message that failed
			} else {
				break
			}
		}
	}
	b.n = 0
	b.acc.publish(b.stats)
}

func (b *batcher) count(f *flow) {
	g := b.gnb
	if b.dl {
		g = f.gnb
	}
	b.acc.add(g, uint64(b.e.cfg.PacketSize))
}

// reader reads one socket batchSize packets at a time (recvmmsg) and
// counts what it receives in its own stats.
type reader struct {
	e     *Engine
	pc    *ipv4.PacketConn
	msgs  []ipv4.Message
	stats *workerStats
	acc   accumulator
	lat   []time.Duration // the batch's one-way delays
}

func (e *Engine) newReader(conn *net.UDPConn, d *dirStats) *reader {
	r := &reader{e: e, pc: ipv4.NewPacketConn(conn), msgs: make([]ipv4.Message, batchSize),
		stats: d.addReader(len(e.cfg.GnbN3IPs))}
	r.acc.gnb = make([]uint64, len(e.cfg.GnbN3IPs))
	for i := range r.msgs {
		r.msgs[i].Buffers = [][]byte{make([]byte, readBufLen)}
	}
	return r
}

// loop hands each packet to handle until the socket is closed.
func (r *reader) loop(handle func([]byte)) {
	defer r.e.readers.Done()
	for {
		n, err := r.pc.ReadBatch(r.msgs, 0)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			continue
		}
		for _, m := range r.msgs[:n] {
			handle(m.Buffers[0][:m.N])
		}
		r.stats.latency.RecordAll(r.lat)
		r.lat = r.lat[:0]
		r.acc.publish(r.stats)
	}
}

// readSink receives uplink after the UPF decapsulated it, on one socket of
// the sink's SO_REUSEPORT group. The source may be NATed (fru-lab's UPF
// masquerades), so the UE comes from the header.
func (r *reader) readSink() {
	r.loop(func(payload []byte) {
		r.receive(payload, false, ipv4HeaderLen+udpHeaderLen+len(payload), -1, 0)
	})
}

// readN3 receives downlink G-PDUs addressed to one gNB's N3 IP.
func (r *reader) readN3(g int) {
	r.loop(func(b []byte) {
		teid, inner, err := parseGpdu(b)
		if err != nil {
			return
		}
		if payload, ok := udpPayload(inner); ok {
			r.receive(payload, true, len(inner), g, teid)
		}
	})
}

// receive counts one packet that came back. For downlink, gnb and teid
// are where it arrived; a UE's downlink in another tunnel or at another
// gNB is a forwarding fault, counted as misrouted rather than received
// (this also keeps each flow's dlLast owned by its own gNB's reader).
func (r *reader) receive(payload []byte, dl bool, ipLen, gnb int, teid uint32) {
	e := r.e
	h, ok := parseHeader(payload)
	if !ok || h.runID != e.cfg.RunID || h.dl != dl || int(h.ue) >= len(e.flows) {
		return // not ours, or left over from an earlier run
	}
	f := e.flows[h.ue].Load()
	if f == nil {
		return
	}
	if dl && (teid != f.dlTeid || gnb != f.gnb) {
		r.acc.misrouted++
		return
	}
	r.acc.add(f.gnb, uint64(ipLen))
	r.lat = append(r.lat, time.Duration(e.cfg.Now().UnixNano()-h.txNanos))
	last := &f.ulLast
	if dl {
		last = &f.dlLast
	}
	for {
		prev := last.Load()
		if int64(h.seq) <= prev {
			r.acc.outOfOrder++
			break
		}
		if last.CompareAndSwap(prev, int64(h.seq)) {
			break
		}
	}
}

// Stop halts the senders at once (design Q12), lets in-flight packets
// arrive for drain, then closes the sockets.
func (e *Engine) Stop(drain time.Duration) {
	if !e.stopped.CompareAndSwap(false, true) || e.cancel == nil {
		return // never started, or already stopped
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
	for _, group := range [][]*net.UDPConn{e.ulOut, e.sinks, e.dlOut} {
		for _, c := range group {
			_ = c.Close()
		}
	}
}
