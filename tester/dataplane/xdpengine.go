package dataplane

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/vishvananda/netlink"

	"tester/xdp"
)

// Engine names for Config.Engine.
const (
	EngineSocket = "socket"
	EngineAFXDP  = "afxdp"
	EngineAuto   = "auto"
)

// ethHeaderLen is an untagged Ethernet header.
const ethHeaderLen = 14

// Frame layout up to the tester header: Ethernet, outer IPv4 and UDP,
// then for uplink the G-PDU header and the inner IPv4 and UDP.
const (
	frameULHead = ethHeaderLen + ipv4HeaderLen + udpHeaderLen + ulHeadLen
	frameDLHead = ethHeaderLen + ipv4HeaderLen + udpHeaderLen
)

// resolveTimeout bounds how long AddUE waits for a next hop's MAC.
const resolveTimeout = 2 * time.Second

// XDPEngine runs one run's data plane over AF_XDP: whole Ethernet frames
// are built in memory the NIC reads and writes, so the kernel's IP, UDP
// and socket code is skipped in both directions. Only the tester's own
// packets are taken from the interface; SCTP, ARP and the rest still go to
// the kernel. It needs the N3 and N6 interface names in Config.
type XDPEngine struct {
	core
	ports    []*xdpPort // one per interface
	n3, n6   *xdpPort
	gnbByIP  map[netip.Addr]int
	ulShards []*shard
	dlShards []*shard

	hopsMu sync.Mutex
	hops   map[netip.Addr]xdp.NextHop // by next-hop address
	noHop  atomic.Int64               // UEs left without traffic: no MAC for their next hop
	hopErr atomic.Pointer[string]

	cancel   context.CancelFunc // senders and sampler
	rxCancel context.CancelFunc
	readers  sync.WaitGroup
}

// xdpPort is one interface: its XDP program and a socket per queue.
type xdpPort struct {
	name    string
	ifindex int
	steer   *xdp.Steering
	socks   []*xdp.Socket
}

func (p *xdpPort) mode() string {
	switch {
	case p.steer.Generic:
		return "generic mode"
	case len(p.socks) > 0 && p.socks[0].ZeroCopy:
		return "zero-copy"
	default:
		return "copy mode"
	}
}

func NewXDP(cfg Config) *XDPEngine {
	return &XDPEngine{core: newCore(cfg, EngineAFXDP), gnbByIP: map[netip.Addr]int{}, hops: map[netip.Addr]xdp.NextHop{}}
}

// rxQueues is how many receive queues an interface has.
func rxQueues(name string) int {
	m, _ := filepath.Glob(filepath.Join("/sys/class/net", name, "queues", "rx-*"))
	return max(1, len(m))
}

func (e *XDPEngine) openPort(name string) (*xdpPort, error) {
	for _, p := range e.ports {
		if p.name == name {
			return p, nil
		}
	}
	l, err := netlink.LinkByName(name)
	if err != nil {
		return nil, fmt.Errorf("interface %q: %w", name, err)
	}
	queues := rxQueues(name)
	if queues > xdp.MaxQueues {
		return nil, fmt.Errorf("%s has %d queues; AF_XDP here serves at most %d (lower them with ethtool -L)", name, queues, xdp.MaxQueues)
	}
	steer, err := xdp.Steer(l.Attrs().Index)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	p := &xdpPort{name: name, ifindex: l.Attrs().Index, steer: steer}
	e.ports = append(e.ports, p)
	for q := range queues {
		s, err := xdp.Open(p.ifindex, q, !steer.Generic)
		if err != nil {
			return nil, fmt.Errorf("%s queue %d: %w", name, q, err)
		}
		p.socks = append(p.socks, s)
		if err := steer.Register(q, s); err != nil {
			return nil, fmt.Errorf("%s queue %d: %w", name, q, err)
		}
	}
	return p, nil
}

