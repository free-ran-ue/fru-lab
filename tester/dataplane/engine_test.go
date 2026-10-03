package dataplane

import (
	"encoding/binary"
	"fmt"
	"net"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

// fakeUPF stands in for a UPF on loopback: uplink G-PDUs arriving on its
// N3 are decapsulated and forwarded to the sink from its own address (as
// fru-lab's masquerading UPF does); downlink UDP arriving on its "N6" is
// wrapped in a G-PDU with a PDU Session Container (as gtp5g does) and sent
// to the UE's gNB. dropEvery > 0 drops every n-th downlink packet.
type fakeUPF struct {
	n3, n6    *net.UDPConn
	sink      netip.AddrPort
	mu        sync.Mutex
	ues       map[uint32]ueRoute // by UE index (from the tester header)
	dropEvery uint64
	dlSeen    atomic.Uint64
	wrongTeid bool                    // put downlink in a tunnel the gNB did not allocate
	dlFrom    map[netip.AddrPort]bool // source of every downlink packet, under mu
	ulFrom    map[netip.AddrPort]bool // source of every uplink G-PDU, under mu
	dlOrder   []uint32                // UE of every downlink packet, in arrival order, under mu
}

type ueRoute struct {
	dlTeid uint32
	gnb    netip.AddrPort
	ueIP   netip.Addr
}

func newFakeUPF(t *testing.T, sink netip.AddrPort, dropEvery uint64) *fakeUPF {
	t.Helper()
	n3, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.100"), Port: GtpPort})
	require.NoError(t, err)
	n6, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.100"), Port: 0})
	require.NoError(t, err)
	u := &fakeUPF{n3: n3, n6: n6, sink: sink, ues: map[uint32]ueRoute{}, dropEvery: dropEvery, dlFrom: map[netip.AddrPort]bool{}, ulFrom: map[netip.AddrPort]bool{}}
	go u.uplink()
	go u.downlink()
	t.Cleanup(func() { _ = n3.Close(); _ = n6.Close() })
	return u
}

func (u *fakeUPF) n3Addr() netip.AddrPort { return u.n3.LocalAddr().(*net.UDPAddr).AddrPort() }
func (u *fakeUPF) n6Addr() netip.AddrPort { return u.n6.LocalAddr().(*net.UDPAddr).AddrPort() }

func (u *fakeUPF) route(ue int, r ueRoute) {
	u.mu.Lock()
	u.ues[uint32(ue)] = r
	u.mu.Unlock()
}

func (u *fakeUPF) uplink() {
	buf := make([]byte, 65536)
	for {
		n, from, err := u.n3.ReadFromUDPAddrPort(buf)
		if err != nil {
			return
		}
		u.mu.Lock()
		u.ulFrom[from] = true
		u.mu.Unlock()
		_, inner, err := parseGpdu(buf[:n])
		if err != nil {
			continue
		}
		if payload, ok := udpPayload(inner); ok {
			_, _ = u.n6.WriteToUDPAddrPort(payload, u.sink)
		}
	}
}

func (u *fakeUPF) downlink() {
	buf := make([]byte, 65536)
	for {
		n, from, err := u.n6.ReadFromUDPAddrPort(buf)
		if err != nil {
			return
		}
		u.mu.Lock()
		u.dlFrom[from] = true
		u.mu.Unlock()
		seen := u.dlSeen.Add(1)
		if u.dropEvery > 0 && seen%u.dropEvery == 0 {
			continue
		}
		h, ok := parseHeader(buf[:n])
		if !ok {
			continue
		}
		u.mu.Lock()
		u.dlOrder = append(u.dlOrder, h.ue)
		r, ok := u.ues[h.ue]
		wrong := u.wrongTeid
		u.mu.Unlock()
		if !ok {
			continue
		}
		inner := make([]byte, ipv4HeaderLen+udpHeaderLen+n)
		putIPv4UDP(inner, u.sink.Addr(), r.ueIP, u.sink.Port(), u.sink.Port())
		copy(inner[ipv4HeaderLen+udpHeaderLen:], buf[:n])
		gtp := []byte{0x34, 0xff, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x85, 1, 0x00, 0x09, 0x00}
		binary.BigEndian.PutUint16(gtp[2:4], uint16(len(inner)+8))
		teid := r.dlTeid
		if wrong {
			teid += 1000
		}
		binary.BigEndian.PutUint32(gtp[4:8], teid)
		_, _ = u.n3.WriteToUDPAddrPort(append(gtp, inner...), r.gnb)
	}
}

// freePort finds a free UDP port on addr.
func freePort(t *testing.T, addr string) uint16 {
	t.Helper()
	c, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP(addr)})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()
	return uint16(c.LocalAddr().(*net.UDPAddr).Port)
}

