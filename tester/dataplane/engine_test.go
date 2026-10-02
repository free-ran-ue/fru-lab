package dataplane

import (
	"encoding/binary"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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
	u := &fakeUPF{n3: n3, n6: n6, sink: sink, ues: map[uint32]ueRoute{}, dropEvery: dropEvery}
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
		n, _, err := u.n3.ReadFromUDPAddrPort(buf)
		if err != nil {
			return
		}
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
		n, _, err := u.n6.ReadFromUDPAddrPort(buf)
		if err != nil {
			return
		}
		seen := u.dlSeen.Add(1)
		if u.dropEvery > 0 && seen%u.dropEvery == 0 {
			continue
		}
		h, ok := parseHeader(buf[:n])
		if !ok {
			continue
		}
		u.mu.Lock()
		r, ok := u.ues[h.ue]
		u.mu.Unlock()
		if !ok {
			continue
		}
		inner := make([]byte, ipv4HeaderLen+udpHeaderLen+n)
		putIPv4UDP(inner, u.sink.Addr(), r.ueIP, u.sink.Port(), u.sink.Port())
		copy(inner[ipv4HeaderLen+udpHeaderLen:], buf[:n])
		gtp := []byte{0x34, 0xff, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x85, 1, 0x00, 0x09, 0x00}
		binary.BigEndian.PutUint16(gtp[2:4], uint16(len(inner)+8))
		binary.BigEndian.PutUint32(gtp[4:8], r.dlTeid)
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
	sink := netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), freePort(t, "127.0.0.1"))
	upf := newFakeUPF(t, sink, dropEvery)
	gnbs := []netip.Addr{netip.MustParseAddr("127.0.0.11"), netip.MustParseAddr("127.0.0.12")}
	e := New(Config{
		RunID: 42, UeCount: 4, GnbN3IPs: gnbs, SinkIP: sink.Addr(), Port: sink.Port(),
		PacketSize: 500, UlBps: mbps * 1e6, DlBps: mbps * 1e6,
		DlTarget: func(netip.Addr) netip.AddrPort { return upf.n6Addr() },
	})
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