// Start loads the XDP program on the N3 and N6 interfaces, opens a socket
// per queue, and starts receiving, sending and sampling.
func (e *XDPEngine) Start() (err error) {
	defer func() {
		if err != nil {
			e.closePorts()
		}
	}()
	cfg := e.cfg
	if cfg.N3Interface == "" || cfg.N6Interface == "" {
		return errors.New("AF_XDP needs the N3 and N6 interface names")
	}
	if frameULHead+HeaderLen+cfg.PacketSize-ulHeadLen+gtpHeaderLen > xdp.FrameSize {
		return fmt.Errorf("packets of %d bytes do not fit an AF_XDP frame (%d bytes)", cfg.PacketSize, xdp.FrameSize)
	}
	if e.n3, err = e.openPort(cfg.N3Interface); err != nil {
		return err
	}
	if e.n6, err = e.openPort(cfg.N6Interface); err != nil {
		return err
	}
	for g, ip := range cfg.GnbN3IPs {
		e.gnbByIP[ip] = g
		if err := e.n3.steer.Allow(ip, GtpPort); err != nil {
			return err
		}
	}
	if err := e.n6.steer.Allow(cfg.SinkIP, cfg.Port); err != nil {
		return err
	}
	for range min(len(e.n3.socks), cfg.Senders) {
		e.ulShards = append(e.ulShards, &shard{})
	}
	for range min(len(e.n6.socks), cfg.Senders) {
		e.dlShards = append(e.dlShards, &shard{})
	}
	var parts []string
	for _, p := range e.ports {
		parts = append(parts, fmt.Sprintf("%s %s, %d queues", p.name, p.mode(), len(p.socks)))
	}
	e.engine = "af_xdp (" + strings.Join(parts, "; ") + ")"

	ctx, cancel := context.WithCancel(context.Background())
	rxCtx, rxCancel := context.WithCancel(context.Background())
	e.cancel, e.rxCancel = cancel, rxCancel
	e.started = cfg.Now()
	for _, p := range e.ports {
		for _, s := range p.socks {
			e.readers.Add(1)
			go e.receive(rxCtx, p, s)
		}
	}
	if cfg.UlBps > 0 {
		for i, sh := range e.ulShards {
			e.senders.Add(1)
			go e.pace(ctx, sh, cfg.UlBps, e.newXDPTx(e.n3.socks[i], false))
		}
	}
	if cfg.DlBps > 0 {
		for i, sh := range e.dlShards {
			e.senders.Add(1)
			go e.pace(ctx, sh, cfg.DlBps, e.newXDPTx(e.n6.socks[i], true))
		}
	}
	e.sampler.Add(1)
	go e.sample(ctx)
	return nil
}

// receive handles one socket's frames until rxCtx is done. A socket on
// the N3 interface sees downlink G-PDUs, one on N6 uplink for the sink;
// one interface can be both.
func (e *XDPEngine) receive(ctx context.Context, p *xdpPort, s *xdp.Socket) {
	defer e.readers.Done()
	var ul, dl *rxCounter
	if p == e.n6 {
		ul = e.newRxCounter(&e.ul)
	}
	if p == e.n3 {
		dl = e.newRxCounter(&e.dl)
	}
	for ctx.Err() == nil {
		now := e.cfg.Now().UnixNano()
		for _, r := range []*rxCounter{ul, dl} {
			if r != nil {
				r.now = now
			}
		}
		if s.Receive(func(f []byte) { e.frame(f, ul, dl) }) == 0 {
			s.Wait(50)
			continue
		}
		for _, r := range []*rxCounter{ul, dl} {
			if r != nil {
				r.publish()
			}
		}
	}
}

// frame checks one received Ethernet frame. The XDP program only passes
// IPv4/UDP without options to an allowed address and port.
func (e *XDPEngine) frame(f []byte, ul, dl *rxCounter) {
	if len(f) < ethHeaderLen+ipv4HeaderLen+udpHeaderLen {
		return
	}
	ip := f[ethHeaderLen:]
	total := int(binary.BigEndian.Uint16(ip[2:4]))
	if total < ipv4HeaderLen+udpHeaderLen || total > len(ip) {
		return
	}
	ip = ip[:total] // without Ethernet padding
	dst, _ := netip.AddrFromSlice(ip[16:20])
	udp := ip[ipv4HeaderLen:]
	port := binary.BigEndian.Uint16(udp[2:4])
	payload := udp[udpHeaderLen:]
	switch {
	case dl != nil && port == GtpPort:
		g, ok := e.gnbByIP[dst]
		if !ok {
			return
		}
		teid, inner, err := parseGpdu(payload)
		if err != nil {
			return
		}
		if p, ok := udpPayload(inner); ok {
			dl.receive(p, true, len(inner), g, teid)
		}
	case ul != nil && port == e.cfg.Port && dst == e.cfg.SinkIP:
		ul.receive(payload, false, total, -1, 0)
	}
}