// startEngine runs 2 gNBs (127.0.0.11/.12) with 4 UEs through a fake UPF.
func startEngine(t *testing.T, mbps float64, dropEvery uint64) (*Engine, *fakeUPF) {
	t.Helper()
	return startEngineWith(t, mbps, dropEvery, func(*Config) {})
}

// startEngineWith is startEngine with the config adjusted by tune.
func startEngineWith(t *testing.T, mbps float64, dropEvery uint64, tune func(*Config)) (*Engine, *fakeUPF) {
	t.Helper()
	sink := netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), freePort(t, "127.0.0.1"))
	upf := newFakeUPF(t, sink, dropEvery)
	gnbs := []netip.Addr{netip.MustParseAddr("127.0.0.11"), netip.MustParseAddr("127.0.0.12")}
	cfg := Config{
		RunID: 42, UeCount: 4, GnbN3IPs: gnbs, SinkIP: sink.Addr(), Port: sink.Port(),
		PacketSize: 500, UlBps: mbps * 1e6, DlBps: mbps * 1e6,
		DlTarget: func(netip.Addr) netip.AddrPort { return upf.n6Addr() },
	}
	tune(&cfg)
	e := New(cfg)
	require.NoError(t, e.Start())
	for ue := range 4 {
		g := ue / 2
		ueIP := netip.AddrFrom4([4]byte{10, 60, 0, byte(ue + 1)})
		upf.route(ue, ueRoute{dlTeid: uint32(100 + ue), gnb: netip.AddrPortFrom(gnbs[g], GtpPort), ueIP: ueIP})
		e.AddUE(ue, g, ueIP, uint32(0x1000+ue), uint32(100+ue), upf.n3Addr())
	}
	return e, upf
}

func TestEngineCarriesUplinkAndDownlinkThroughTheUPF(t *testing.T) {
	e, _ := startEngine(t, 2, 0) // 4 UEs x 2 Mbps each way
	time.Sleep(1500 * time.Millisecond)
	e.Stop(100 * time.Millisecond)
	s := e.Snapshot()

	require.Equal(t, 4, s.ActiveUes)
	for name, d := range map[string]DirSnapshot{"ul": s.Ul, "dl": s.Dl} {
		require.Positive(t, d.TxPackets, name)
		require.Equal(t, d.TxPackets, d.RxPackets, "%s: nothing is lost on loopback", name)
		require.Equal(t, 0.0, d.LossRate, name)
		require.Zero(t, d.OutOfOrder, name)
		require.Positive(t, d.Latency.Count, name)
		// 8 Mbps for ~1.5 s ≈ 1.5 MB; allow for start-up and ticker jitter
		require.InDelta(t, 1.5e6, float64(d.TxBytes), 0.4e6, name)
	}
	require.NotEmpty(t, s.Series)
	require.Len(t, s.Gnbs, 2)
	require.Equal(t, s.Gnbs[0].DlTxBytes, s.Gnbs[0].DlRxBytes)
	require.Positive(t, s.Gnbs[1].UlRxBytes)
}

func TestEngineCountsDownlinkLoss(t *testing.T) {
	e, _ := startEngine(t, 2, 2) // the fake UPF drops every 2nd downlink packet
	time.Sleep(time.Second)
	e.Stop(100 * time.Millisecond)
	s := e.Snapshot()
	require.InDelta(t, 0.5, s.Dl.LossRate, 0.05)
	require.Equal(t, 0.0, s.Ul.LossRate)
}

func TestEngineIgnoresForeignAndStaleTraffic(t *testing.T) {
	e, upf := startEngine(t, 0, 0) // no traffic of its own
	b := make([]byte, 100)
	putHeader(b, header{runID: 41, ue: 0}) // an earlier run
	_, _ = upf.n6.WriteToUDPAddrPort(b, netip.AddrPortFrom(e.cfg.SinkIP, e.cfg.Port))
	_, _ = upf.n6.WriteToUDPAddrPort([]byte("hello"), netip.AddrPortFrom(e.cfg.SinkIP, e.cfg.Port))
	time.Sleep(50 * time.Millisecond)
	e.Stop(0)
	require.Zero(t, e.Snapshot().Ul.RxPackets)
}