// xdpTx sends one shard's frames through one socket.
type xdpTx struct {
	c     *core
	sock  *xdp.Socket
	dl    bool
	queue int // frames queued since the last flush
	now   int64
	stats *workerStats
	acc   accumulator
}

func (e *XDPEngine) newXDPTx(s *xdp.Socket, dl bool) *xdpTx {
	d := &e.ul
	if dl {
		d = &e.dl
	}
	t := &xdpTx{c: &e.core, sock: s, dl: dl, stats: d.addSender(len(e.cfg.GnbN3IPs))}
	t.acc.gnb = make([]uint64, len(e.cfg.GnbN3IPs))
	return t
}

func (t *xdpTx) downlink() bool { return t.dl }

func (t *xdpTx) add(f *flow) {
	if t.queue == 0 {
		t.now = t.c.cfg.Now().UnixNano()
	}
	size := t.c.cfg.PacketSize
	fill := func(frame []byte) int {
		if t.dl {
			copy(frame, f.frameDL)
			putHeader(frame[frameDLHead:], header{runID: t.c.cfg.RunID, ue: f.ue, dl: true, seq: f.dlSeq, txNanos: t.now})
			return ethHeaderLen + size
		}
		copy(frame, f.frameUL)
		putHeader(frame[frameULHead:], header{runID: t.c.cfg.RunID, ue: f.ue, seq: f.ulSeq, txNanos: t.now})
		return ethHeaderLen + ipv4HeaderLen + udpHeaderLen + gtpHeaderLen + size
	}
	if !t.sock.Queue(fill) {
		t.flush()
		if !t.sock.Queue(fill) {
			t.acc.errors++ // every buffer is still in flight
			return
		}
	}
	if t.dl {
		f.dlSeq++
	} else {
		f.ulSeq++
	}
	t.queue++
	t.acc.add(f.gnb, uint64(size))
}

func (t *xdpTx) flush() {
	t.sock.Flush()
	t.queue = 0
	t.acc.publish(t.stats)
}

// AddUE builds the UE's uplink and downlink frame templates, looking up
// the MACs of the next hops toward the UPF's N3 and toward the UE (the
// UPF's N6), and starts its traffic after StartDelay.
func (e *XDPEngine) AddUE(ue, gnb int, ueIP netip.Addr, ulTeid, dlTeid uint32, upfN3 netip.AddrPort) {
	if e.stopped.Load() {
		return
	}
	f := e.newFlow(ue, gnb, ueIP, ulTeid, dlTeid, upfN3)
	if err := e.buildFrames(f); err != nil {
		e.noHop.Add(1)
		msg := err.Error()
		e.hopErr.CompareAndSwap(nil, &msg)
		return
	}
	if e.cfg.StartDelay <= 0 {
		e.activate(f)
		return
	}
	time.AfterFunc(e.cfg.StartDelay, func() { e.activate(f) })
}

func (e *XDPEngine) activate(f *flow) {
	if e.stopped.Load() {
		return
	}
	e.flows[f.ue].Store(f)
	if len(e.ulShards) > 0 {
		e.ulShards[int(f.ue)%len(e.ulShards)].add(f)
	}
	if len(e.dlShards) > 0 {
		e.dlShards[int(f.ue)%len(e.dlShards)].add(f)
	}
	e.active.Add(1)
}

// hop is the next hop toward dst, which must leave through want.
func (e *XDPEngine) hop(dst netip.Addr, want *xdpPort) (xdp.NextHop, error) {
	routes, err := netlink.RouteGet(net.IP(dst.AsSlice()))
	if err != nil || len(routes) == 0 {
		return xdp.NextHop{}, fmt.Errorf("no route to %s", dst)
	}
	via := dst
	if gw, ok := netip.AddrFromSlice(routes[0].Gw.To4()); ok && routes[0].Gw != nil {
		via = gw
	}
	e.hopsMu.Lock()
	h, ok := e.hops[via]
	e.hopsMu.Unlock()
	if !ok {
		if h, err = xdp.Resolve(dst, resolveTimeout); err != nil {
			return xdp.NextHop{}, err
		}
		e.hopsMu.Lock()
		e.hops[via] = h
		e.hopsMu.Unlock()
	}
	if h.Ifindex != want.ifindex {
		return xdp.NextHop{}, fmt.Errorf("%s is reached through another interface than %s", dst, want.name)
	}
	return h, nil
}

// buildFrames writes the flow's frame templates up to the tester header;
// the outer UDP source port varies by UE so a receiving NIC spreads the
// flows over its queues.
func (e *XDPEngine) buildFrames(f *flow) error {
	size := e.cfg.PacketSize
	if e.cfg.UlBps > 0 {
		h, err := e.hop(f.upf.Addr(), e.n3)
		if err != nil {
			return err
		}
		b := make([]byte, frameULHead)
		putEthernet(b, h)
		outer := ipv4HeaderLen + udpHeaderLen + gtpHeaderLen + size
		putIPv4UDPHeader(b[ethHeaderLen:], outer, e.cfg.GnbN3IPs[f.gnb], f.upf.Addr(), uint16(32768+f.ue%16384), f.upf.Port())
		copy(b[ethHeaderLen+ipv4HeaderLen+udpHeaderLen:], f.ulHead)
		f.frameUL = b
	}
	if e.cfg.DlBps > 0 {
		h, err := e.hop(f.ueIP, e.n6)
		if err != nil {
			return err
		}
		b := make([]byte, frameDLHead)
		putEthernet(b, h)
		putIPv4UDPHeader(b[ethHeaderLen:], size, e.cfg.SinkIP, f.ueIP, e.cfg.Port, e.cfg.Port)
		f.frameDL = b
	}
	return nil
}

func putEthernet(b []byte, h xdp.NextHop) {
	copy(b[0:6], h.Dst)
	copy(b[6:12], h.Src)
	binary.BigEndian.PutUint16(b[12:14], 0x0800)
}

// Snapshot is the shared snapshot; when some UEs could not get traffic
// for want of a next hop's MAC, the engine line says so.
func (e *XDPEngine) Snapshot() Snapshot {
	s := e.core.Snapshot()
	if n := e.noHop.Load(); n > 0 {
		msg := ""
		if p := e.hopErr.Load(); p != nil {
			msg = *p
		}
		s.Engine += fmt.Sprintf("; %d UEs without traffic: %s", n, msg)
	}
	return s
}

// Stop halts the senders at once, lets in-flight packets arrive for
// drain, then stops receiving and takes the program off the interfaces.
func (e *XDPEngine) Stop(drain time.Duration) {
	if !e.stopped.CompareAndSwap(false, true) || e.cancel == nil {
		return
	}
	e.cancel()
	e.senders.Wait()
	time.Sleep(drain)
	e.rxCancel()
	e.readers.Wait()
	e.sampler.Wait()
	e.takeSample()
	e.closePorts()
}

func (e *XDPEngine) closePorts() {
	for _, p := range e.ports {
		p.steer.Close()
		for _, s := range p.socks {
			s.Close()
		}
	}
	e.ports = nil
}

// isPhysical reports whether name is a NIC rather than a bridge, veth or
// other virtual link: AF_XDP pays off on a NIC, while on a Docker bridge
// the packets still cross veths and the bridge.
func isPhysical(name string) (bool, string) {
	l, err := netlink.LinkByName(name)
	if err != nil {
		return false, fmt.Sprintf("no interface %q", name)
	}
	if l.Type() != "device" || name == "lo" {
		return false, fmt.Sprintf("%s is a %s, not a NIC", name, l.Type())
	}
	if _, err := os.Stat(filepath.Join("/sys/class/net", name, "device")); err != nil {
		return false, fmt.Sprintf("%s is not backed by a device", name)
	}
	return true, ""
}