func TestStartFailsWhenAnAddressIsNotLocal(t *testing.T) {
	e := New(Config{UeCount: 1, GnbN3IPs: []netip.Addr{netip.MustParseAddr("192.0.2.1")},
		SinkIP: netip.MustParseAddr("127.0.0.1"), Port: 9, PacketSize: 100})
	err := e.Start()
	require.ErrorContains(t, err, "bind gNB-1 N3 192.0.2.1:2152")
	e.Stop(0) // safe on a never-started engine
}

// A UE's traffic starts only StartDelay after AddUE: right after the PDU
// session is up, the UPF may not know the gNB's downlink tunnel yet, and
// packets sent in that window are lost.
func TestTrafficStartsAfterTheStartDelay(t *testing.T) {
	sink := netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), freePort(t, "127.0.0.1"))
	upf := newFakeUPF(t, sink, 0)
	gnbs := []netip.Addr{netip.MustParseAddr("127.0.0.11")}
	e := New(Config{
		RunID: 1, UeCount: 1, GnbN3IPs: gnbs, SinkIP: sink.Addr(), Port: sink.Port(),
		PacketSize: 500, UlBps: 1e6, DlBps: 1e6, StartDelay: 300 * time.Millisecond,
		DlTarget: func(netip.Addr) netip.AddrPort { return upf.n6Addr() },
	})
	require.NoError(t, e.Start())
	ueIP := netip.MustParseAddr("10.60.0.1")
	upf.route(0, ueRoute{dlTeid: 100, gnb: netip.AddrPortFrom(gnbs[0], GtpPort), ueIP: ueIP})
	e.AddUE(0, 0, ueIP, 0x1000, 100, upf.n3Addr())

	time.Sleep(150 * time.Millisecond)
	s := e.Snapshot()
	require.Zero(t, s.Ul.TxPackets+s.Dl.TxPackets, "nothing is sent during the delay")
	require.Zero(t, s.ActiveUes)

	time.Sleep(400 * time.Millisecond)
	e.Stop(50 * time.Millisecond)
	s = e.Snapshot()
	require.Equal(t, 1, s.ActiveUes)
	require.Positive(t, s.Dl.TxPackets)
	require.Equal(t, s.Dl.TxPackets, s.Dl.RxPackets)
}

// Q11 asks for receive verification: downlink that comes back in a tunnel
// the gNB did not allocate for that UE is a UPF fault, not a delivery.
func TestDownlinkInTheWrongTunnelIsMisroutedNotReceived(t *testing.T) {
	sink := netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), freePort(t, "127.0.0.1"))
	upf := newFakeUPF(t, sink, 0)
	upf.mu.Lock()
	upf.wrongTeid = true
	upf.mu.Unlock()
	gnbs := []netip.Addr{netip.MustParseAddr("127.0.0.11")}
	e := New(Config{RunID: 3, UeCount: 1, GnbN3IPs: gnbs, SinkIP: sink.Addr(), Port: sink.Port(),
		PacketSize: 500, DlBps: 1e6, DlTarget: func(netip.Addr) netip.AddrPort { return upf.n6Addr() }})
	require.NoError(t, e.Start())
	ueIP := netip.MustParseAddr("10.60.0.1")
	upf.route(0, ueRoute{dlTeid: 100, gnb: netip.AddrPortFrom(gnbs[0], GtpPort), ueIP: ueIP})
	e.AddUE(0, 0, ueIP, 0x1000, 100, upf.n3Addr())
	time.Sleep(300 * time.Millisecond)
	e.Stop(50 * time.Millisecond)
	s := e.Snapshot()
	require.Positive(t, s.Dl.TxPackets)
	require.Zero(t, s.Dl.RxPackets)
	require.Equal(t, s.Dl.TxPackets, s.Dl.Misrouted)
}

// One socket carries one writer at a time (Go locks writes per socket),
// which capped downlink at ~80 k packets/s; each sender needs its own,
// and how many there are follows Config.Senders.
func TestDownlinkSendersFollowTheSendersSetting(t *testing.T) {
	e, upf := startEngineWith(t, 1, 0, func(c *Config) { c.Senders = 3 })
	time.Sleep(500 * time.Millisecond)
	e.Stop(50 * time.Millisecond)

	upf.mu.Lock()
	defer upf.mu.Unlock()
	require.Len(t, upf.dlFrom, 3, "4 UEs over 3 downlink senders, each with its own socket")
	for from := range upf.dlFrom {
		require.Equal(t, netip.MustParseAddr("127.0.0.1"), from.Addr(), "downlink leaves from the sink IP")
	}
}

// One sender per gNB capped a gNB's uplink at one socket's rate; each gNB
// now gets several, all sending from its N3 IP (its :2152 socket only
// receives downlink).
func TestUplinkUsesSeveralSendersPerGnbFromItsN3IP(t *testing.T) {
	e, upf := startEngineWith(t, 1, 0, func(c *Config) { c.Senders = 4 }) // 2 per gNB
	time.Sleep(500 * time.Millisecond)
	e.Stop(50 * time.Millisecond)

	upf.mu.Lock()
	defer upf.mu.Unlock()
	perGnb := map[netip.Addr]int{}
	for from := range upf.ulFrom {
		require.NotEqual(t, uint16(GtpPort), from.Port())
		perGnb[from.Addr()]++
	}
	require.Equal(t, map[netip.Addr]int{netip.MustParseAddr("127.0.0.11"): 2, netip.MustParseAddr("127.0.0.12"): 2}, perGnb)
}

// One sink socket with one reader could not keep up with the UPF and the
// kernel dropped uplink that the UPF had forwarded (counted as loss). The
// sink is now several SO_REUSEPORT sockets, one reader each.
func TestUplinkIsReadOnSeveralSinkSockets(t *testing.T) {
	e, _ := startEngineWith(t, 0, 0, func(c *Config) { c.Receivers = 4 })
	defer e.Stop(0)
	require.Len(t, e.sinks, 4)
	for _, c := range e.sinks {
		require.Equal(t, netip.AddrPortFrom(e.cfg.SinkIP, e.cfg.Port), c.LocalAddr().(*net.UDPAddr).AddrPort())
	}

	// uplink from many sources (the UPF's NAT gives each UE its own port)
	sent := 0
	for ue := range 4 {
		c, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.100")})
		require.NoError(t, err)
		for seq := range 50 {
			b := make([]byte, 100)
			putHeader(b, header{runID: 42, ue: uint32(ue), seq: uint32(seq), txNanos: time.Now().UnixNano()})
			_, err := c.WriteToUDPAddrPort(b, netip.AddrPortFrom(e.cfg.SinkIP, e.cfg.Port))
			require.NoError(t, err)
			sent++
		}
		_ = c.Close()
	}
	require.Eventually(t, func() bool { return e.Snapshot().Ul.RxPackets == uint64(sent) }, 2*time.Second, 10*time.Millisecond)
	require.Zero(t, e.Snapshot().Ul.OutOfOrder)
}

// Batched sends (sendmmsg) must still carry every packet, each with its
// own sequence number, at a rate well above the 1 ms tick.
func TestBatchedSendingLosesNothingAtAHighRate(t *testing.T) {
	// GRO off: under the race detector a reader's 64 KB GRO buffers make it
	// ten times slower and the sink overflows (without -race it is as fast)
	e, upf := startEngineWith(t, 40, 0, func(c *Config) { c.Senders = 2; c.Receivers = 2; c.NoGRO = true }) // 4 UEs x 40 Mbps each way
	time.Sleep(time.Second)
	drops := stopCountingDrops(e, upf, 200*time.Millisecond)
	s := e.Snapshot()
	for name, d := range map[string]DirSnapshot{"ul": s.Ul, "dl": s.Dl} {
		require.Greater(t, d.TxPackets, uint64(30000), name)
		require.Equal(t, d.TxPackets, d.RxPackets+drops[name], "%s: every packet arrives or the kernel dropped it for a full buffer", name)
		require.Zero(t, d.OutOfOrder, name)
		require.Zero(t, d.SendErrors, name)
	}
}

// gsoBatcher is an uplink batcher for gNB 127.0.0.11 with 3 UEs, sending
// to a plain socket that stands in for the UPF.
func gsoBatcher(t *testing.T, noGSO bool) (*batcher, []*flow, *net.UDPConn) {
	t.Helper()
	upf, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.100")})
	require.NoError(t, err)
	t.Cleanup(func() { _ = upf.Close() })
	e := New(Config{RunID: 9, UeCount: 3, GnbN3IPs: []netip.Addr{netip.MustParseAddr("127.0.0.11")},
		SinkIP: netip.MustParseAddr("127.0.0.1"), Port: 9200, PacketSize: 500, NoGSO: noGSO})
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.11")})
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	var flows []*flow
	for ue := range 3 {
		e.AddUE(ue, 0, netip.AddrFrom4([4]byte{10, 60, 0, byte(ue + 1)}), uint32(0x100+ue), 1, upf.LocalAddr().(*net.UDPAddr).AddrPort())
		flows = append(flows, e.flows[ue].Load())
	}
	return e.newBatcher(conn, false, 0), flows, upf
}

// readUplink reads n G-PDUs and checks each is a whole, separate packet:
// the UE's headers, its own sequence number, zeros after the tester header.
func readUplink(t *testing.T, upf *net.UDPConn, n int) []header {
	t.Helper()
	var got []header
	buf := make([]byte, 65536)
	require.NoError(t, upf.SetReadDeadline(time.Now().Add(2*time.Second)))
	for range n {
		m, err := upf.Read(buf)
		require.NoError(t, err)
		require.Equal(t, 500+gtpHeaderLen, m, "one packet per datagram")
		teid, inner, err := parseGpdu(buf[:m])
		require.NoError(t, err)
		payload, ok := udpPayload(inner)
		require.True(t, ok)
		h, ok := parseHeader(payload)
		require.True(t, ok)
		require.Equal(t, uint32(0x100)+h.ue, teid)
		require.Equal(t, make([]byte, len(payload)-HeaderLen), payload[HeaderLen:])
		got = append(got, h)
	}
	return got
}

// With UDP GSO, packets to the same UPF share one message (one send for
// many packets, split by the kernel), whichever UE they belong to.
func TestUplinkPacketsShareOneGSOMessage(t *testing.T) {
	b, flows, upf := gsoBatcher(t, false)
	require.Greater(t, b.gso, 1, "this kernel supports UDP GSO")
	for i := range 30 {
		b.add(flows[i%3])
	}
	require.Equal(t, 1, b.n)
	require.Equal(t, 30, b.segs[0])
	b.flush()
	got := readUplink(t, upf, 30)
	for i, h := range got {
		require.Equal(t, uint32(i%3), h.ue)
		require.Equal(t, uint32(i/3), h.seq)
	}
	require.Equal(t, uint64(30), b.stats.packets.Load())
}

func TestNoGSOSendsOnePacketPerMessage(t *testing.T) {
	b, flows, upf := gsoBatcher(t, true)
	require.Equal(t, 1, b.gso)
	for range 5 {
		b.add(flows[0])
	}
	require.Equal(t, 5, b.n)
	b.flush()
	readUplink(t, upf, 5)
}

// If the kernel refuses GSO (a device without checksum offload, or a
// packet over the path MTU), the batch is resent one packet at a time and
// GSO stays off.
func TestRefusedGSOFallsBackToSinglePackets(t *testing.T) {
	b, flows, upf := gsoBatcher(t, false)
	for range 4 {
		b.add(flows[1])
	}
	b.dropGSO(0)
	b.n, b.used = 0, 0
	b.acc.publish(b.stats)
	require.Equal(t, 1, b.gso)
	readUplink(t, upf, 4)
	require.Equal(t, uint64(4), b.stats.packets.Load())
	require.True(t, gsoRefused(&net.OpError{Err: unix.EIO}))
}

// DlBatch gives each UE a run of packets in a row, so downlink (one
// destination per UE) can use GSO too.
func TestDownlinkBatchSendsEachUEARunOfPackets(t *testing.T) {
	// 40 Mbps of 500-byte packets is 10 per ms per UE
	e, upf := startEngineWith(t, 40, 0, func(c *Config) { c.Senders = 1; c.DlBatch = time.Millisecond })
	time.Sleep(300 * time.Millisecond)
	drops := stopCountingDrops(e, upf, 100*time.Millisecond)
	upf.mu.Lock()
	defer upf.mu.Unlock()
	runs := map[int]int{}
	for i := 0; i < len(upf.dlOrder); {
		j := i
		for j < len(upf.dlOrder) && upf.dlOrder[j] == upf.dlOrder[i] {
			j++
		}
		runs[j-i]++
		i = j
	}
	total := 0
	for _, c := range runs {
		total += c
	}
	require.Greater(t, runs[10], total*8/10, "runs of 10 packets per UE: %v", runs)
	s := e.Snapshot()
	require.Equal(t, s.Dl.TxPackets, s.Dl.RxPackets+drops["dl"])
}

// stopCountingDrops stops e's senders, waits drain for packets on the way,
// and returns, per direction, how many packets the kernel dropped on the
// way because a receive buffer was full; then it stops e.
//
// Tests run without CAP_NET_ADMIN, so their sockets keep net.core.rmem_max
// (often 208 KB) where fru-tester forces 8 MB. Under -race on a CI runner
// with few CPUs, the fake UPF (one goroutine per direction) and the
// engine's readers fall behind a burst, and a GSO send puts a whole burst
// into a queue at once, so the kernel drops what does not fit. That is
// this test host's limit, not a loss in the engine: a packet that is
// neither received nor counted here still fails the test.
func stopCountingDrops(e *Engine, upf *fakeUPF, drain time.Duration) map[string]uint64 {
	e.cancel()
	e.senders.Wait()
	time.Sleep(drain)
	ul := []netip.AddrPort{upf.n3Addr(), netip.AddrPortFrom(e.cfg.SinkIP, e.cfg.Port)}
	dl := []netip.AddrPort{upf.n6Addr()}
	for _, ip := range e.cfg.GnbN3IPs {
		dl = append(dl, netip.AddrPortFrom(ip, GtpPort))
	}
	drops := map[string]uint64{"ul": bufferDrops(ul), "dl": bufferDrops(dl)}
	e.Stop(0)
	return drops
}

// bufferDrops sums the drops column of /proc/net/udp over the sockets
// bound to any of addrs (an SO_REUSEPORT group has several).
func bufferDrops(addrs []netip.AddrPort) uint64 {
	want := map[string]bool{}
	for _, a := range addrs {
		ip := a.Addr().As4()
		want[fmt.Sprintf("%08X:%04X", binary.NativeEndian.Uint32(ip[:]), a.Port())] = true
	}
	b, err := os.ReadFile("/proc/net/udp")
	if err != nil {
		return 0
	}
	var total uint64
	for _, line := range strings.Split(string(b), "\n")[1:] {
		f := strings.Fields(line)
		if len(f) < 13 || !want[f[1]] {
			continue
		}
		n, _ := strconv.ParseUint(f[len(f)-1], 10, 64)
		total += n
	}
	return total
}

// sendGSO sends n UL-sink payloads of seg bytes for UE 0 to the sink as
// one UDP GSO message, which loopback delivers in one piece to a socket
// with UDP_GRO.
func sendGSO(t *testing.T, to netip.AddrPort, runID uint32, n, seg int) {
	t.Helper()
	c, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.100")})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()
	require.NoError(t, setGSO(c, seg))
	buf := make([]byte, n*seg)
	for i := range n {
		putHeader(buf[i*seg:], header{runID: runID, ue: 0, seq: uint32(i), txNanos: time.Now().UnixNano()})
	}
	_, err = c.WriteToUDPAddrPort(buf, to)
	require.NoError(t, err)
}

// With UDP GRO a reader gets many packets in one message, with their
// length in a control message, and splits them back into packets.
func TestCoalescedUplinkIsSplitIntoPackets(t *testing.T) {
	for _, noGRO := range []bool{false, true} {
		t.Run(fmt.Sprintf("noGRO=%v", noGRO), func(t *testing.T) {
			e, _ := startEngineWith(t, 0, 0, func(c *Config) { c.Receivers = 1; c.NoGRO = noGRO })
			defer e.Stop(0)
			sendGSO(t, netip.AddrPortFrom(e.cfg.SinkIP, e.cfg.Port), 42, 20, 472)
			require.Eventually(t, func() bool { return e.Snapshot().Ul.RxPackets == 20 }, 2*time.Second, 10*time.Millisecond)
			s := e.Snapshot().Ul
			require.Equal(t, uint64(20*500), s.RxBytes)
			require.Zero(t, s.OutOfOrder)
		})
	}
}

func TestGROSocketGetsOneMessageWithTheSegmentLength(t *testing.T) {
	c, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	require.NoError(t, err)
	defer func() { _ = c.Close() }()
	require.NoError(t, setGRO(c), "this kernel supports UDP GRO")
	sendGSO(t, c.LocalAddr().(*net.UDPAddr).AddrPort(), 1, 5, 300)
	buf, oob := make([]byte, groBufLen), make([]byte, unix.CmsgSpace(4))
	require.NoError(t, c.SetReadDeadline(time.Now().Add(2*time.Second)))
	n, oobn, _, _, err := c.ReadMsgUDP(buf, oob)
	require.NoError(t, err)
	require.Equal(t, 5*300, n)
	require.Equal(t, 300, groSegment(oob[:oobn]))
	require.Zero(t, groSegment(nil))
}
