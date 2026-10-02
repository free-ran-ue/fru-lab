# Throughput Tester Phase 3 (Data Plane) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Every UE starts fixed-rate UL and DL traffic as soon as its PDU session is up, with no TUN devices.
- **UL:** GTP-U G-PDUs sent from the gNB's N3 IP to the UPF's N3.
- **DL:** plain UDP from an N6 sink IP to the UE's IP, which the tester routes via the UPF's N6.
- **Verification:** both directions are received back (UL at the N6 sink, DL as G-PDUs on the gNB's N3), so the Run page shows sent vs received throughput, pps, totals, loss, one-way latency and a per-second chart.

**Architecture:**
- New package `dataplane`.
  - `packet.go`: GTP-U, IPv4/UDP and a 24-byte payload header (magic, run ID, UE+direction, sequence, tx time; design §8), so packets are attributed to their UE even through the UPF's NAT.
  - `engine.go`: one UDP socket per gNB N3 IP on :2152 (UL TX and DL RX) and one N6 socket on sink:port (DL TX and UL RX). Per-gNB UL senders and 4 DL senders, each paced by a 1 ms token bucket.
  - `stats.go`: atomic counters, a latency histogram, a 1 s sampler and a 300 s series.
- `run` configures each gNB's N3 IP (design N1), the sink IP if the host lacks it, and the UE-pool route (Q16). It starts the engine before N2 and hands each established UE to it (Q9). On Stop it halts traffic first (Q12).

**Tech Stack:** Go 1.26.2 standard `net` UDP sockets (the Q4 PoC tier), vishvananda/netlink routes, React SVG chart.

**Spec:**
- Design artifact sections 7 (網卡與路由), 8 (資料面打流量), 9 (資料面的數字), 10 (執行畫面) and 12 "第 3 期". Local copy: `docs/superpowers/throughput-tester-design.html`.
- Answers: `/home/alonza/fru-lab/5g-throughput-tester.md` (Q4, Q9, Q10, Q11, Q15, Q16, N1).
- Base branch: `feat/throughput-tester-phase2`.

## Global Constraints

- Everything in the Phase 1 and Phase 2 plans' Global Constraints still holds.
- PoC tier only: plain UDP sockets, no AF_PACKET or AF_XDP (Q4). The engine sits behind the run's `Dataplane` interface so a faster sender can replace it later.
- One fixed UL and DL rate per UE, one packet size (Q10). 0 Mbps turns a direction off.
- GTP-U header: 8 bytes, no sequence number, no extension headers, no QFI (Q15). Downlink parsing still skips extension headers, because gtp5g adds a PDU Session Container.
- Every UE starts traffic as soon as its PDU session is up (Q9). Stop halts traffic immediately (Q12) and waits 200 ms for in-flight packets before closing the sockets.
- Throughput counts **inner IP packet** bytes. Loss is `1 - rx/tx` so far. Latency is one-way, because tx and rx share the host clock.
- The tester adds, and later removes, only what it needs: per-gNB N3 IPs, the sink IP (as /32, only if missing) and the UE-pool route (only if absent). A different existing route to the pool is an error, never overwritten.
- free5GC template change: the SMF's `urrThreshold` (1000 or 300 bytes) is raised to 10 GB in every template. Acceptance found that at a few hundred Mbps the UPF's usage reports starve PFCP, session modifications time out, and DL never reaches the gNB.

## Review Focus

1. **The UPF masquerades UL** (fru-lab's UPF runs `MASQUERADE` on its interfaces), so the sink sees the UPF's IP as source. UL must still be attributed per UE through the payload header. Pinned by `TestEngineCarriesUplinkAndDownlinkThroughTheUPF` (Task 3; the fake UPF forwards from its own address).
2. **gtp5g adds a PDU Session Container extension header to DL G-PDUs.** The engine must skip it, not drop the packet. Pinned by `TestParseGpduSkipsExtensionHeaders` (Task 2) and by the fake UPF in Task 3, which always sends one.
3. **Late packets from an earlier run, or anything that isn't ours, arriving at the sink** must not be counted. Pinned by `TestEngineIgnoresForeignAndStaleTraffic` (Task 3).
4. **The UE-pool route already exists:** leave an identical one alone, and refuse a different one with a clear message. Pinned by `TestNetlinkAddRemoveOnDummyLink` (Task 5, needs root).
5. **The engine cannot bind a gNB N3 IP** (address in use, or IP missing): the run fails with that message and rolls back every IP and route. Pinned by `TestDataplaneStartFailureFailsTheRunAndRollsBack` (Task 6).

Known Phase 3 limitations, documented in Task 10:
- A few packets in the first milliseconds after each PDU session are lost: DL that leaves before the UPF learns the gNB's tunnel. Real networks behave the same way. Loss is cumulative, so this fades over a run.
- The PoC sender topped out at about 78 k packets/s (~875 Mbps at 1400 B) for DL on the test host.
- Back-to-back runs with the same SUPIs can still leave free5GC with duplicate PDU sessions ("Duplicated PDU session ID"); Phase 4's release and deregistration on Stop fixes that.

## File Structure

| Path | Responsibility |
|---|---|
| `tester/metrics/latency.go` | Exported, mutex-protected latency histogram |
| `tester/dataplane/packet.go` | Payload header, UL G-PDU template, IPv4/UDP, G-PDU parsing |
| `tester/dataplane/engine.go` | Sockets, per-UE flows, paced senders, receivers, Stop |
| `tester/dataplane/stats.go` | Snapshot types, 1 s sampler, series |
| `tester/profile/profile.go`, `expand.go` | `Traffic`, `Network.N6`, validation; sink/UPF-N6 IPs excluded from gNB allocation |
| `tester/netcfg/netcfg.go` | `EnsureRoute`, `RemoveRoute` |
| `tester/run/controller.go`, `snapshot.go` | N3/sink/route configuration, engine lifecycle, `Snapshot.Dataplane` |
| `web/openapi.yaml` + client | `TesterTraffic`, `network.n6`, `TesterDataplaneSnapshot` |
| `web/frontend/src/page/tester/*` | Traffic and N6 on Setup; data-plane card and chart on Run |
| `web/backend/internal/context/templates/*/config/smf*cfg.yaml` | `urrThreshold` raised |
| `docs/tester-guide.md` | Data plane usage, numbers, limitations |

---

### Task 0: Branch

- [ ] `git checkout feat/throughput-tester-phase2 && git checkout -b feat/throughput-tester-phase3`

### Task 1: Latency histogram for the data plane

**Files:** Create `tester/metrics/latency.go`; modify `tester/metrics/histogram_test.go`
**Produces:** `metrics.Latency` with `Record(time.Duration)` and `Snapshot() LatencySnapshot{Count uint64; AvgMs, P50Ms, P99Ms, MaxMs float64}`.

- [ ] **RED:** Replace `tester/metrics/histogram_test.go` with the version below; its new `TestLatencySnapshot` is the failing test. Run `cd tester && go test ./metrics/`. Expected: FAIL (`undefined: Latency`).

```go
package metrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHistogramQuantilesWithinResolution(t *testing.T) {
	var h histogram
	for i := 1; i <= 1000; i++ {
		h.record(time.Duration(i) * time.Millisecond)
	}
	require.InEpsilon(t, 500.0, ms(h.quantile(0.50)), 0.05)
	require.InEpsilon(t, 950.0, ms(h.quantile(0.95)), 0.05)
	require.InEpsilon(t, 990.0, ms(h.quantile(0.99)), 0.05)
	require.Equal(t, time.Second, h.quantile(1.0))
	require.Equal(t, 500500*time.Microsecond, h.mean())
}

func TestHistogramEdges(t *testing.T) {
	var h histogram
	require.Equal(t, time.Duration(0), h.quantile(0.5))
	require.Equal(t, time.Duration(0), h.mean())
	h.record(0)
	h.record(time.Hour * 24 * 365)
	require.Equal(t, time.Hour*24*365, h.max)
	// sub-microsecond samples share bucket 0, reported as its 1µs bound
	require.Equal(t, time.Microsecond, h.quantile(0.5))
}

func TestLatencySnapshot(t *testing.T) {
	var l Latency
	require.Equal(t, LatencySnapshot{}, l.Snapshot())
	l.Record(2 * time.Millisecond)
	l.Record(4 * time.Millisecond)
	s := l.Snapshot()
	require.Equal(t, uint64(2), s.Count)
	require.InDelta(t, 3.0, s.AvgMs, 0.001)
	require.InDelta(t, 4.0, s.MaxMs, 0.001)
}
```

- [ ] **GREEN:** Create `tester/metrics/latency.go`, then run `go test -race ./metrics/`. Expected: ok.

```go
package metrics

import (
	"sync"
	"time"
)

// Latency is a concurrency-safe latency histogram, for the data plane's
// per-packet one-way delays.
type Latency struct {
	mu sync.Mutex
	h  histogram
}

func (l *Latency) Record(d time.Duration) {
	l.mu.Lock()
	l.h.record(d)
	l.mu.Unlock()
}

// LatencySnapshot is in milliseconds.
type LatencySnapshot struct {
	Count uint64  `json:"count"`
	AvgMs float64 `json:"avgMs"`
	P50Ms float64 `json:"p50Ms"`
	P99Ms float64 `json:"p99Ms"`
	MaxMs float64 `json:"maxMs"`
}

func (l *Latency) Snapshot() LatencySnapshot {
	l.mu.Lock()
	defer l.mu.Unlock()
	return LatencySnapshot{
		Count: l.h.total, AvgMs: ms(l.h.mean()),
		P50Ms: ms(l.h.quantile(0.50)), P99Ms: ms(l.h.quantile(0.99)), MaxMs: ms(l.h.max),
	}
}
```

- [ ] Commit `feat: concurrency-safe latency histogram for the data plane`.

### Task 2: Packets

**Files:** Create `tester/dataplane/packet.go`, `tester/dataplane/packet_test.go`
**Produces:**
- `dataplane.HeaderLen`, `MinPacketSize` and `GtpPort`.
- Unexported helpers `putHeader`, `parseHeader`, `ulTemplate`, `putIPv4UDP`, `parseGpdu`, `udpPayload`.

- [ ] **RED:** Create `packet_test.go`, then run `go test ./dataplane/`. Expected: FAIL (`undefined: header`).

```go
package dataplane

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHeaderRoundTrip(t *testing.T) {
	b := make([]byte, HeaderLen)
	in := header{runID: 7, ue: 41, dl: true, seq: 99, txNanos: 1234567890123}
	putHeader(b, in)
	out, ok := parseHeader(b)
	require.True(t, ok)
	require.Equal(t, in, out)
	b[0] = 'X'
	_, ok = parseHeader(b)
	require.False(t, ok, "foreign traffic is ignored")
}

func TestUlTemplateIsAValidGpdu(t *testing.T) {
	pkt := ulTemplate(0x10000001, netip.MustParseAddr("10.60.0.7"), netip.MustParseAddr("10.0.1.1"), 9200, 100)
	require.Len(t, pkt, 8+100)
	teid, inner, err := parseGpdu(pkt)
	require.NoError(t, err)
	require.Equal(t, uint32(0x10000001), teid)
	require.Len(t, inner, 100)
	require.Equal(t, uint16(0), ipChecksum(inner[:20]), "valid IPv4 header checksum sums to zero")
	require.Equal(t, []byte{10, 60, 0, 7}, inner[12:16])
	payload, ok := udpPayload(inner)
	require.True(t, ok)
	require.Len(t, payload, 100-28)
}

func TestParseGpduSkipsExtensionHeaders(t *testing.T) {
	inner := make([]byte, 40)
	putIPv4UDP(inner, netip.MustParseAddr("10.0.1.1"), netip.MustParseAddr("10.60.0.7"), 9200, 9200)
	// flags E set, seq/npdu zero, next ext = 0x85 (PDU session container,
	// 1 unit = 4 bytes, ending with next type 0)
	pkt := append([]byte{0x34, 0xff, 0, 0, 0, 0, 0, 9, 0, 0, 0, 0x85, 1, 0x00, 0x09, 0x00}, inner...)
	teid, got, err := parseGpdu(pkt)
	require.NoError(t, err)
	require.Equal(t, uint32(9), teid)
	require.Equal(t, inner, got)

	_, _, err = parseGpdu([]byte{0x30, 0x01, 0, 0, 0, 0, 0, 1}) // echo request, not a G-PDU
	require.ErrorIs(t, err, errNotGpdu)
	_, _, err = parseGpdu([]byte{0x34, 0xff, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0x85, 0}) // ext length 0
	require.ErrorIs(t, err, errNotGpdu)
}
```

- [ ] **GREEN:** Create `packet.go`, then run `go test ./dataplane/`. Expected: ok.

```go
// Package dataplane generates and verifies user-plane traffic for
// established UEs without TUN devices: uplink is built as GTP-U and sent
// to the UPF's N3 from the gNB's N3 IP; downlink is sent as plain UDP to
// the UE's IP (routed to the UPF's N6) and comes back as GTP-U on the
// gNB's N3 IP. Every packet carries a small header so the receiving side
// can attribute it to a UE even through NAT and measure one-way latency.
package dataplane

import (
	"encoding/binary"
	"errors"
	"net/netip"
)

const (
	// HeaderLen is the tester header at the start of every UDP payload.
	HeaderLen = 24
	// MinPacketSize is the smallest inner IP packet: IPv4 + UDP + header.
	MinPacketSize = ipv4HeaderLen + udpHeaderLen + HeaderLen
	// GtpPort is the GTP-U port on both N3 ends.
	GtpPort = 2152

	ipv4HeaderLen = 20
	udpHeaderLen  = 8
	gtpHeaderLen  = 8
	dlFlag        = 1 << 31 // high bit of the UE field marks downlink
)

var magic = [4]byte{'F', 'R', 'U', 'T'}

// header is the payload header (design §8):
// magic(4) runID(4) ue|dir(4) seq(4) txNanos(8), big endian.
type header struct {
	runID   uint32
	ue      uint32 // 0-based UE index
	dl      bool
	seq     uint32
	txNanos int64
}

func putHeader(b []byte, h header) {
	copy(b[0:4], magic[:])
	binary.BigEndian.PutUint32(b[4:8], h.runID)
	v := h.ue
	if h.dl {
		v |= dlFlag
	}
	binary.BigEndian.PutUint32(b[8:12], v)
	binary.BigEndian.PutUint32(b[12:16], h.seq)
	binary.BigEndian.PutUint64(b[16:24], uint64(h.txNanos))
}

func parseHeader(b []byte) (header, bool) {
	if len(b) < HeaderLen || [4]byte(b[0:4]) != magic {
		return header{}, false
	}
	v := binary.BigEndian.Uint32(b[8:12])
	return header{
		runID:   binary.BigEndian.Uint32(b[4:8]),
		ue:      v &^ dlFlag,
		dl:      v&dlFlag != 0,
		seq:     binary.BigEndian.Uint32(b[12:16]),
		txNanos: int64(binary.BigEndian.Uint64(b[16:24])),
	}, true
}

// ulTemplate builds one UE's uplink G-PDU: an 8-byte GTP-U header (no
// sequence number or extension headers, per design Q15) carrying an IPv4/
// UDP packet from the UE to the N6 sink. packetSize is the inner IP
// packet's total length. Only the tester header changes per packet, and
// UDP checksum 0 is valid for IPv4, so the template is reused as is.
func ulTemplate(teid uint32, ue, sink netip.Addr, port uint16, packetSize int) []byte {
	b := make([]byte, gtpHeaderLen+packetSize)
	b[0] = 0x30 // version 1, protocol type GTP, no optional fields
	b[1] = 0xff // G-PDU
	binary.BigEndian.PutUint16(b[2:4], uint16(packetSize))
	binary.BigEndian.PutUint32(b[4:8], teid)
	putIPv4UDP(b[gtpHeaderLen:], ue, sink, port, port)
	return b
}

// putIPv4UDP writes IPv4 and UDP headers for a packet of len(b) bytes.
func putIPv4UDP(b []byte, src, dst netip.Addr, sport, dport uint16) {
	ip := b[:ipv4HeaderLen]
	ip[0] = 0x45 // v4, 20-byte header
	binary.BigEndian.PutUint16(ip[2:4], uint16(len(b)))
	ip[6] = 0x40 // don't fragment
	ip[8] = 64   // TTL
	ip[9] = 17   // UDP
	s, d := src.As4(), dst.As4()
	copy(ip[12:16], s[:])
	copy(ip[16:20], d[:])
	binary.BigEndian.PutUint16(ip[10:12], ipChecksum(ip))
	udp := b[ipv4HeaderLen:]
	binary.BigEndian.PutUint16(udp[0:2], sport)
	binary.BigEndian.PutUint16(udp[2:4], dport)
	binary.BigEndian.PutUint16(udp[4:6], uint16(len(udp)))
}

func ipChecksum(h []byte) uint16 {
	var sum uint32
	for i := 0; i < len(h); i += 2 {
		sum += uint32(binary.BigEndian.Uint16(h[i:]))
	}
	for sum > 0xffff {
		sum = sum>>16 + sum&0xffff
	}
	return ^uint16(sum)
}

var errNotGpdu = errors.New("not a G-PDU")

// parseGpdu returns the TEID and inner packet of a GTP-U G-PDU, skipping
// the optional fields and any extension headers (gtp5g adds a PDU Session
// Container to downlink packets).
func parseGpdu(b []byte) (uint32, []byte, error) {
	if len(b) < gtpHeaderLen || b[0]>>5 != 1 || b[1] != 0xff {
		return 0, nil, errNotGpdu
	}
	teid := binary.BigEndian.Uint32(b[4:8])
	off := gtpHeaderLen
	if b[0]&0x07 != 0 { // E, S or PN set: 4 more bytes
		if len(b) < off+4 {
			return 0, nil, errNotGpdu
		}
		next := b[off+3]
		off += 4
		for b[0]&0x04 != 0 && next != 0 { // extension headers
			if len(b) < off+1 {
				return 0, nil, errNotGpdu
			}
			l := int(b[off]) * 4
			if l == 0 || len(b) < off+l {
				return 0, nil, errNotGpdu
			}
			next = b[off+l-1]
			off += l
		}
	}
	return teid, b[off:], nil
}

// udpPayload returns the UDP payload of an IPv4/UDP packet.
func udpPayload(ip []byte) ([]byte, bool) {
	if len(ip) < ipv4HeaderLen || ip[0]>>4 != 4 || ip[9] != 17 {
		return nil, false
	}
	ihl := int(ip[0]&0x0f) * 4
	if len(ip) < ihl+udpHeaderLen {
		return nil, false
	}
	return ip[ihl+udpHeaderLen:], true
}
```

- [ ] Commit `feat: gtp-u and payload header packets for the data plane`.

### Task 3: Engine

**Files:** Create `tester/dataplane/engine.go`, `tester/dataplane/stats.go`, `tester/dataplane/engine_test.go`
**Produces:**
- `dataplane.Config{RunID uint32; UeCount int; GnbN3IPs []netip.Addr; SinkIP netip.Addr; Port uint16; PacketSize int; UlBps, DlBps float64; DlTarget func(netip.Addr) netip.AddrPort; Now func() time.Time}`.
- `dataplane.New(Config) *Engine`, with methods `Start() error`, `AddUE(ue, gnb int, ueIP netip.Addr, ulTeid, dlTeid uint32, upfN3 netip.AddrPort)`, `Stop(drain time.Duration)` (safe if never started) and `Snapshot() Snapshot`.
- `dataplane.Snapshot{ActiveUes; Ul, Dl DirSnapshot; Gnbs []GnbTraffic; Series []Point}` with the JSON shape that Task 7 mirrors.

The tests bind real UDP sockets on loopback (127.0.0.1/.11/.12/.100) and use a fake UPF that NATs UL and adds a PDU Session Container to DL. They take about 9 s with `-race`.

- [ ] **RED:** Create `engine_test.go`, then run `go test ./dataplane/`. Expected: FAIL (`undefined: New`).

```go
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
```

- [ ] **GREEN:** Create `engine.go` and `stats.go`, then run `go test -race -count=3 ./dataplane/`. Expected: ok.

```go
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
```

```go
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
	e.series = append(e.series, p)
	if len(e.series) > historyLen {
		e.series = e.series[len(e.series)-historyLen:]
	}
}

func (e *Engine) Snapshot() Snapshot {
	e.mu.Lock()
	last, pps := e.last, e.lastPps
	series := append([]Point{}, e.series...)
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
		OutOfOrder: c.outOfOrder.Load(), SendErrors: c.sendErrors.Load(),
		Latency: c.latency.Snapshot(),
	}
	if d.TxPackets > 0 && d.RxPackets < d.TxPackets {
		d.LossRate = 1 - float64(d.RxPackets)/float64(d.TxPackets)
	}
	return d
}
```

- [ ] Commit `feat: data plane engine with paced senders and rx verification`.

### Task 4: Profile — traffic and N6

**Files:** Replace `tester/profile/profile.go`, `tester/profile/expand.go`, `tester/profile/expand_test.go`
**Produces:** `profile.Traffic{UlMbps, DlMbps float64; PacketSize, Port int}` and `profile.N6Network{Interface, SinkIP, UpfIP, UePool string}`, with field paths `traffic.*` and `network.n6.*`. The sink IP and the UPF N6 IP are excluded from N2/N3 allocation.

- [ ] **RED:** Replace `expand_test.go`, then run `go test ./profile/`. Expected: FAIL (`unknown field N6`, `Traffic`).

```go
package profile

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
)

func sampleProfile() Profile {
	return Profile{
		Name:  "baseline",
		Scale: Scale{GnbCount: 3, UeCount: 10},
		Gnb: GnbTemplate{
			GnbIDStart: "000314", NamePattern: "gNB-{i}",
			Mcc: "208", Mnc: "93", Tac: "000001", Sst: 1, Sd: "010203",
		},
		Ue: UeTemplate{
			MsinStart: "0000000001", Key: "8baf473f2f8fd09487cccbd7097c6862",
			Opc: "8e27b6af0e692e750f32667a3b14605d", Amf: "8000", Sqn: "000000000023",
			Integrity: "nia2", Ciphering: "nea0", Dnn: "internet", Sst: 1, Sd: "010203",
		},
		Network: Network{
			N2: N2Network{Interface: "ens19", Cidr: "10.0.1.0/24", StartIP: "10.0.1.1", AmfIP: "10.0.1.1", AmfPort: 38412},
			N3: N3Network{Interface: "ens20", Cidr: "10.0.2.0/24", StartIP: "10.0.2.2", UpfIP: "10.0.2.1", UpfPort: 2152},
			N6: N6Network{Interface: "ens21", SinkIP: "10.0.3.2", UpfIP: "10.0.3.1", UePool: "10.60.0.0/16"},
		},
		Traffic: Traffic{UlMbps: 1, DlMbps: 5, PacketSize: 1400, Port: 9200},
		Rates: Rates{
			N2:           StageRate{TimeoutMs: 5000, Retries: 1},
			Registration: ProcedureRate{RatePerSec: 50, MaxInFlight: 200, TimeoutMs: 10000, Retries: 1},
			Pdu:          ProcedureRate{RatePerSec: 50, MaxInFlight: 200, TimeoutMs: 10000, Retries: 1},
		},
	}
}

func TestExpandFillsGnbsInOrder(t *testing.T) {
	plan, err := Expand(sampleProfile(), []netip.Addr{netip.MustParseAddr("10.0.1.3")})
	require.NoError(t, err)
	require.Equal(t, 4, plan.UesPerGnb)
	require.Equal(t, 24, plan.N2Prefix)
	require.Equal(t, []GnbSpec{
		// .1 is the AMF, .3 is already on the host
		{Index: 1, Name: "gNB-1", GnbID: "000314", N2IP: "10.0.1.2", N3IP: "10.0.2.2", UeCount: 4, UeFirst: 1, UeLast: 4,
			FirstSupi: "imsi-208930000000001", LastSupi: "imsi-208930000000004"},
		{Index: 2, Name: "gNB-2", GnbID: "000315", N2IP: "10.0.1.4", N3IP: "10.0.2.3", UeCount: 4, UeFirst: 5, UeLast: 8,
			FirstSupi: "imsi-208930000000005", LastSupi: "imsi-208930000000008"},
		{Index: 3, Name: "gNB-3", GnbID: "000316", N2IP: "10.0.1.5", N3IP: "10.0.2.4", UeCount: 2, UeFirst: 9, UeLast: 10,
			FirstSupi: "imsi-208930000000009", LastSupi: "imsi-208930000000010"},
	}, plan.Gnbs)
	require.Len(t, plan.Ues, 10)
	require.Equal(t, UeSpec{Index: 10, Gnb: 2, Msin: "0000000010", Supi: "imsi-208930000000010"}, plan.Ues[9])
}

func TestExpandMoreGnbsThanUes(t *testing.T) {
	p := sampleProfile()
	p.Scale = Scale{GnbCount: 3, UeCount: 2}
	plan, err := Expand(p, nil)
	require.NoError(t, err)
	require.Equal(t, []int{1, 1, 0}, []int{plan.Gnbs[0].UeCount, plan.Gnbs[1].UeCount, plan.Gnbs[2].UeCount})
	require.Equal(t, 0, plan.Gnbs[2].UeFirst)
}

func TestExpandReportsEveryFieldError(t *testing.T) {
	p := sampleProfile()
	p.Name = " "
	p.Scale.GnbCount = 0
	p.Gnb.NamePattern = "gNB"
	p.Gnb.Tac = "1"
	p.Network.N2.AmfIP = "amf"
	p.Rates.N2.TimeoutMs = 0
	_, err := Expand(p, nil)
	var verr *ValidationError
	require.ErrorAs(t, err, &verr)
	fields := []string{}
	for _, fe := range verr.Errors {
		fields = append(fields, fe.Field)
	}
	require.ElementsMatch(t, []string{"name", "scale.gnbCount", "gnb.namePattern", "gnb.tac", "network.n2.amfIp", "rates.n2.timeoutMs"}, fields)
}

func TestExpandReportsCidrShortfallPerInterface(t *testing.T) {
	p := sampleProfile()
	p.Scale.GnbCount = 10
	p.Network.N2.Cidr = "10.0.1.0/29"
	_, err := Expand(p, nil)
	var verr *ValidationError
	require.ErrorAs(t, err, &verr)
	require.Equal(t, []FieldError{{Field: "network.n2.cidr", Message: "10.0.1.0/29 from 10.0.1.1 has only 5 usable IPs, need 10 (short by 5)"}}, verr.Errors)
}

func TestExpandReportsGnbIDOverflow(t *testing.T) {
	p := sampleProfile()
	p.Gnb.GnbIDStart = "fffffe"
	_, err := Expand(p, nil)
	var verr *ValidationError
	require.ErrorAs(t, err, &verr)
	require.Equal(t, "gnb.gnbIdStart", verr.Errors[0].Field)
}

func TestExpandSharedN2N3CidrNeverReusesAnIP(t *testing.T) {
	p := sampleProfile()
	p.Network.N3 = N3Network{Interface: "ens19", Cidr: "10.0.1.0/24", StartIP: "10.0.1.1", UpfIP: "10.0.1.6", UpfPort: 2152}
	plan, err := Expand(p, nil)
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, g := range plan.Gnbs {
		require.False(t, seen[g.N2IP], "duplicate %s", g.N2IP)
		seen[g.N2IP] = true
	}
	for _, g := range plan.Gnbs {
		require.False(t, seen[g.N3IP], "N3 IP %s reuses an N2 IP", g.N3IP)
		seen[g.N3IP] = true
	}
	for _, core := range []string{"10.0.1.1", "10.0.1.6"} { // AMF, UPF
		require.False(t, seen[core], "core IP %s was handed to a gNB", core)
	}
	// N2 takes .2-.4 (AMF is .1); N3 skips those and the UPF at .6
	require.Equal(t, []string{"10.0.1.5", "10.0.1.7", "10.0.1.8"},
		[]string{plan.Gnbs[0].N3IP, plan.Gnbs[1].N3IP, plan.Gnbs[2].N3IP})
}

func TestExpandBoundsN2Timeout(t *testing.T) {
	p := sampleProfile()
	p.Rates.N2.TimeoutMs = 60001
	_, err := Expand(p, nil)
	var verr *ValidationError
	require.ErrorAs(t, err, &verr)
	require.Equal(t, []FieldError{{Field: "rates.n2.timeoutMs", Message: "must be between 1 and 60000"}}, verr.Errors)
}

func TestExpandValidatesUeTemplateAndRates(t *testing.T) {
	p := sampleProfile()
	p.Ue.MsinStart = "000001" // 3+2+6 = 11 digits
	p.Ue.Key = "xyz"
	p.Ue.Integrity = "nia9"
	p.Ue.Dnn = ""
	p.Rates.Registration.RatePerSec = 0
	p.Rates.Pdu.TimeoutMs = 60001
	_, err := Expand(p, nil)
	var verr *ValidationError
	require.ErrorAs(t, err, &verr)
	require.ElementsMatch(t, []FieldError{
		{Field: "ue.msinStart", Message: "must be 10 digits so MCC+MNC+MSIN is 15"},
		{Field: "ue.key", Message: "must be 32 hex digits"},
		{Field: "ue.integrity", Message: "must be one of nia0, nia1, nia2, nia3"},
		{Field: "ue.dnn", Message: "must not be empty"},
		{Field: "rates.registration.ratePerSec", Message: "must be between 1 and 100000"},
		{Field: "rates.pdu.timeoutMs", Message: "must be between 1 and 60000"},
	}, verr.Errors)
}

func TestExpandReportsMsinOverflow(t *testing.T) {
	p := sampleProfile()
	p.Ue.MsinStart = "9999999995" // 10 UEs need ...04
	_, err := Expand(p, nil)
	var verr *ValidationError
	require.ErrorAs(t, err, &verr)
	require.Equal(t, "ue.msinStart", verr.Errors[0].Field)
	require.Contains(t, verr.Errors[0].Message, "overflows")
}

func TestExpandValidatesTrafficAndN6(t *testing.T) {
	p := sampleProfile()
	p.Traffic = Traffic{UlMbps: -1, DlMbps: 20000, PacketSize: 32, Port: 0}
	p.Network.N6 = N6Network{Interface: "", SinkIP: "x", UpfIP: "10.0.3.1", UePool: "10.60.0.0"}
	_, err := Expand(p, nil)
	var verr *ValidationError
	require.ErrorAs(t, err, &verr)
	require.ElementsMatch(t, []FieldError{
		{Field: "traffic.ulMbps", Message: "must be between 0 and 10000"},
		{Field: "traffic.dlMbps", Message: "must be between 0 and 10000"},
		{Field: "traffic.packetSize", Message: "must be between 64 and 1400"},
		{Field: "traffic.port", Message: "must be between 1 and 65535"},
		{Field: "network.n6.interface", Message: "must not be empty"},
		{Field: "network.n6.sinkIp", Message: `"x" is not an IPv4 address`},
		{Field: "network.n6.uePool", Message: `"10.60.0.0" is not an IPv4 CIDR`},
	}, verr.Errors)
}

// fru-lab's core puts N2, N3 and N6 on one bridge: the sink and the UPF's
// N6 address must never be handed to a gNB.
func TestExpandNeverAllocatesTheSinkOrUpfN6IP(t *testing.T) {
	p := sampleProfile()
	p.Network.N6.SinkIP, p.Network.N6.UpfIP = "10.0.1.2", "10.0.1.4"
	plan, err := Expand(p, nil)
	require.NoError(t, err)
	for _, g := range plan.Gnbs {
		require.NotContains(t, []string{"10.0.1.2", "10.0.1.4"}, g.N2IP)
	}
}
```

- [ ] **GREEN:** Replace `profile.go` and `expand.go`, then run `go test ./profile/`. Expected: ok. (`./run` won't build until Task 6.)

```go
// Package profile holds the user-facing description of a throughput test
// and turns it into the concrete per-gNB settings a run needs.
package profile

// Profile is what the setup page edits and what a run starts from. Field
// names are the JSON contract shared with fru-lab's frontend.
type Profile struct {
	Name    string      `json:"name"`
	Scale   Scale       `json:"scale"`
	Gnb     GnbTemplate `json:"gnb"`
	Ue      UeTemplate  `json:"ue"`
	Traffic Traffic     `json:"traffic"`
	Network Network     `json:"network"`
	Rates   Rates       `json:"rates"`
}

// Traffic is what every established UE sends and receives, at a fixed
// rate (design Q10). PacketSize is the inner IP packet's length; Port is
// the UDP port used at the N6 sink and at the (simulated) UEs.
type Traffic struct {
	UlMbps     float64 `json:"ulMbps"` // per UE; 0 = no uplink
	DlMbps     float64 `json:"dlMbps"` // per UE; 0 = no downlink
	PacketSize int     `json:"packetSize"`
	Port       int     `json:"port"`
}

type Scale struct {
	GnbCount int `json:"gnbCount"`
	UeCount  int `json:"ueCount"`
}

// GnbTemplate is expanded once per gNB. GnbIDStart is hex and keeps its
// width when incremented; NamePattern must contain "{i}" (1-based index).
type GnbTemplate struct {
	GnbIDStart  string `json:"gnbIdStart"`
	NamePattern string `json:"namePattern"`
	Mcc         string `json:"mcc"`
	Mnc         string `json:"mnc"`
	Tac         string `json:"tac"`
	Sst         int    `json:"sst"`
	Sd          string `json:"sd"`
}

// UeTemplate is expanded once per UE. MsinStart is decimal and keeps its
// width when incremented; MCC+MNC+MSIN must be 15 digits. The UE uses the
// gNB template's PLMN. Key/Opc/Amf/Sqn must match the subscriber data the
// user created in the core (the tester does not provision subscribers).
type UeTemplate struct {
	MsinStart string `json:"msinStart"`
	Key       string `json:"key"`
	Opc       string `json:"opc"`
	Amf       string `json:"amf"`
	Sqn       string `json:"sqn"`
	Integrity string `json:"integrity"` // nia0..nia3
	Ciphering string `json:"ciphering"` // nea0..nea3
	Dnn       string `json:"dnn"`
	Sst       int    `json:"sst"`
	Sd        string `json:"sd"`
}

type Network struct {
	N2 N2Network `json:"n2"`
	N3 N3Network `json:"n3"`
	N6 N6Network `json:"n6"`
}

// N6Network is the data-network side. Uplink leaves the UPF addressed to
// SinkIP (added to Interface if the host does not have it); downlink is
// sent from SinkIP to the UEs' IPs, which the tester routes via UpfIP
// (UePool via UpfIP dev Interface) for the duration of the run (Q16).
type N6Network struct {
	Interface string `json:"interface"`
	SinkIP    string `json:"sinkIp"`
	UpfIP     string `json:"upfIp"`
	UePool    string `json:"uePool"`
}

// N2Network gives each gNB its own local IP from Cidr, starting at StartIP.
type N2Network struct {
	Interface string `json:"interface"`
	Cidr      string `json:"cidr"`
	StartIP   string `json:"startIp"`
	AmfIP     string `json:"amfIp"`
	AmfPort   int    `json:"amfPort"`
}

// N3Network gives each gNB its own N3 IP from Cidr (design N1); the data
// plane sends uplink from it and the UPF sends downlink to it.
type N3Network struct {
	Interface string `json:"interface"`
	Cidr      string `json:"cidr"`
	StartIP   string `json:"startIp"`
	UpfIP     string `json:"upfIp"`
	UpfPort   int    `json:"upfPort"`
}

type Rates struct {
	N2           StageRate     `json:"n2"`
	Registration ProcedureRate `json:"registration"`
	Pdu          ProcedureRate `json:"pdu"`
}

// ProcedureRate paces a per-UE stage: at most RatePerSec new attempts per
// second (token bucket) and at most MaxInFlight attempts at once
// (semaphore), each bounded by TimeoutMs; Retries as in StageRate.
type ProcedureRate struct {
	RatePerSec  int `json:"ratePerSec"`
	MaxInFlight int `json:"maxInFlight"`
	TimeoutMs   int `json:"timeoutMs"`
	Retries     int `json:"retries"`
}

// StageRate bounds one attempt with TimeoutMs; Retries is how many more
// attempts a failed item gets (it is requeued at the back each time).
type StageRate struct {
	TimeoutMs int `json:"timeoutMs"`
	Retries   int `json:"retries"`
}
```

```go
package profile

import (
	"fmt"
	"net/netip"
	"regexp"
	"strings"
)

// GnbSpec is one concrete gNB a run will bring up. UeFirst/UeLast are the
// 1-based UE indexes it owns (both 0 when it owns none); FirstSupi and
// LastSupi are those UEs' SUPIs, so the user can check them against the
// subscribers in the core.
type GnbSpec struct {
	Index     int    `json:"index"`
	Name      string `json:"name"`
	GnbID     string `json:"gnbId"`
	N2IP      string `json:"n2Ip"`
	N3IP      string `json:"n3Ip"`
	UeCount   int    `json:"ueCount"`
	UeFirst   int    `json:"ueFirst"`
	UeLast    int    `json:"ueLast"`
	FirstSupi string `json:"firstSupi"`
	LastSupi  string `json:"lastSupi"`
}

// UeSpec is one concrete UE. Gnb is the 0-based index into Plan.Gnbs.
type UeSpec struct {
	Index int    // 1-based
	Gnb   int    // 0-based index into Plan.Gnbs
	Msin  string // incremented from UeTemplate.MsinStart
	Supi  string // "imsi-" + MCC + MNC + MSIN
}

// Plan is a profile expanded against a specific host. Ues is left out of
// the JSON: the setup page only needs the per-gNB SUPI ranges, and a full
// UE list would make every validate response O(UE count).
type Plan struct {
	Gnbs      []GnbSpec `json:"gnbs"`
	Ues       []UeSpec  `json:"-"`
	UesPerGnb int       `json:"uesPerGnb"`
	N2Prefix  int       `json:"n2Prefix"`
	N3Prefix  int       `json:"n3Prefix"`
}

var (
	reMcc  = regexp.MustCompile(`^[0-9]{3}$`)
	reMnc  = regexp.MustCompile(`^[0-9]{2,3}$`)
	reHex6 = regexp.MustCompile(`^[0-9a-fA-F]{6}$`)
	reHex  = func(n int) *regexp.Regexp { return regexp.MustCompile(fmt.Sprintf(`^[0-9a-fA-F]{%d}$`, n)) }
	reKey  = reHex(32)
	reAmf  = reHex(4)
	reSqn  = reHex(12)
	reNia  = regexp.MustCompile(`^nia[0-3]$`)
	reNea  = regexp.MustCompile(`^nea[0-3]$`)
)

// Expand validates p and, if it is valid, assigns every gNB its ID, name,
// N2/N3 IPs and UE range. hostIPs are addresses already configured on this
// machine; they are never handed out. All problems are reported together
// in a *ValidationError.
func Expand(p Profile, hostIPs []netip.Addr) (*Plan, error) {
	verr := &ValidationError{}
	validateFields(p, verr)
	if len(verr.Errors) > 0 {
		return nil, verr
	}

	count := p.Scale.GnbCount
	// Neither core IP may be handed out on either side: N2 and N3 may share
	// one interface and CIDR.
	exclude := append([]netip.Addr{
		netip.MustParseAddr(p.Network.N2.AmfIP),
		netip.MustParseAddr(p.Network.N3.UpfIP),
		netip.MustParseAddr(p.Network.N6.UpfIP),
		netip.MustParseAddr(p.Network.N6.SinkIP),
	}, hostIPs...)
	n2IPs, err := AllocateIPs(p.Network.N2.Cidr, p.Network.N2.StartIP, count, exclude)
	if err != nil {
		verr.add("network.n2.cidr", err.Error())
	}
	// ...and N3 must never get an IP already given to a gNB's N2.
	n3Exclude := append(append([]netip.Addr{}, exclude...), n2IPs...)
	n3IPs, err := AllocateIPs(p.Network.N3.Cidr, p.Network.N3.StartIP, count, n3Exclude)
	if err != nil {
		verr.add("network.n3.cidr", err.Error())
	}
	if _, err := IncrementHex(p.Gnb.GnbIDStart, count-1); err != nil {
		verr.add("gnb.gnbIdStart", err.Error())
	}
	if _, err := IncrementDecimal(p.Ue.MsinStart, p.Scale.UeCount-1); err != nil {
		verr.add("ue.msinStart", err.Error())
	}
	if len(verr.Errors) > 0 {
		return nil, verr
	}

	perGnb := (p.Scale.UeCount + count - 1) / count
	plan := &Plan{
		Gnbs:      make([]GnbSpec, 0, count),
		UesPerGnb: perGnb,
		N2Prefix:  netip.MustParsePrefix(p.Network.N2.Cidr).Bits(),
		N3Prefix:  netip.MustParsePrefix(p.Network.N3.Cidr).Bits(),
	}
	plan.Ues = make([]UeSpec, 0, p.Scale.UeCount)
	nextUe := 1
	for i := 0; i < count; i++ {
		id, _ := IncrementHex(p.Gnb.GnbIDStart, i)
		ues := min(perGnb, p.Scale.UeCount-nextUe+1)
		spec := GnbSpec{
			Index:   i + 1,
			Name:    RenderName(p.Gnb.NamePattern, i+1),
			GnbID:   strings.ToLower(id),
			N2IP:    n2IPs[i].String(),
			N3IP:    n3IPs[i].String(),
			UeCount: ues,
		}
		for range ues {
			msin, _ := IncrementDecimal(p.Ue.MsinStart, nextUe-1)
			plan.Ues = append(plan.Ues, UeSpec{
				Index: nextUe, Gnb: i, Msin: msin,
				Supi: "imsi-" + p.Gnb.Mcc + p.Gnb.Mnc + msin,
			})
			nextUe++
		}
		if ues > 0 {
			spec.UeFirst, spec.UeLast = nextUe-ues, nextUe-1
			spec.FirstSupi = plan.Ues[spec.UeFirst-1].Supi
			spec.LastSupi = plan.Ues[spec.UeLast-1].Supi
		}
		plan.Gnbs = append(plan.Gnbs, spec)
	}
	return plan, nil
}

func validateFields(p Profile, verr *ValidationError) {
	if strings.TrimSpace(p.Name) == "" {
		verr.add("name", "must not be empty")
	}
	if p.Scale.GnbCount < 1 {
		verr.add("scale.gnbCount", "must be at least 1")
	}
	if p.Scale.UeCount < 1 {
		verr.add("scale.ueCount", "must be at least 1")
	}
	if len(p.Gnb.GnbIDStart) < 6 || len(p.Gnb.GnbIDStart) > 8 || len(p.Gnb.GnbIDStart)%2 != 0 {
		verr.add("gnb.gnbIdStart", "must be 6 or 8 hex digits")
	} else if _, err := IncrementHex(p.Gnb.GnbIDStart, 0); err != nil {
		verr.add("gnb.gnbIdStart", "must be 6 or 8 hex digits")
	}
	if !strings.Contains(p.Gnb.NamePattern, "{i}") {
		verr.add("gnb.namePattern", `must contain "{i}" so every gNB name is unique`)
	}
	if !reMcc.MatchString(p.Gnb.Mcc) {
		verr.add("gnb.mcc", "must be 3 digits")
	}
	if !reMnc.MatchString(p.Gnb.Mnc) {
		verr.add("gnb.mnc", "must be 2 or 3 digits")
	}
	if !reHex6.MatchString(p.Gnb.Tac) {
		verr.add("gnb.tac", "must be 6 hex digits")
	}
	if p.Gnb.Sst < 0 || p.Gnb.Sst > 255 {
		verr.add("gnb.sst", "must be between 0 and 255")
	}
	if p.Gnb.Sd != "" && !reHex6.MatchString(p.Gnb.Sd) {
		verr.add("gnb.sd", "must be empty or 6 hex digits")
	}
	validateUe(p, verr)
	validateTraffic(p, verr)
	validateEndpoint(verr, "network.n2", p.Network.N2.Interface, p.Network.N2.Cidr, p.Network.N2.StartIP, "amfIp", p.Network.N2.AmfIP, "amfPort", p.Network.N2.AmfPort)
	validateEndpoint(verr, "network.n3", p.Network.N3.Interface, p.Network.N3.Cidr, p.Network.N3.StartIP, "upfIp", p.Network.N3.UpfIP, "upfPort", p.Network.N3.UpfPort)
	// The upper bound keeps Stop prompt: in-flight attempts finish within
	// their timeout before teardown can start.
	if p.Rates.N2.TimeoutMs < 1 || p.Rates.N2.TimeoutMs > 60000 {
		verr.add("rates.n2.timeoutMs", "must be between 1 and 60000")
	}
	if p.Rates.N2.Retries < 0 {
		verr.add("rates.n2.retries", "must not be negative")
	}
	validateProcedureRate(verr, "rates.registration", p.Rates.Registration)
	validateProcedureRate(verr, "rates.pdu", p.Rates.Pdu)
}

func validateUe(p Profile, verr *ValidationError) {
	u := p.Ue
	if u.MsinStart == "" || strings.Trim(u.MsinStart, "0123456789") != "" {
		verr.add("ue.msinStart", "must be decimal digits")
	} else if n := len(p.Gnb.Mcc) + len(p.Gnb.Mnc) + len(u.MsinStart); reMcc.MatchString(p.Gnb.Mcc) && reMnc.MatchString(p.Gnb.Mnc) && n != 15 {
		verr.add("ue.msinStart", fmt.Sprintf("must be %d digits so MCC+MNC+MSIN is 15", 15-len(p.Gnb.Mcc)-len(p.Gnb.Mnc)))
	}
	if !reKey.MatchString(u.Key) {
		verr.add("ue.key", "must be 32 hex digits")
	}
	if !reKey.MatchString(u.Opc) {
		verr.add("ue.opc", "must be 32 hex digits")
	}
	if !reAmf.MatchString(u.Amf) {
		verr.add("ue.amf", "must be 4 hex digits")
	}
	if !reSqn.MatchString(u.Sqn) {
		verr.add("ue.sqn", "must be 12 hex digits")
	}
	if !reNia.MatchString(u.Integrity) {
		verr.add("ue.integrity", "must be one of nia0, nia1, nia2, nia3")
	}
	if !reNea.MatchString(u.Ciphering) {
		verr.add("ue.ciphering", "must be one of nea0, nea1, nea2, nea3")
	}
	if strings.TrimSpace(u.Dnn) == "" {
		verr.add("ue.dnn", "must not be empty")
	}
	if u.Sst < 0 || u.Sst > 255 {
		verr.add("ue.sst", "must be between 0 and 255")
	}
	if u.Sd != "" && !reHex6.MatchString(u.Sd) {
		verr.add("ue.sd", "must be empty or 6 hex digits")
	}
}

func validateTraffic(p Profile, verr *ValidationError) {
	t, n6 := p.Traffic, p.Network.N6
	for field, v := range map[string]float64{"traffic.ulMbps": t.UlMbps, "traffic.dlMbps": t.DlMbps} {
		if v < 0 || v > 10000 {
			verr.add(field, "must be between 0 and 10000")
		}
	}
	if t.PacketSize < 64 || t.PacketSize > 1400 {
		verr.add("traffic.packetSize", "must be between 64 and 1400")
	}
	if t.Port < 1 || t.Port > 65535 {
		verr.add("traffic.port", "must be between 1 and 65535")
	}
	if strings.TrimSpace(n6.Interface) == "" {
		verr.add("network.n6.interface", "must not be empty")
	}
	for field, v := range map[string]string{"network.n6.sinkIp": n6.SinkIP, "network.n6.upfIp": n6.UpfIP} {
		if a, err := netip.ParseAddr(v); err != nil || !a.Is4() {
			verr.add(field, fmt.Sprintf("%q is not an IPv4 address", v))
		}
	}
	if pre, err := netip.ParsePrefix(n6.UePool); err != nil || !pre.Addr().Is4() {
		verr.add("network.n6.uePool", fmt.Sprintf("%q is not an IPv4 CIDR", n6.UePool))
	}
}

func validateProcedureRate(verr *ValidationError, base string, r ProcedureRate) {
	if r.RatePerSec < 1 || r.RatePerSec > 100000 {
		verr.add(base+".ratePerSec", "must be between 1 and 100000")
	}
	if r.MaxInFlight < 1 || r.MaxInFlight > 100000 {
		verr.add(base+".maxInFlight", "must be between 1 and 100000")
	}
	if r.TimeoutMs < 1 || r.TimeoutMs > 60000 {
		verr.add(base+".timeoutMs", "must be between 1 and 60000")
	}
	if r.Retries < 0 {
		verr.add(base+".retries", "must not be negative")
	}
}

func validateEndpoint(verr *ValidationError, base, iface, cidr, start, peerField, peerIP, portField string, port int) {
	if strings.TrimSpace(iface) == "" {
		verr.add(base+".interface", "must not be empty")
	}
	if p, err := netip.ParsePrefix(cidr); err != nil || !p.Addr().Is4() {
		verr.add(base+".cidr", fmt.Sprintf("%q is not an IPv4 CIDR", cidr))
	}
	if a, err := netip.ParseAddr(start); err != nil || !a.Is4() {
		verr.add(base+".startIp", fmt.Sprintf("%q is not an IPv4 address", start))
	}
	if a, err := netip.ParseAddr(peerIP); err != nil || !a.Is4() {
		verr.add(base+"."+peerField, fmt.Sprintf("%q is not an IPv4 address", peerIP))
	}
	if port < 1 || port > 65535 {
		verr.add(base+"."+portField, "must be between 1 and 65535")
	}
}
```

- [ ] Commit `feat: tester traffic and n6 settings`.

### Task 5: Routes

**Files:** Replace `tester/netcfg/netcfg.go`, `tester/netcfg/netcfg_test.go`
**Produces:** `AddrManager.EnsureRoute(iface, dst netip.Prefix, gw netip.Addr) (added bool, err error)` and `RemoveRoute(iface, dst, gw) error`.

- [ ] **RED:** Replace `netcfg_test.go`, then run `go vet ./netcfg/`. Expected: FAIL (`m.EnsureRoute undefined`).

```go
package netcfg

import (
	"net/netip"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
)

// Needs root: sudo FRU_TESTER_NETLINK=1 go test ./netcfg/
func TestNetlinkAddRemoveOnDummyLink(t *testing.T) {
	if os.Getenv("FRU_TESTER_NETLINK") != "1" {
		t.Skip("set FRU_TESTER_NETLINK=1 and run as root to exercise netlink")
	}
	link := &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: "frutest0"}}
	require.NoError(t, netlink.LinkAdd(link))
	t.Cleanup(func() { _ = netlink.LinkDel(link) })
	require.NoError(t, netlink.LinkSetUp(link)) // routes need an up link

	m := Netlink{}
	names, err := m.Interfaces()
	require.NoError(t, err)
	require.Contains(t, names, "frutest0")

	primary := netip.MustParsePrefix("10.250.0.2/24")
	secondary := netip.MustParsePrefix("10.250.0.3/24")
	require.NoError(t, m.Add("frutest0", primary))
	require.NoError(t, m.Add("frutest0", secondary))

	ips, err := m.HostIPv4s()
	require.NoError(t, err)
	require.Contains(t, ips, primary.Addr())
	require.Contains(t, ips, secondary.Addr())

	// removing the primary may take the secondary with it; the second
	// Remove must still succeed.
	require.NoError(t, m.Remove("frutest0", primary))
	require.NoError(t, m.Remove("frutest0", secondary))
	require.NoError(t, m.Remove("frutest0", secondary))

	require.ErrorContains(t, m.Add("no-such-if0", primary), `interface "no-such-if0"`)

	// routes: add once, recognise our own route, refuse a conflicting one
	require.NoError(t, m.Add("frutest0", primary))
	pool := netip.MustParsePrefix("10.251.0.0/16")
	gw := netip.MustParseAddr("10.250.0.9")
	added, err := m.EnsureRoute("frutest0", pool, gw)
	require.NoError(t, err)
	require.True(t, added)
	added, err = m.EnsureRoute("frutest0", pool, gw)
	require.NoError(t, err)
	require.False(t, added, "an identical route is left alone")
	_, err = m.EnsureRoute("frutest0", pool, netip.MustParseAddr("10.250.0.8"))
	require.ErrorContains(t, err, "already routed via 10.250.0.9")
	require.NoError(t, m.RemoveRoute("frutest0", pool, gw))
	require.NoError(t, m.RemoveRoute("frutest0", pool, gw), "removing twice is fine")
}
```

- [ ] **GREEN:** Replace `netcfg.go`, then run `go test -c -o /tmp/netcfg.test ./netcfg/ && sudo FRU_TESTER_NETLINK=1 /tmp/netcfg.test -test.v`. Expected: PASS, and no `frutest0` left behind.

```go
// Package netcfg adds and removes the per-gNB IPs on host interfaces.
package netcfg

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"syscall"

	"github.com/vishvananda/netlink"
)

// AddrManager is what a run needs from the host network stack. The run
// package depends on this interface so tests can use a fake.
type AddrManager interface {
	// HostIPv4s lists every IPv4 address already on any interface.
	HostIPv4s() ([]netip.Addr, error)
	// Interfaces lists the names of every link on the host.
	Interfaces() ([]string, error)
	Add(iface string, addr netip.Prefix) error
	// Remove must treat an already-missing address as success, because the
	// kernel drops secondary addresses when their primary is removed.
	Remove(iface string, addr netip.Prefix) error
	// EnsureRoute makes sure dst is routed via gw on iface. It reports
	// whether it added the route; an identical existing route is left
	// alone (added=false), a different existing route is an error.
	EnsureRoute(iface string, dst netip.Prefix, gw netip.Addr) (added bool, err error)
	// RemoveRoute deletes a route EnsureRoute added; a missing route is
	// not an error.
	RemoveRoute(iface string, dst netip.Prefix, gw netip.Addr) error
}

// Netlink is the real AddrManager; it needs CAP_NET_ADMIN.
type Netlink struct{}

func (Netlink) HostIPv4s() ([]netip.Addr, error) {
	list, err := netlink.AddrList(nil, netlink.FAMILY_V4)
	if err != nil {
		return nil, fmt.Errorf("list host addresses: %w", err)
	}
	out := make([]netip.Addr, 0, len(list))
	for _, a := range list {
		if ip, ok := netip.AddrFromSlice(a.IP.To4()); ok {
			out = append(out, ip)
		}
	}
	return out, nil
}

func (Netlink) Interfaces() ([]string, error) {
	links, err := netlink.LinkList()
	if err != nil {
		return nil, fmt.Errorf("list host interfaces: %w", err)
	}
	names := make([]string, 0, len(links))
	for _, l := range links {
		names = append(names, l.Attrs().Name)
	}
	return names, nil
}

func (Netlink) Add(iface string, addr netip.Prefix) error {
	link, err := netlink.LinkByName(iface)
	if err != nil {
		return fmt.Errorf("interface %q: %w", iface, err)
	}
	if err := netlink.AddrAdd(link, toNetlink(addr)); err != nil {
		return fmt.Errorf("add %s to %s: %w", addr, iface, err)
	}
	return nil
}

func (Netlink) Remove(iface string, addr netip.Prefix) error {
	link, err := netlink.LinkByName(iface)
	if err != nil {
		return fmt.Errorf("interface %q: %w", iface, err)
	}
	if err := netlink.AddrDel(link, toNetlink(addr)); err != nil {
		if errors.Is(err, syscall.EADDRNOTAVAIL) {
			return nil
		}
		return fmt.Errorf("remove %s from %s: %w", addr, iface, err)
	}
	return nil
}

func (Netlink) EnsureRoute(iface string, dst netip.Prefix, gw netip.Addr) (bool, error) {
	link, err := netlink.LinkByName(iface)
	if err != nil {
		return false, fmt.Errorf("interface %q: %w", iface, err)
	}
	existing, err := netlink.RouteListFiltered(netlink.FAMILY_V4,
		&netlink.Route{Dst: ipNet(dst)}, netlink.RT_FILTER_DST)
	if err != nil {
		return false, fmt.Errorf("list routes to %s: %w", dst, err)
	}
	if len(existing) > 0 {
		r := existing[0]
		if r.Gw.Equal(net.IP(gw.AsSlice())) && r.LinkIndex == link.Attrs().Index {
			return false, nil
		}
		return false, fmt.Errorf("%s is already routed via %s; remove that route or change the UE pool", dst, r.Gw)
	}
	if err := netlink.RouteAdd(&netlink.Route{LinkIndex: link.Attrs().Index, Dst: ipNet(dst), Gw: net.IP(gw.AsSlice())}); err != nil {
		return false, fmt.Errorf("route %s via %s dev %s: %w", dst, gw, iface, err)
	}
	return true, nil
}

func (Netlink) RemoveRoute(iface string, dst netip.Prefix, gw netip.Addr) error {
	link, err := netlink.LinkByName(iface)
	if err != nil {
		return fmt.Errorf("interface %q: %w", iface, err)
	}
	err = netlink.RouteDel(&netlink.Route{LinkIndex: link.Attrs().Index, Dst: ipNet(dst), Gw: net.IP(gw.AsSlice())})
	if err != nil && !errors.Is(err, syscall.ESRCH) {
		return fmt.Errorf("remove route %s via %s: %w", dst, gw, err)
	}
	return nil
}

func ipNet(p netip.Prefix) *net.IPNet {
	p = p.Masked()
	ip := p.Addr().As4()
	return &net.IPNet{IP: net.IP(ip[:]), Mask: net.CIDRMask(p.Bits(), 32)}
}

func toNetlink(p netip.Prefix) *netlink.Addr {
	ip := p.Addr().As4()
	return &netlink.Addr{IPNet: &net.IPNet{
		IP:   net.IP(ip[:]),
		Mask: net.CIDRMask(p.Bits(), 32),
	}}
}
```

- [ ] Commit `feat: add and remove the ue pool route`.

### Task 6: Run controller — configure, start traffic, stop traffic

**Files:** Replace `tester/run/controller.go`, `tester/run/snapshot.go`, `tester/run/controller_test.go`, `tester/run/pipeline_e2e_test.go`
**Consumes:** Tasks 1–5.
**Produces:**
- `run.Deps.NewDataplane func(dataplane.Config) Dataplane` and the `run.Dataplane` interface.
- `Snapshot.Dataplane dataplane.Snapshot`. It is always present, with empty slices before the engine exists.
- Configuration order:
  1. per gNB: N2 then N3 IP;
  2. the sink as /32 if missing;
  3. the route (if absent).
- Teardown removes the routes, then the addresses in reverse order.

- [ ] **RED:** Replace both test files, then run `go vet ./run/`. Expected: FAIL (`unknown field NewDataplane`, `snap.Dataplane`).

```go
package run

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"sync"
	"syscall"
	"testing"
	"time"

	"maps"

	loggergo "github.com/Alonza0314/logger-go/v2"
	"github.com/stretchr/testify/require"

	"tester/dataplane"
	"tester/gnb"
	"tester/metrics"
	"tester/profile"
)

// fakeAddrs records Add/Remove calls; failAdd makes the Nth Add fail.
// onAdd, if set, runs after every successful Add.
type fakeAddrs struct {
	mu      sync.Mutex
	host    []netip.Addr
	onAdd   func()
	present []netip.Prefix
	log     []string
	failAdd int // 1-based; 0 = never
	adds    int
}

func (f *fakeAddrs) HostIPv4s() ([]netip.Addr, error) { return f.host, nil }
func (f *fakeAddrs) Interfaces() ([]string, error) {
	return []string{"lo", "eth-n2", "eth-n3", "eth-n6"}, nil
}

func (f *fakeAddrs) EnsureRoute(iface string, dst netip.Prefix, gw netip.Addr) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.log = append(f.log, "route "+dst.String()+" via "+gw.String()+" dev "+iface)
	return true, nil
}

func (f *fakeAddrs) RemoveRoute(iface string, dst netip.Prefix, gw netip.Addr) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.log = append(f.log, "unroute "+dst.String())
	return nil
}

// fakeDataplane records the UEs handed to the data plane.
type fakeDataplane struct {
	mu       sync.Mutex
	cfg      dataplane.Config
	ues      map[int]fakeFlow
	stopped  bool
	startErr error
}

type fakeFlow struct {
	gnb            int
	ueIP           netip.Addr
	ulTeid, dlTeid uint32
	upf            netip.AddrPort
}

func (d *fakeDataplane) Start() error { return d.startErr }
func (d *fakeDataplane) AddUE(ue, gnb int, ueIP netip.Addr, ul, dl uint32, upf netip.AddrPort) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.ues[ue] = fakeFlow{gnb: gnb, ueIP: ueIP, ulTeid: ul, dlTeid: dl, upf: upf}
}
func (d *fakeDataplane) Stop(time.Duration) { d.mu.Lock(); d.stopped = true; d.mu.Unlock() }
func (d *fakeDataplane) Snapshot() dataplane.Snapshot {
	d.mu.Lock()
	defer d.mu.Unlock()
	return dataplane.Snapshot{ActiveUes: len(d.ues), Gnbs: []dataplane.GnbTraffic{}, Series: []dataplane.Point{}}
}

func (d *fakeDataplane) flows() map[int]fakeFlow {
	d.mu.Lock()
	defer d.mu.Unlock()
	return maps.Clone(d.ues)
}

func (f *fakeAddrs) Add(iface string, p netip.Prefix) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.adds++
	if f.adds == f.failAdd {
		return errors.New("operation not permitted")
	}
	f.present = append(f.present, p)
	f.log = append(f.log, "add "+iface+" "+p.String())
	if f.onAdd != nil {
		f.mu.Unlock()
		f.onAdd()
		f.mu.Lock()
	}
	return nil
}

func (f *fakeAddrs) Remove(iface string, p netip.Prefix) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, q := range f.present {
		if q == p {
			f.present = append(f.present[:i], f.present[i+1:]...)
		}
	}
	f.log = append(f.log, "remove "+iface+" "+p.String())
	return nil
}

func (f *fakeAddrs) snapshot() ([]netip.Prefix, []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]netip.Prefix(nil), f.present...), append([]string(nil), f.log...)
}

// fakeConn answers the first Read (the NG Setup answer) with reply, then
// blocks later reads until the conn is closed or dropped (drop simulates
// the AMF tearing down the association).
type fakeConn struct {
	reply  func() ([]byte, error)
	closed chan struct{}
	drop   chan struct{}
	once   sync.Once
	reads  int
	// closeDelay mimics free5gc/sctp's Close blocking up to 1 s (SO_LINGER)
	closeDelay time.Duration
	// interrupts is how many reads after the NG Setup answer fail with EINTR
	interrupts int
}

func (c *fakeConn) Write(b []byte) (int, error) { return len(b), nil }
func (c *fakeConn) Read(b []byte) (int, error) {
	c.reads++
	if c.reads == 1 {
		raw, err := c.reply()
		if err != nil {
			return 0, err
		}
		return copy(b, raw), nil
	}
	if c.reads <= 1+c.interrupts {
		return 0, syscall.EINTR
	}
	select {
	case <-c.closed:
		return 0, net.ErrClosed
	case <-c.drop:
		return 0, io.EOF
	}
}
func (c *fakeConn) Close() error {
	c.once.Do(func() { close(c.closed) })
	time.Sleep(c.closeDelay)
	return nil
}

// fakeDialer: script[localIP] returns per-attempt behaviour.
type fakeDialer struct {
	mu     sync.Mutex
	script func(localIP string, attempt int) (reply func() ([]byte, error), dialErr error)
	counts map[string]int
	opened []*fakeConn
	// closeDelay and interrupts are copied into every conn this dialer opens
	closeDelay time.Duration
	interrupts int
}

func (d *fakeDialer) Dial(localIP, amfIP string, amfPort int, timeout time.Duration) (gnb.Conn, error) {
	d.mu.Lock()
	d.counts[localIP]++
	n := d.counts[localIP]
	d.mu.Unlock()
	reply, dialErr := d.script(localIP, n)
	if dialErr != nil {
		return nil, dialErr
	}
	c := &fakeConn{reply: reply, closed: make(chan struct{}), drop: make(chan struct{}), closeDelay: d.closeDelay, interrupts: d.interrupts}
	d.mu.Lock()
	d.opened = append(d.opened, c)
	d.mu.Unlock()
	return c, nil
}

func newFakeDialer(script func(string, int) (func() ([]byte, error), error)) *fakeDialer {
	return &fakeDialer{script: script, counts: map[string]int{}}
}

func accept(t *testing.T) func() ([]byte, error) {
	b := ngSetupResponseBytes(t)
	return func() ([]byte, error) { return b, nil }
}

func testProfile() profile.Profile {
	return profile.Profile{
		Name:  "unit",
		Scale: profile.Scale{GnbCount: 3, UeCount: 10},
		Gnb: profile.GnbTemplate{GnbIDStart: "000314", NamePattern: "gNB-{i}",
			Mcc: "208", Mnc: "93", Tac: "000001", Sst: 1, Sd: "010203"},
		Ue: profile.UeTemplate{MsinStart: "0000000001", Key: "8baf473f2f8fd09487cccbd7097c6862",
			Opc: "8e27b6af0e692e750f32667a3b14605d", Amf: "8000", Sqn: "000000000023",
			Integrity: "nia2", Ciphering: "nea0", Dnn: "internet", Sst: 1, Sd: "010203"},
		Network: profile.Network{
			N2: profile.N2Network{Interface: "eth-n2", Cidr: "10.0.1.0/24", StartIP: "10.0.1.10", AmfIP: "10.0.1.1", AmfPort: 38412},
			N3: profile.N3Network{Interface: "eth-n3", Cidr: "10.0.2.0/24", StartIP: "10.0.2.10", UpfIP: "10.0.2.1", UpfPort: 2152},
			N6: profile.N6Network{Interface: "eth-n6", SinkIP: "10.0.3.2", UpfIP: "10.0.3.1", UePool: "10.60.0.0/16"},
		},
		Traffic: profile.Traffic{UlMbps: 1, DlMbps: 5, PacketSize: 1400, Port: 9200},
		// N2-only tests use fakeConn, which never answers NAS: keep the UE
		// stages short so Stop does not wait long for their timeouts.
		Rates: profile.Rates{
			N2:           profile.StageRate{TimeoutMs: 1000, Retries: 1},
			Registration: profile.ProcedureRate{RatePerSec: 1000, MaxInFlight: 100, TimeoutMs: 100, Retries: 0},
			Pdu:          profile.ProcedureRate{RatePerSec: 1000, MaxInFlight: 100, TimeoutMs: 100, Retries: 0},
		},
	}
}

func newTestController(addrs *fakeAddrs, dialer *fakeDialer) *Controller {
	lg := loggergo.NewLogger("", true) // debugMode=true logs to stdout instead of a file
	lg.SetLevel("error")
	return NewController(Deps{Addrs: addrs, Dialer: dialer, Log: lg.WithTags("TEST"), NewID: func() string { return "run-1" },
		NewDataplane: func(c dataplane.Config) Dataplane { return &fakeDataplane{cfg: c, ues: map[int]fakeFlow{}} }})
}

// waitFor blocks until ok(snapshot) holds, re-checking on every change.
func waitFor(t *testing.T, c *Controller, what string, ok func(Snapshot) bool) Snapshot {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		ch := c.Changed()
		snap := c.Snapshot()
		if ok(snap) {
			return snap
		}
		select {
		case <-ch:
		case <-deadline:
			t.Fatalf("timed out waiting for %s; state=%q error=%q", what, snap.State, snap.Error)
		}
	}
}

func waitState(t *testing.T, c *Controller, want State) Snapshot {
	t.Helper()
	return waitFor(t, c, "state "+string(want), func(s Snapshot) bool { return s.State == want })
}

func TestRunAllGnbsUpThenStopCleansUp(t *testing.T) {
	addrs := &fakeAddrs{}
	dialer := newFakeDialer(func(string, int) (func() ([]byte, error), error) { return accept(t), nil })
	c := newTestController(addrs, dialer)

	_, err := c.Start(testProfile())
	require.NoError(t, err)
	snap := waitState(t, c, StateRunning)

	require.Equal(t, int64(3), snap.N2.Accepted)
	require.True(t, snap.N2.Done)
	for _, g := range snap.Gnbs {
		require.Equal(t, GnbUp, g.State)
		require.Equal(t, 1, g.Attempts)
	}
	present, _ := addrs.snapshot()
	require.Len(t, present, 7) // N2 + N3 per gNB, and the N6 sink

	_, err = c.Stop()
	require.NoError(t, err)
	snap = waitState(t, c, StateStopped)
	require.NotNil(t, snap.StoppedAt)
	for _, g := range snap.Gnbs {
		require.Equal(t, GnbClosed, g.State)
	}
	for _, conn := range dialer.opened {
		<-conn.closed
	}
	present, log := addrs.snapshot()
	require.Empty(t, present)
	require.Equal(t, []string{
		"add eth-n2 10.0.1.10/24", "add eth-n3 10.0.2.10/24",
		"add eth-n2 10.0.1.11/24", "add eth-n3 10.0.2.11/24",
		"add eth-n2 10.0.1.12/24", "add eth-n3 10.0.2.12/24",
		"add eth-n6 10.0.3.2/32", "route 10.60.0.0/16 via 10.0.3.1 dev eth-n6",
		"unroute 10.60.0.0/16",
		"remove eth-n6 10.0.3.2/32",
		"remove eth-n3 10.0.2.12/24", "remove eth-n2 10.0.1.12/24",
		"remove eth-n3 10.0.2.11/24", "remove eth-n2 10.0.1.11/24",
		"remove eth-n3 10.0.2.10/24", "remove eth-n2 10.0.1.10/24",
	}, log)
}

func TestRunRetriesThenClassifiesFinalOutcome(t *testing.T) {
	addrs := &fakeAddrs{}
	reject := ngSetupFailureBytes(t)
	dialer := newFakeDialer(func(ip string, attempt int) (func() ([]byte, error), error) {
		switch ip {
		case "10.0.1.10": // fails once, then succeeds on the retry
			if attempt == 1 {
				return nil, fmt.Errorf("sctp connect: %w", syscall.ECONNREFUSED)
			}
			return accept(t), nil
		case "10.0.1.11": // AMF rejects every time
			return func() ([]byte, error) { return reject, nil }, nil
		default: // AMF never answers
			return func() ([]byte, error) { return nil, fmt.Errorf("read: %w", syscall.EAGAIN) }, nil
		}
	})
	c := newTestController(addrs, dialer)

	_, err := c.Start(testProfile())
	require.NoError(t, err)
	snap := waitState(t, c, StateRunning)

	require.Equal(t, int64(3), snap.N2.Attempted)
	require.Equal(t, int64(3), snap.N2.Retries)
	require.Equal(t, int64(1), snap.N2.Accepted)
	require.Equal(t, int64(1), snap.N2.Rejected)
	require.Equal(t, int64(1), snap.N2.TimedOut)
	require.Equal(t, []metrics.CauseCount{{Cause: "misc(4)", Count: 1}, {Cause: "resource temporarily unavailable", Count: 1}}, snap.N2.Causes)
	require.Equal(t, GnbUp, snap.Gnbs[0].State)
	require.Equal(t, 2, snap.Gnbs[0].Attempts)
	require.Empty(t, snap.Gnbs[0].Cause)
	require.Equal(t, GnbFailed, snap.Gnbs[1].State)
	require.Equal(t, "ng setup rejected: misc(4)", snap.Gnbs[1].Cause)
	require.Equal(t, GnbFailed, snap.Gnbs[2].State)
}

func TestStartRejectsSecondRunAndInvalidProfile(t *testing.T) {
	addrs := &fakeAddrs{}
	dialer := newFakeDialer(func(string, int) (func() ([]byte, error), error) { return accept(t), nil })
	c := newTestController(addrs, dialer)

	bad := testProfile()
	bad.Scale.GnbCount = 0
	_, err := c.Start(bad)
	var verr *profile.ValidationError
	require.ErrorAs(t, err, &verr)
	require.Equal(t, StateIdle, c.Snapshot().State)

	_, err = c.Start(testProfile())
	require.NoError(t, err)
	_, err = c.Start(testProfile())
	require.ErrorIs(t, err, ErrRunActive)

	waitState(t, c, StateRunning)
	_, err = c.Stop()
	require.NoError(t, err)
	waitState(t, c, StateStopped)
	_, err = c.Stop()
	require.ErrorIs(t, err, ErrNotRunning)

	_, err = c.Start(testProfile()) // allowed again once stopped
	require.NoError(t, err)
	c.Shutdown()
	require.Equal(t, StateStopped, c.Snapshot().State)
}

func TestHostIPsAreSkippedWhenAllocating(t *testing.T) {
	addrs := &fakeAddrs{host: []netip.Addr{netip.MustParseAddr("10.0.1.11")}}
	dialer := newFakeDialer(func(string, int) (func() ([]byte, error), error) { return accept(t), nil })
	c := newTestController(addrs, dialer)
	plan, err := c.Validate(testProfile())
	require.NoError(t, err)
	require.Equal(t, []string{"10.0.1.10", "10.0.1.12", "10.0.1.13"},
		[]string{plan.Gnbs[0].N2IP, plan.Gnbs[1].N2IP, plan.Gnbs[2].N2IP})
}

func TestIPConfigFailureRollsBackAndFails(t *testing.T) {
	addrs := &fakeAddrs{failAdd: 5} // gNB-3's N2 IP (each gNB adds N2 then N3)
	dialer := newFakeDialer(func(string, int) (func() ([]byte, error), error) { return accept(t), nil })
	c := newTestController(addrs, dialer)

	_, err := c.Start(testProfile())
	require.NoError(t, err)
	snap := waitState(t, c, StateFailed)
	require.Contains(t, snap.Error, "configure gNB-3: operation not permitted")
	require.Equal(t, int64(0), snap.N2.Attempted)
	present, _ := addrs.snapshot()
	require.Empty(t, present)
	require.Empty(t, dialer.counts)
}

func TestStopDuringN2LeavesQueuedGnbsPending(t *testing.T) {
	addrs := &fakeAddrs{}
	release := make(chan struct{})
	dialer := newFakeDialer(func(string, int) (func() ([]byte, error), error) {
		return func() ([]byte, error) {
			<-release // hold every NG Setup until the test lets go
			return nil, fmt.Errorf("read: %w", syscall.EAGAIN)
		}, nil
	})
	p := testProfile()
	p.Rates.N2.Retries = 3
	c := newTestController(addrs, dialer)
	_, err := c.Start(p)
	require.NoError(t, err)
	waitFor(t, c, "3 attempts in flight", func(s Snapshot) bool { return s.N2.InFlight == 3 })

	_, err = c.Stop()
	require.NoError(t, err)
	close(release)
	snap := waitState(t, c, StateStopped)
	require.Equal(t, int64(3), snap.N2.Attempted)
	require.Equal(t, int64(0), snap.N2.Retries, "no retries after Stop")
	present, _ := addrs.snapshot()
	require.Empty(t, present)
}

func TestValidateFlagsUnknownInterface(t *testing.T) {
	c := newTestController(&fakeAddrs{}, newFakeDialer(nil))
	p := testProfile()
	p.Network.N2.Interface = "ens199"
	_, err := c.Validate(p)
	var verr *profile.ValidationError
	require.ErrorAs(t, err, &verr)
	require.Equal(t, []profile.FieldError{{Field: "network.n2.interface", Message: `no interface named "ens199" on this host`}}, verr.Errors)

	p.Scale.GnbCount = 0 // other field errors are reported alongside
	_, err = c.Validate(p)
	require.ErrorAs(t, err, &verr)
	require.Len(t, verr.Errors, 2)
}

func TestStopWhileConfiguringSkipsN2(t *testing.T) {
	var c *Controller
	addrs := &fakeAddrs{}
	addrs.onAdd = func() {
		addrs.onAdd = nil
		_, _ = c.Stop() // press Stop right after the first IP is added
	}
	dialer := newFakeDialer(func(string, int) (func() ([]byte, error), error) { return accept(t), nil })
	c = newTestController(addrs, dialer)

	_, err := c.Start(testProfile())
	require.NoError(t, err)
	snap := waitState(t, c, StateStopped)
	require.Equal(t, int64(0), snap.N2.Attempted)
	require.Empty(t, dialer.counts)
	present, _ := addrs.snapshot()
	require.Empty(t, present)
}

func TestLostAssociationIsReported(t *testing.T) {
	addrs := &fakeAddrs{}
	dialer := newFakeDialer(func(string, int) (func() ([]byte, error), error) { return accept(t), nil })
	c := newTestController(addrs, dialer)
	_, err := c.Start(testProfile())
	require.NoError(t, err)
	waitState(t, c, StateRunning)

	dialer.mu.Lock()
	close(dialer.opened[0].drop) // the AMF drops one association
	dialer.mu.Unlock()

	snap := waitFor(t, c, "one gNB lost", func(s Snapshot) bool {
		for _, g := range s.Gnbs {
			if g.State == GnbLost {
				return true
			}
		}
		return false
	})
	lost := 0
	for _, g := range snap.Gnbs {
		if g.State == GnbLost {
			lost++
			require.Equal(t, "association lost: EOF", g.Cause)
		}
	}
	require.Equal(t, 1, lost)
	require.Equal(t, StateRunning, c.Snapshot().State, "one lost gNB does not end the run")
	_, err = c.Stop()
	require.NoError(t, err)
	waitState(t, c, StateStopped)
}

// Each SCTP Close can block up to 1 s (SO_LINGER); closing serially would
// outlast docker stop's grace period with many gNBs and leak their IPs.
func TestStopClosesAssociationsConcurrently(t *testing.T) {
	addrs := &fakeAddrs{}
	dialer := newFakeDialer(func(string, int) (func() ([]byte, error), error) { return accept(t), nil })
	dialer.closeDelay = 300 * time.Millisecond
	c := newTestController(addrs, dialer)
	_, err := c.Start(testProfile())
	require.NoError(t, err)
	waitState(t, c, StateRunning)

	start := time.Now()
	_, err = c.Stop()
	require.NoError(t, err)
	waitState(t, c, StateStopped)
	require.Less(t, time.Since(start), 600*time.Millisecond, "3 closes of 300 ms each should overlap")
}

// A recvmsg with SO_RCVTIMEO set is not restarted after a signal; EINTR on
// a healthy association must not mark the gNB lost.
func TestInterruptedReadDoesNotLoseGnb(t *testing.T) {
	addrs := &fakeAddrs{}
	dialer := newFakeDialer(func(string, int) (func() ([]byte, error), error) { return accept(t), nil })
	dialer.interrupts = 3
	c := newTestController(addrs, dialer)
	_, err := c.Start(testProfile())
	require.NoError(t, err)
	waitState(t, c, StateRunning)

	time.Sleep(100 * time.Millisecond) // let every watchConn hit its EINTRs
	for _, g := range c.Snapshot().Gnbs {
		require.Equal(t, GnbUp, g.State, "gNB %s: %s", g.Name, g.Cause)
	}
	_, err = c.Stop()
	require.NoError(t, err)
	waitState(t, c, StateStopped)
}
```

```go
package run

import (
	"encoding/json"
	"errors"
	"net/netip"
	"testing"
	"time"

	loggergo "github.com/Alonza0314/logger-go/v2"
	"github.com/stretchr/testify/require"

	"tester/dataplane"
	"tester/gnb"
	"tester/internal/fakecore"
	"tester/metrics"
	"tester/profile"
)

// amfDialer connects every gNB to the same fake AMF over a fresh pipe;
// IPs listed in refuse fail to connect.
type amfDialer struct {
	amf    *fakecore.AMF
	refuse map[string]bool
}

func (d amfDialer) Dial(localIP, _ string, _ int, _ time.Duration) (gnb.Conn, error) {
	if d.refuse[localIP] {
		return nil, errors.New("connection refused")
	}
	g, a := fakecore.Pipe()
	go func() { _ = d.amf.Serve(a) }()
	return g, nil
}

func newFakeAMF(behavior func(supi string) fakecore.Behavior) *fakecore.AMF {
	return &fakecore.AMF{
		Subscriber: fakecore.Subscriber{Mcc: "208", Mnc: "93",
			Key: "8baf473f2f8fd09487cccbd7097c6862", Opc: "8e27b6af0e692e750f32667a3b14605d",
			Amf: "8000", Sqn: "000000000023"},
		Behavior: behavior,
		UpfIP:    netip.MustParseAddr("10.0.1.5"),
	}
}

func e2eProfile() profile.Profile {
	p := testProfile()
	p.Rates.Registration = profile.ProcedureRate{RatePerSec: 1000, MaxInFlight: 100, TimeoutMs: 2000, Retries: 1}
	p.Rates.Pdu = profile.ProcedureRate{RatePerSec: 1000, MaxInFlight: 100, TimeoutMs: 2000, Retries: 1}
	return p
}

func newE2EController(d amfDialer) (*Controller, *fakeAddrs) {
	c, addrs, _ := newE2EControllerWithDataplane(d, nil)
	return c, addrs
}

// newE2EControllerWithDataplane also returns the fake data plane; startErr
// makes its Start fail.
func newE2EControllerWithDataplane(d amfDialer, startErr error) (*Controller, *fakeAddrs, *fakeDataplane) {
	lg := loggergo.NewLogger("", true)
	lg.SetLevel("error")
	addrs := &fakeAddrs{}
	dp := &fakeDataplane{ues: map[int]fakeFlow{}, startErr: startErr}
	c := NewController(Deps{Addrs: addrs, Dialer: d, Log: lg.WithTags("TEST"), NewID: func() string { return "e2e" },
		NewDataplane: func(cfg dataplane.Config) Dataplane { dp.cfg = cfg; return dp }})
	return c, addrs, dp
}

func stopAndWait(t *testing.T, c *Controller) Snapshot {
	t.Helper()
	_, err := c.Stop()
	require.NoError(t, err)
	return waitState(t, c, StateStopped)
}

func TestRunRegistersAndEstablishesEveryUe(t *testing.T) {
	amf := newFakeAMF(nil)
	c, addrs := newE2EController(amfDialer{amf: amf})
	_, err := c.Start(e2eProfile())
	require.NoError(t, err)

	snap := waitFor(t, c, "10 UEs established", func(s Snapshot) bool { return s.Ues.Established == 10 })
	require.Equal(t, StateRunning, snap.State)
	for _, st := range []metrics.StageSnapshot{snap.Registration, snap.Pdu} {
		require.Equal(t, int64(10), st.Accepted, st.Name)
		require.True(t, st.Done, st.Name)
		require.Greater(t, st.AvgMs, 0.0, st.Name)
	}
	require.Equal(t, []int{4, 4, 2}, []int{snap.Gnbs[0].Established, snap.Gnbs[1].Established, snap.Gnbs[2].Established})
	require.Equal(t, []int{4, 4, 2}, []int{snap.Gnbs[0].Registered, snap.Gnbs[1].Registered, snap.Gnbs[2].Registered})
	require.Empty(t, snap.FailedUes)
	require.Eventually(t, func() bool { return len(amf.SetupResponses()) == 10 }, time.Second, 5*time.Millisecond)

	snap = stopAndWait(t, c)
	require.Equal(t, UeSummary{Established: 10}, snap.Ues)
	present, _ := addrs.snapshot()
	require.Empty(t, present)
}

func TestRegistrationRejectIsRetriedThenListed(t *testing.T) {
	amf := newFakeAMF(func(supi string) fakecore.Behavior {
		if supi == "imsi-208930000000003" {
			return fakecore.Behavior{RejectRegistration: 7}
		}
		return fakecore.Behavior{}
	})
	c, _ := newE2EController(amfDialer{amf: amf})
	_, err := c.Start(e2eProfile())
	require.NoError(t, err)

	snap := waitFor(t, c, "pdu stage done", func(s Snapshot) bool { return s.Pdu.Done })
	require.Equal(t, int64(9), snap.Registration.Accepted)
	require.Equal(t, int64(1), snap.Registration.Rejected)
	require.Equal(t, int64(1), snap.Registration.Retries)
	require.Equal(t, []metrics.CauseCount{{Cause: "5gmm(7)", Count: 1}}, snap.Registration.Causes)
	require.Equal(t, int64(1), snap.Pdu.Skipped, "a UE that never registered skips the PDU stage")
	require.Equal(t, []UeFailure{{Supi: "imsi-208930000000003", Gnb: "gNB-1", Stage: "registration", Cause: "5gmm(7)", Attempts: 2}}, snap.FailedUes)
	require.Equal(t, UeSummary{Established: 9, Failed: 1}, snap.Ues)
	stopAndWait(t, c)
}

func TestPduRejectIsListed(t *testing.T) {
	amf := newFakeAMF(func(supi string) fakecore.Behavior {
		if supi == "imsi-208930000000010" {
			return fakecore.Behavior{RejectPdu: 27}
		}
		return fakecore.Behavior{}
	})
	c, _ := newE2EController(amfDialer{amf: amf})
	p := e2eProfile()
	p.Rates.Pdu.Retries = 0
	_, err := c.Start(p)
	require.NoError(t, err)
	snap := waitFor(t, c, "pdu stage done", func(s Snapshot) bool { return s.Pdu.Done })
	require.Equal(t, []UeFailure{{Supi: "imsi-208930000000010", Gnb: "gNB-3", Stage: "pdu", Cause: "5gsm(27)", Attempts: 1}}, snap.FailedUes)
	require.Equal(t, 1, snap.Gnbs[2].Established)
	require.Equal(t, 2, snap.Gnbs[2].Registered)
	stopAndWait(t, c)
}

func TestUesOfFailedGnbAreSkipped(t *testing.T) {
	c, _ := newE2EController(amfDialer{amf: newFakeAMF(nil), refuse: map[string]bool{"10.0.1.12": true}})
	_, err := c.Start(e2eProfile())
	require.NoError(t, err)
	snap := waitFor(t, c, "pdu stage done", func(s Snapshot) bool { return s.Pdu.Done })
	require.Equal(t, GnbFailed, snap.Gnbs[2].State)
	require.Equal(t, int64(2), snap.Registration.Skipped)
	require.Equal(t, int64(8), snap.Registration.Accepted)
	require.Equal(t, UeSummary{Established: 8, Skipped: 2}, snap.Ues)
	stopAndWait(t, c)
}

func TestStopDuringRegistrationCancelsQueuedUes(t *testing.T) {
	c, _ := newE2EController(amfDialer{amf: newFakeAMF(nil)})
	p := e2eProfile()
	p.Rates.Registration.RatePerSec = 5 // 10 UEs would take ~2 s
	_, err := c.Start(p)
	require.NoError(t, err)
	waitFor(t, c, "one UE established", func(s Snapshot) bool { return s.Ues.Established >= 1 })

	snap := stopAndWait(t, c)
	require.True(t, snap.Registration.Done, "stopped stage must be complete")
	require.True(t, snap.Pdu.Done)
	require.Positive(t, snap.Ues.Cancelled)
	require.Equal(t, int64(snap.Ues.Cancelled), snap.Registration.Skipped)
	require.Equal(t, 10, snap.Ues.Established+snap.Ues.Cancelled)
	frozen := snap.Registration.TotalTimeMs
	time.Sleep(50 * time.Millisecond)
	require.Equal(t, frozen, c.Snapshot().Registration.TotalTimeMs, "total time stops at the last finish")
}

// A run where every UE succeeds must still send "failedUes": [] — the
// Run page reads failedUes.length, and null blanked it.
func TestSnapshotJSONHasEmptyFailedUesNotNull(t *testing.T) {
	c, _ := newE2EController(amfDialer{amf: newFakeAMF(nil)})
	_, err := c.Start(e2eProfile())
	require.NoError(t, err)
	waitFor(t, c, "10 UEs established", func(s Snapshot) bool { return s.Ues.Established == 10 })
	raw, err := json.Marshal(c.Snapshot())
	require.NoError(t, err)
	require.Contains(t, string(raw), `"failedUes":[]`)
	stopAndWait(t, c)
}

func TestEstablishedUesStartTrafficWithTheirTunnel(t *testing.T) {
	c, _, dp := newE2EControllerWithDataplane(amfDialer{amf: newFakeAMF(nil)}, nil)
	_, err := c.Start(e2eProfile())
	require.NoError(t, err)
	waitFor(t, c, "10 UEs established", func(s Snapshot) bool { return s.Ues.Established == 10 })

	require.Equal(t, uint16(9200), dp.cfg.Port)
	require.Equal(t, 1400, dp.cfg.PacketSize)
	require.Equal(t, 5e6, dp.cfg.DlBps)
	require.Equal(t, []netip.Addr{netip.MustParseAddr("10.0.2.10"), netip.MustParseAddr("10.0.2.11"), netip.MustParseAddr("10.0.2.12")}, dp.cfg.GnbN3IPs)
	flows := dp.flows()
	require.Len(t, flows, 10)
	teids := map[uint32]bool{}
	for ue, f := range flows {
		require.Equal(t, ue/4, f.gnb, "UE %d", ue)
		require.Equal(t, netip.MustParseAddrPort("10.0.1.5:2152"), f.upf, "UPF N3 from the PDU Session Resource Setup")
		require.Equal(t, uint32(0x1000), f.ulTeid&0xffffff00, "UL TEID from the AMF's transfer")
		teids[f.dlTeid] = true
	}
	require.Len(t, teids, 10)
	stopAndWait(t, c)
	require.True(t, dp.stopped)
}

func TestDataplaneStartFailureFailsTheRunAndRollsBack(t *testing.T) {
	c, addrs, _ := newE2EControllerWithDataplane(amfDialer{amf: newFakeAMF(nil)}, errors.New("bind gNB-1 N3 10.0.2.10:2152: address already in use"))
	_, err := c.Start(e2eProfile())
	require.NoError(t, err)
	snap := waitState(t, c, StateFailed)
	require.Contains(t, snap.Error, "start data plane: bind gNB-1 N3")
	present, _ := addrs.snapshot()
	require.Empty(t, present)
	require.Equal(t, int64(0), snap.N2.Attempted)
}
```

- [ ] **GREEN:** Replace `snapshot.go` and `controller.go`, then run `cd tester && go vet ./... && go test -race -count=3 ./... && golangci-lint run ./...`. Expected: all ok, `0 issues.`

```go
// Package run owns the lifecycle of the single active test run: configure
// gNB IPs, bring up N2 for every gNB, hold the associations until Stop,
// then tear everything down. One run at a time (design Q17).
package run

import (
	"time"

	"tester/dataplane"
	"tester/metrics"
	"tester/profile"
)

type State string

const (
	StateIdle        State = "idle"        // no run since the process started
	StateConfiguring State = "configuring" // adding gNB IPs to host interfaces
	StateN2          State = "n2"          // SCTP + NG Setup in progress
	StateRunning     State = "running"     // N2 finished; UEs register / establish PDU sessions, then hold
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
	State       GnbState `json:"state"`
	Attempts    int      `json:"attempts"`
	LatencyMs   float64  `json:"latencyMs"`
	Cause       string   `json:"cause"`
	Registered  int      `json:"registered"`  // UEs of this gNB that completed registration
	Established int      `json:"established"` // UEs of this gNB with a PDU session
}

// UeState is where one UE is in the pipeline.
type UeState string

const (
	UePending      UeState = "pending" // waiting for its gNB or a registration slot
	UeRegistering  UeState = "registering"
	UeRegistered   UeState = "registered" // waiting for a PDU slot
	UeEstablishing UeState = "establishing"
	UeEstablished  UeState = "established"
	UeFailed       UeState = "failed"    // registration or PDU failed for good
	UeSkipped      UeState = "skipped"   // its gNB never came up
	UeCancelled    UeState = "cancelled" // the run was stopped before it finished
)

// UeSummary counts UEs per state; the fields add up to the UE count.
type UeSummary struct {
	Pending      int `json:"pending"`
	Registering  int `json:"registering"`
	Registered   int `json:"registered"`
	Establishing int `json:"establishing"`
	Established  int `json:"established"`
	Failed       int `json:"failed"`
	Skipped      int `json:"skipped"`
	Cancelled    int `json:"cancelled"`
}

func (s *UeSummary) add(st UeState, d int) {
	switch st {
	case UePending:
		s.Pending += d
	case UeRegistering:
		s.Registering += d
	case UeRegistered:
		s.Registered += d
	case UeEstablishing:
		s.Establishing += d
	case UeEstablished:
		s.Established += d
	case UeFailed:
		s.Failed += d
	case UeSkipped:
		s.Skipped += d
	case UeCancelled:
		s.Cancelled += d
	}
}

// UeFailure is one row of the failed-UE list.
type UeFailure struct {
	Supi     string `json:"supi"`
	Gnb      string `json:"gnb"`
	Stage    string `json:"stage"` // "registration" or "pdu"
	Cause    string `json:"cause"`
	Attempts int    `json:"attempts"`
}

// maxFailuresListed bounds the snapshot; UeSummary.Failed has the total.
const maxFailuresListed = 200

// Snapshot is everything the run page shows; it is what GET /api/run and
// every WebSocket frame carry.
type Snapshot struct {
	RunID        string                `json:"runId"`
	ProfileName  string                `json:"profileName"`
	State        State                 `json:"state"`
	Error        string                `json:"error"`
	StartedAt    *time.Time            `json:"startedAt"`
	StoppedAt    *time.Time            `json:"stoppedAt"`
	N2           metrics.StageSnapshot `json:"n2"`
	Registration metrics.StageSnapshot `json:"registration"`
	Pdu          metrics.StageSnapshot `json:"pdu"`
	Gnbs         []GnbStatus           `json:"gnbs"`
	Ues          UeSummary             `json:"ues"`
	FailedUes    []UeFailure           `json:"failedUes"` // first maxFailuresListed failures
	Dataplane    dataplane.Snapshot    `json:"dataplane"`
}
```

```go
package run

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/fnv"
	"net/netip"
	"slices"
	"sync"
	"time"

	loggergoModel "github.com/Alonza0314/logger-go/v2/model"

	"tester/dataplane"
	"tester/gnb"
	"tester/metrics"
	"tester/netcfg"
	"tester/procedure"
	"tester/profile"
	"tester/ue"
)

var (
	ErrRunActive  = errors.New("a run is already active; stop it first")
	ErrNotRunning = errors.New("no active run to stop")
)

// Deps are the controller's side effects, injected so tests can fake them.
type Deps struct {
	Addrs  netcfg.AddrManager
	Dialer gnb.Dialer
	Log    loggergoModel.LoggerInterface
	Now    func() time.Time
	NewID  func() string
	// NewDataplane builds the traffic engine; nil = dataplane.New.
	NewDataplane func(dataplane.Config) Dataplane
}

// Dataplane is the slice of *dataplane.Engine a run uses.
type Dataplane interface {
	Start() error
	AddUE(ue, gnb int, ueIP netip.Addr, ulTeid, dlTeid uint32, upfN3 netip.AddrPort)
	Stop(drain time.Duration)
	Snapshot() dataplane.Snapshot
}

// dataplaneDrain is how long Stop lets in-flight packets arrive before
// closing the sockets, so the final loss figure is not inflated.
const dataplaneDrain = 200 * time.Millisecond

type Controller struct {
	deps Deps

	mu      sync.Mutex
	current *run
	changed chan struct{} // closed and replaced on every state change
}

func NewController(deps Deps) *Controller {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	if deps.NewID == nil {
		deps.NewID = func() string { return deps.Now().UTC().Format("20060102-150405") }
	}
	if deps.NewDataplane == nil {
		deps.NewDataplane = func(c dataplane.Config) Dataplane { return dataplane.New(c) }
	}
	return &Controller{deps: deps, changed: make(chan struct{})}
}

// Changed returns a channel that is closed at the next state change. The
// stream handler waits on it so clients see transitions immediately.
func (c *Controller) Changed() <-chan struct{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.changed
}

func (c *Controller) notify() {
	c.mu.Lock()
	defer c.mu.Unlock()
	close(c.changed)
	c.changed = make(chan struct{})
}

// Validate expands p against this host without touching it, and also
// checks that the named interfaces exist here.
func (c *Controller) Validate(p profile.Profile) (*profile.Plan, error) {
	hostIPs, err := c.deps.Addrs.HostIPv4s()
	if err != nil {
		return nil, err
	}
	ifaces, err := c.deps.Addrs.Interfaces()
	if err != nil {
		return nil, err
	}
	plan, err := profile.Expand(p, hostIPs)

	verr := &profile.ValidationError{}
	if !errors.As(err, &verr) && err != nil {
		return nil, err
	}
	for _, f := range []struct{ field, name string }{
		{"network.n2.interface", p.Network.N2.Interface},
		{"network.n3.interface", p.Network.N3.Interface},
	} {
		field, name := f.field, f.name
		if name != "" && !slices.Contains(ifaces, name) && !hasField(verr, field) {
			verr.Errors = append(verr.Errors, profile.FieldError{Field: field, Message: fmt.Sprintf("no interface named %q on this host", name)})
		}
	}
	if len(verr.Errors) > 0 {
		return nil, verr
	}
	return plan, nil
}

func hasField(verr *profile.ValidationError, field string) bool {
	for _, fe := range verr.Errors {
		if fe.Field == field {
			return true
		}
	}
	return false
}

// Start validates p and launches a run in the background. It returns a
// *profile.ValidationError for bad input and ErrRunActive if a run is
// still going.
func (c *Controller) Start(p profile.Profile) (Snapshot, error) {
	c.mu.Lock()
	if c.current != nil && !c.current.state().Finished() {
		c.mu.Unlock()
		return Snapshot{}, ErrRunActive
	}
	c.mu.Unlock()

	plan, err := c.Validate(p)
	if err != nil {
		return Snapshot{}, err
	}
	r := newRun(c.deps.NewID(), p, plan, c.deps, c.notify)

	c.mu.Lock()
	if c.current != nil && !c.current.state().Finished() {
		c.mu.Unlock()
		return Snapshot{}, ErrRunActive
	}
	c.current = r
	c.mu.Unlock()

	go r.execute()
	c.notify()
	return r.snapshot(), nil
}

// Stop asks the active run to tear down. It returns immediately; watch
// the snapshot for StateStopped.
func (c *Controller) Stop() (Snapshot, error) {
	c.mu.Lock()
	r := c.current
	c.mu.Unlock()
	if r == nil || r.state().Finished() || r.state() == StateStopping {
		return Snapshot{}, ErrNotRunning
	}
	r.requestStop()
	return r.snapshot(), nil
}

func (c *Controller) Snapshot() Snapshot {
	c.mu.Lock()
	r := c.current
	c.mu.Unlock()
	if r == nil {
		empty := metrics.NewStage("", 0).Snapshot()
		return Snapshot{State: StateIdle, Gnbs: []GnbStatus{}, FailedUes: []UeFailure{},
			N2: empty, Registration: empty, Pdu: empty, Dataplane: emptyDataplane()}
	}
	return r.snapshot()
}

// Shutdown stops any active run and waits for teardown, for process exit.
func (c *Controller) Shutdown() {
	c.mu.Lock()
	r := c.current
	c.mu.Unlock()
	if r == nil {
		return
	}
	if !r.state().Finished() {
		r.requestStop()
	}
	<-r.done
}

type run struct {
	id      string
	profile profile.Profile
	plan    *profile.Plan
	deps    Deps
	notify  func()

	n2, reg, pdu *metrics.Stage
	regStage     *procStage
	pduStage     *procStage
	teids        gnb.TeidAllocator

	stop context.CancelFunc
	ctx  context.Context
	done chan struct{}

	mu        sync.Mutex
	st        State
	errMsg    string
	startedAt time.Time
	stoppedAt time.Time
	gnbs      []GnbStatus
	conns     []gnb.Conn
	assocs    []*gnb.Association
	ues       []ueRun
	summary   UeSummary
	failures  []UeFailure
	added     []addedAddr  // in the order they were added
	routes    []addedRoute // routes this run added
	dp        Dataplane    // nil until the IPs are configured
}

type addedRoute struct {
	iface string
	dst   netip.Prefix
	gw    netip.Addr
}

// ueRun is one UE's progress; guarded by run.mu except nas and link,
// which only the UE's current attempt touches.
type ueRun struct {
	spec        profile.UeSpec
	state       UeState
	regAttempts int
	pduAttempts int
	regStart    time.Time
	pduStart    time.Time
	regDone     bool // has a final registration outcome (or was skipped)
	pduDone     bool
	ueIP        string
	pduSetup    *gnb.PduSetup

	nas  *ue.UE
	link *gnb.UeLink
}

type addedAddr struct {
	iface  string
	prefix netip.Prefix
}

func newRun(id string, p profile.Profile, plan *profile.Plan, deps Deps, notify func()) *run {
	ctx, cancel := context.WithCancel(context.Background())
	r := &run{
		id: id, profile: p, plan: plan, deps: deps, notify: notify,
		n2:        metrics.NewStage("n2", len(plan.Gnbs)),
		reg:       metrics.NewStage("registration", len(plan.Ues)),
		pdu:       metrics.NewStage("pdu", len(plan.Ues)),
		ctx:       ctx,
		stop:      cancel,
		done:      make(chan struct{}),
		st:        StateConfiguring,
		startedAt: deps.Now(),
		gnbs:      make([]GnbStatus, len(plan.Gnbs)),
		conns:     make([]gnb.Conn, len(plan.Gnbs)),
		assocs:    make([]*gnb.Association, len(plan.Gnbs)),
		ues:       make([]ueRun, len(plan.Ues)),
		failures:  []UeFailure{},
	}
	for i, spec := range plan.Gnbs {
		r.gnbs[i] = GnbStatus{GnbSpec: spec, State: GnbPending}
	}
	for i, spec := range plan.Ues {
		r.ues[i] = ueRun{spec: spec, state: UePending}
	}
	r.summary.Pending = len(plan.Ues)
	r.regStage = newProcStage(p.Rates.Registration, len(plan.Ues), r.attemptRegistration)
	r.pduStage = newProcStage(p.Rates.Pdu, len(plan.Ues), r.attemptPdu)
	return r
}

func (r *run) state() State {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.st
}

func (r *run) setState(s State) {
	r.mu.Lock()
	r.st = s
	if s == StateStopped || s == StateFailed {
		r.stoppedAt = r.deps.Now()
	}
	r.mu.Unlock()
	r.deps.Log.Infof("run %s: %s", r.id, s)
	r.notify()
}

func (r *run) updateGnb(i int, f func(*GnbStatus)) {
	r.mu.Lock()
	f(&r.gnbs[i])
	r.mu.Unlock()
	r.notify()
}

// setUeState must be called with r.mu held.
func (r *run) setUeState(i int, st UeState) {
	r.summary.add(r.ues[i].state, -1)
	r.summary.add(st, 1)
	r.ues[i].state = st
}

func (r *run) requestStop() { r.stop() }

func (r *run) snapshot() Snapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	snap := Snapshot{
		RunID: r.id, ProfileName: r.profile.Name, State: r.st, Error: r.errMsg,
		N2: r.n2.Snapshot(), Registration: r.reg.Snapshot(), Pdu: r.pdu.Snapshot(),
		Gnbs: append([]GnbStatus(nil), r.gnbs...),
		Ues:  r.summary,
		// copy into a non-nil slice: an empty list must encode as [] (the
		// Run page reads failedUes.length; null blanked it)
		FailedUes: append([]UeFailure{}, r.failures...),
		Dataplane: emptyDataplane(),
	}
	if r.dp != nil {
		snap.Dataplane = r.dp.Snapshot()
	}
	started := r.startedAt
	snap.StartedAt = &started
	if !r.stoppedAt.IsZero() {
		stopped := r.stoppedAt
		snap.StoppedAt = &stopped
	}
	return snap
}

func (r *run) execute() {
	defer close(r.done)

	err := r.configureIPs()
	if err == nil && r.ctx.Err() == nil {
		err = r.startDataplane()
	}
	if err != nil {
		r.mu.Lock()
		r.errMsg = err.Error()
		r.mu.Unlock()
		r.removeIPs()
		r.setState(StateFailed)
		return
	}

	var pipelines sync.WaitGroup
	if r.ctx.Err() == nil { // Stop may arrive while IPs are being added
		pipelines.Add(2)
		go func() { defer pipelines.Done(); r.regStage.run(r.ctx) }()
		go func() { defer pipelines.Done(); r.pduStage.run(r.ctx) }()
		r.setState(StateN2)
		r.runN2()
	}
	if r.ctx.Err() == nil {
		r.setState(StateRunning)
		<-r.ctx.Done()
	}

	r.setState(StateStopping)
	if dp := r.dataplane(); dp != nil {
		dp.Stop(dataplaneDrain) // traffic stops at once (design Q12)
	}
	pipelines.Wait() // in-flight procedures finish or time out (design N4)
	r.skipUnfinished()
	r.closeConns()
	r.removeIPs()
	r.setState(StateStopped)
}

// skipUnfinished closes every stage after Stop: gNBs still waiting for
// N2 and UEs still queued are counted as skipped, so each stage is Done
// and its total time stops growing.
func (r *run) skipUnfinished() {
	r.regStage.drain()
	r.pduStage.drain()
	r.mu.Lock()
	for i := range r.gnbs {
		if s := r.gnbs[i].State; s == GnbPending || s == GnbConnecting {
			r.n2.Skip()
		}
	}
	for i := range r.ues {
		u := &r.ues[i]
		if !u.regDone {
			u.regDone = true
			r.reg.Skip()
		}
		if !u.pduDone {
			u.pduDone = true
			r.pdu.Skip()
		}
		switch u.state {
		case UePending, UeRegistering, UeRegistered, UeEstablishing:
			r.setUeState(i, UeCancelled)
		}
	}
	r.mu.Unlock()
	r.notify()
}

// configureIPs puts everything the run needs on the host: each gNB's N2
// and N3 IP, the N6 sink IP if the host lacks it, and the UE pool route
// via the UPF's N6 (design Q16). removeIPs undoes exactly what was added.
func (r *run) configureIPs() error {
	nw := r.profile.Network
	for _, g := range r.plan.Gnbs {
		for _, a := range []struct {
			iface, ip string
			bits      int
		}{{nw.N2.Interface, g.N2IP, r.plan.N2Prefix}, {nw.N3.Interface, g.N3IP, r.plan.N3Prefix}} {
			if r.ctx.Err() != nil {
				return nil // stopping; execute() skips N2 and removes what was added
			}
			if err := r.addAddr(a.iface, netip.PrefixFrom(netip.MustParseAddr(a.ip), a.bits)); err != nil {
				return fmt.Errorf("configure %s: %w", g.Name, err)
			}
		}
	}
	hostIPs, err := r.deps.Addrs.HostIPv4s()
	if err != nil {
		return err
	}
	sink := netip.MustParseAddr(nw.N6.SinkIP)
	if !slices.Contains(hostIPs, sink) {
		if err := r.addAddr(nw.N6.Interface, netip.PrefixFrom(sink, 32)); err != nil {
			return fmt.Errorf("configure N6 sink: %w", err)
		}
	}
	pool, gw := netip.MustParsePrefix(nw.N6.UePool), netip.MustParseAddr(nw.N6.UpfIP)
	added, err := r.deps.Addrs.EnsureRoute(nw.N6.Interface, pool, gw)
	if err != nil {
		return fmt.Errorf("configure UE pool route: %w", err)
	}
	if added {
		r.mu.Lock()
		r.routes = append(r.routes, addedRoute{iface: nw.N6.Interface, dst: pool, gw: gw})
		r.mu.Unlock()
	}
	return nil
}

func (r *run) addAddr(iface string, prefix netip.Prefix) error {
	if err := r.deps.Addrs.Add(iface, prefix); err != nil {
		return err
	}
	r.mu.Lock()
	r.added = append(r.added, addedAddr{iface: iface, prefix: prefix})
	r.mu.Unlock()
	return nil
}

func (r *run) startDataplane() error {
	t, n6 := r.profile.Traffic, r.profile.Network.N6
	n3 := make([]netip.Addr, len(r.plan.Gnbs))
	for i, g := range r.plan.Gnbs {
		n3[i] = netip.MustParseAddr(g.N3IP)
	}
	dp := r.deps.NewDataplane(dataplane.Config{
		RunID: runIDHash(r.id), UeCount: len(r.plan.Ues), GnbN3IPs: n3,
		SinkIP: netip.MustParseAddr(n6.SinkIP), Port: uint16(t.Port), PacketSize: t.PacketSize,
		UlBps: t.UlMbps * 1e6, DlBps: t.DlMbps * 1e6,
	})
	if err := dp.Start(); err != nil {
		return fmt.Errorf("start data plane: %w", err)
	}
	r.mu.Lock()
	r.dp = dp
	r.mu.Unlock()
	return nil
}

func (r *run) dataplane() Dataplane {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.dp
}

// runIDHash tags this run's packets, so leftovers from an earlier run that
// arrive late are not counted.
func runIDHash(id string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(id))
	return h.Sum32()
}

func emptyDataplane() dataplane.Snapshot {
	return dataplane.Snapshot{Gnbs: []dataplane.GnbTraffic{}, Series: []dataplane.Point{}}
}

// removeIPs removes the routes this run added, then walks the addresses
// backwards so a primary address (added first) goes last; removing it
// first would make the kernel drop the secondaries with it.
func (r *run) removeIPs() {
	r.mu.Lock()
	added, routes := r.added, r.routes
	r.added, r.routes = nil, nil
	r.mu.Unlock()
	var errs []error
	for _, rt := range routes {
		if err := r.deps.Addrs.RemoveRoute(rt.iface, rt.dst, rt.gw); err != nil {
			errs = append(errs, err)
		}
	}
	for i := len(added) - 1; i >= 0; i-- {
		if err := r.deps.Addrs.Remove(added[i].iface, added[i].prefix); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		r.mu.Lock()
		if r.errMsg != "" {
			errs = append([]error{errors.New(r.errMsg)}, errs...)
		}
		r.errMsg = errors.Join(errs...).Error()
		r.mu.Unlock()
	}
}

// maxConcurrentCloses bounds teardown goroutines. Each SCTP Close may
// block up to 1 s (free5gc/sctp sets SO_LINGER=1s), so closing serially
// would outlast `docker stop`'s grace period and leak gNB IPs.
const maxConcurrentCloses = 256

func (r *run) closeConns() {
	sem := make(chan struct{}, maxConcurrentCloses)
	var wg sync.WaitGroup
	for i := range r.conns {
		r.mu.Lock()
		conn := r.conns[i]
		r.conns[i] = nil
		r.mu.Unlock()
		if conn == nil {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer func() { <-sem; wg.Done() }()
			_ = conn.Close()
			r.updateGnb(i, func(g *GnbStatus) { g.State = GnbClosed })
		}()
	}
	wg.Wait()
}

// runN2 brings every gNB up concurrently. A failed attempt with retries
// left goes to the back of the queue (design N3). It returns when every
// gNB has a final outcome, or when Stop is requested (queued gNBs are
// then left pending; in-flight attempts finish within their timeout).
func (r *run) runN2() {
	count := len(r.plan.Gnbs)
	n := &n2Round{
		maxAttempts:  r.profile.Rates.N2.Retries + 1,
		timeout:      time.Duration(r.profile.Rates.N2.TimeoutMs) * time.Millisecond,
		queue:        make(chan int, count),
		firstAttempt: make([]time.Time, count),
		left:         count,
		allDone:      make(chan struct{}),
	}
	for i := range count {
		n.queue <- i
	}

	var workers sync.WaitGroup
	for range count {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for {
				select {
				case <-r.ctx.Done():
					return
				case <-n.allDone:
					return
				case i := <-n.queue:
					if r.ctx.Err() != nil { // select picks randomly among ready cases
						return
					}
					r.attemptN2(n, i)
				}
			}
		}()
	}
	workers.Wait()
}

// n2Round is the shared state of one runN2 call. queue never holds more
// than count items because each gNB is in it at most once.
type n2Round struct {
	maxAttempts  int
	timeout      time.Duration
	queue        chan int
	firstAttempt []time.Time // only touched by the worker holding index i

	mu      sync.Mutex
	left    int
	allDone chan struct{}
}

func (n *n2Round) finished() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.left--
	if n.left == 0 {
		close(n.allDone)
	}
}

func (r *run) attemptN2(n *n2Round, i int) {
	r.mu.Lock()
	r.gnbs[i].Attempts++
	attempt := r.gnbs[i].Attempts
	r.gnbs[i].State = GnbConnecting
	spec := r.gnbs[i].GnbSpec
	r.mu.Unlock()
	if attempt == 1 {
		n.firstAttempt[i] = r.deps.Now()
	}
	r.n2.Begin(attempt > 1)
	r.notify()

	conn, id, err := r.connectGnb(spec, n.timeout)
	latency := r.deps.Now().Sub(n.firstAttempt[i])
	if err == nil {
		r.n2.Finish(metrics.Accepted, latency, "")
		r.updateGnb(i, func(g *GnbStatus) {
			g.State, g.LatencyMs, g.Cause = GnbUp, float64(latency)/float64(time.Millisecond), ""
		})
		r.startGnb(i, conn, id)
		n.finished()
		return
	}

	r.deps.Log.Warnf("run %s: %s attempt %d: %v", r.id, spec.Name, attempt, err)
	if attempt < n.maxAttempts && r.ctx.Err() == nil {
		r.n2.Retrying()
		r.updateGnb(i, func(g *GnbStatus) { g.State, g.Cause = GnbPending, err.Error() })
		n.queue <- i
		return
	}
	r.n2.Finish(Classify(err), latency, CauseOf(err))
	r.updateGnb(i, func(g *GnbStatus) { g.State, g.Cause = GnbFailed, err.Error() })
	r.skipGnbUes(i)
	n.finished()
}

// startGnb wraps an up gNB's association and releases its UEs into the
// registration stage.
func (r *run) startGnb(i int, conn gnb.Conn, id gnb.Identity) {
	n3, _ := netip.ParseAddr(r.plan.Gnbs[i].N3IP)
	assoc := gnb.NewAssociation(conn, id, n3, &r.teids)
	r.mu.Lock()
	r.conns[i] = conn
	r.assocs[i] = assoc
	r.mu.Unlock()
	go r.watchAssociation(i, conn, assoc)
	for u := range r.ues {
		if r.ues[u].spec.Gnb == i {
			r.regStage.enqueue(u)
		}
	}
}

// watchAssociation runs the gNB's NGAP read loop. If it ends while the
// run is not stopping, the AMF side went away: the gNB is marked lost,
// and its UEs' procedures fail through DownlinkLost.
func (r *run) watchAssociation(i int, conn gnb.Conn, assoc *gnb.Association) {
	err := assoc.Run()
	if r.ctx.Err() != nil {
		return // our own Close during Stop
	}
	r.mu.Lock()
	if r.conns[i] == conn {
		r.conns[i] = nil
	}
	r.mu.Unlock()
	_ = conn.Close()
	r.updateGnb(i, func(g *GnbStatus) { g.State, g.Cause = GnbLost, "association lost: "+err.Error() })
}

// skipGnbUes marks every UE of a gNB that never came up.
func (r *run) skipGnbUes(gi int) {
	r.mu.Lock()
	for i := range r.ues {
		u := &r.ues[i]
		if u.spec.Gnb != gi {
			continue
		}
		u.regDone, u.pduDone = true, true
		r.reg.Skip()
		r.pdu.Skip()
		r.setUeState(i, UeSkipped)
	}
	r.mu.Unlock()
	r.notify()
}

func (r *run) recordFailure(i int, stage, cause string, attempts int) {
	if len(r.failures) < maxFailuresListed {
		r.failures = append(r.failures, UeFailure{
			Supi: r.ues[i].spec.Supi, Gnb: r.gnbs[r.ues[i].spec.Gnb].Name,
			Stage: stage, Cause: cause, Attempts: attempts,
		})
	}
}

// attemptRegistration is the registration stage's callback; it returns
// true when the UE should be requeued for another attempt.
func (r *run) attemptRegistration(i int) bool {
	rates := r.profile.Rates.Registration
	r.mu.Lock()
	u := &r.ues[i]
	u.regAttempts++
	attempt := u.regAttempts
	if attempt == 1 {
		u.regStart = r.deps.Now()
	}
	assoc := r.assocs[u.spec.Gnb]
	r.setUeState(i, UeRegistering)
	r.mu.Unlock()
	r.reg.Begin(attempt > 1)
	r.notify()

	var out procedure.Outcome
	if u.nas == nil {
		var err error
		if u.nas, err = ue.New(ue.ConfigFrom(u.spec, r.profile.Gnb, r.profile.Ue)); err != nil {
			out = procedure.Outcome{Result: metrics.Failed, Cause: err.Error()}
		}
	}
	if u.nas != nil {
		u.link, out = procedure.Register(assoc, u.nas, time.Duration(rates.TimeoutMs)*time.Millisecond)
	}
	end := r.deps.Now()
	if !out.DoneAt.IsZero() {
		end = out.DoneAt // registration time stops at Registration Complete
	}
	latency := end.Sub(u.regStart)

	if out.Result == metrics.Accepted {
		r.reg.Finish(metrics.Accepted, latency, "")
		r.mu.Lock()
		u.regDone = true
		r.setUeState(i, UeRegistered)
		r.gnbs[u.spec.Gnb].Registered++
		r.mu.Unlock()
		r.pduStage.enqueue(i)
		r.notify()
		return false
	}
	if attempt <= rates.Retries && r.ctx.Err() == nil {
		r.reg.Retrying()
		r.mu.Lock()
		r.setUeState(i, UePending)
		r.mu.Unlock()
		r.notify()
		return true
	}
	r.reg.Finish(out.Result, latency, out.Cause)
	r.pdu.Skip()
	r.mu.Lock()
	u.regDone, u.pduDone = true, true
	r.setUeState(i, UeFailed)
	r.recordFailure(i, "registration", out.Cause, attempt)
	r.mu.Unlock()
	r.notify()
	return false
}

// attemptPdu is the PDU stage's callback.
func (r *run) attemptPdu(i int) bool {
	rates := r.profile.Rates.Pdu
	r.mu.Lock()
	u := &r.ues[i]
	u.pduAttempts++
	attempt := u.pduAttempts
	if attempt == 1 {
		u.pduStart = r.deps.Now()
	}
	assoc := r.assocs[u.spec.Gnb]
	r.setUeState(i, UeEstablishing)
	r.mu.Unlock()
	r.pdu.Begin(attempt > 1)
	r.notify()

	out := procedure.EstablishPdu(assoc, u.link, u.nas, time.Duration(rates.TimeoutMs)*time.Millisecond)
	latency := r.deps.Now().Sub(u.pduStart)

	if out.Result == metrics.Accepted {
		r.pdu.Finish(metrics.Accepted, latency, "")
		r.mu.Lock()
		u.pduDone = true
		u.ueIP, u.pduSetup = out.UeIP.String(), out.Pdu
		r.setUeState(i, UeEstablished)
		r.gnbs[u.spec.Gnb].Established++
		dp := r.dp
		r.mu.Unlock()
		r.startTraffic(dp, i, out)
		r.notify()
		return false
	}
	if attempt <= rates.Retries && r.ctx.Err() == nil {
		r.pdu.Retrying()
		r.mu.Lock()
		r.setUeState(i, UeRegistered)
		r.mu.Unlock()
		r.notify()
		return true
	}
	r.pdu.Finish(out.Result, latency, out.Cause)
	r.mu.Lock()
	u.pduDone = true
	r.setUeState(i, UeFailed)
	r.recordFailure(i, "pdu", out.Cause, attempt)
	r.mu.Unlock()
	r.notify()
	return false
}

// startTraffic hands an established UE to the data plane (design Q9:
// each UE starts as soon as its PDU session is up).
func (r *run) startTraffic(dp Dataplane, i int, out procedure.Outcome) {
	if dp == nil || out.Pdu == nil || len(out.Pdu.UlTeid) != 4 {
		return
	}
	upf := out.Pdu.UpfIP
	if !upf.IsValid() {
		upf = netip.MustParseAddr(r.profile.Network.N3.UpfIP)
	}
	dp.AddUE(i, r.ues[i].spec.Gnb, out.UeIP, binary.BigEndian.Uint32(out.Pdu.UlTeid), out.Pdu.DlTeid,
		netip.AddrPortFrom(upf, uint16(r.profile.Network.N3.UpfPort)))
}

func (r *run) connectGnb(spec profile.GnbSpec, timeout time.Duration) (gnb.Conn, gnb.Identity, error) {
	id, err := gnb.NewIdentity(spec, r.profile.Gnb)
	if err != nil {
		return nil, id, err
	}
	req, err := id.NGSetupRequest()
	if err != nil {
		return nil, id, fmt.Errorf("encode ng setup request: %w", err)
	}
	n2 := r.profile.Network.N2
	conn, err := r.deps.Dialer.Dial(spec.N2IP, n2.AmfIP, n2.AmfPort, timeout)
	if err != nil {
		return nil, id, err
	}
	if err := gnb.ExchangeNGSetup(conn, req); err != nil {
		_ = conn.Close()
		return nil, id, err
	}
	return conn, id, nil
}
```

- [ ] Commit `feat: start ue traffic when the pdu session is up`.

### Task 7: OpenAPI and client

- [ ] Save the script below as `/tmp/openapi_phase3.py`, then run `python3 /tmp/openapi_phase3.py web/openapi.yaml && make openapi && sudo chown -R "$USER" web/frontend/src/api`. Expected: `api.ts` gains `TesterTraffic`, `TesterDataplaneSnapshot`, `TesterTrafficDirection`, `TesterLatency`, `TesterGnbTraffic` and `TesterTrafficPoint`. `yarn build` fails in `testerDefaults.ts` until Task 8; commit the two together.

```python
import sys
p=sys.argv[1]; s=open(p).read()
def sub(old,new):
    global s; assert s.count(old)==1,(old[:80],s.count(old)); s=s.replace(old,new)
sub('''      required: [name, scale, gnb, ue, network, rates]''','''      required: [name, scale, gnb, ue, traffic, network, rates]''')
sub('''        ue:
          $ref: '#/components/schemas/TesterUeTemplate'
        network:
          type: object
          required: [n2, n3]''','''        ue:
          $ref: '#/components/schemas/TesterUeTemplate'
        traffic:
          $ref: '#/components/schemas/TesterTraffic'
        network:
          type: object
          required: [n2, n3, n6]''')
sub('''                upfIp:
                  type: string
                  example: 10.0.2.1
                upfPort:
                  type: integer
                  example: 2152
        rates:''','''                upfIp:
                  type: string
                  example: 10.0.2.1
                upfPort:
                  type: integer
                  example: 2152
            n6:
              type: object
              description: Data-network side. Uplink is addressed to sinkIp (added to the interface if the host lacks it); downlink is sent from it to the UEs, whose pool is routed via upfIp for the run.
              required: [interface, sinkIp, upfIp, uePool]
              properties:
                interface:
                  type: string
                  example: docker-cn-ran
                sinkIp:
                  type: string
                  example: 10.0.1.1
                upfIp:
                  type: string
                  example: 10.0.1.5
                uePool:
                  type: string
                  example: 10.60.0.0/16
        rates:''')
sub('''    TesterUeTemplate:''','''    TesterTraffic:
      type: object
      description: Fixed-rate traffic of every established UE. packetSize is the inner IP packet length.
      required: [ulMbps, dlMbps, packetSize, port]
      properties:
        ulMbps:
          type: number
          description: Per UE; 0 disables uplink.
          example: 1
        dlMbps:
          type: number
          description: Per UE; 0 disables downlink.
          example: 5
        packetSize:
          type: integer
          example: 1400
        port:
          type: integer
          description: UDP port at the N6 sink and at the UEs.
          example: 9200
    TesterUeTemplate:''')
sub('''      required: [runId, profileName, state, error, n2, registration, pdu, gnbs, ues, failedUes]''','''      required: [runId, profileName, state, error, n2, registration, pdu, gnbs, ues, failedUes, dataplane]''')
sub('''          description: The first 200 UEs that failed; ues.failed has the total.
          items:
            $ref: '#/components/schemas/TesterUeFailure\'''','''          description: The first 200 UEs that failed; ues.failed has the total.
          items:
            $ref: '#/components/schemas/TesterUeFailure'
        dataplane:
          $ref: '#/components/schemas/TesterDataplaneSnapshot'
    TesterDataplaneSnapshot:
      type: object
      required: [activeUes, ul, dl, gnbs, series]
      properties:
        activeUes:
          type: integer
        ul:
          $ref: '#/components/schemas/TesterTrafficDirection'
        dl:
          $ref: '#/components/schemas/TesterTrafficDirection'
        gnbs:
          type: array
          description: Per gNB, same order as gnbs in the run snapshot.
          items:
            $ref: '#/components/schemas/TesterGnbTraffic'
        series:
          type: array
          description: One point per second, the last 300 seconds.
          items:
            $ref: '#/components/schemas/TesterTrafficPoint'
    TesterTrafficDirection:
      type: object
      description: Tx is what the tester sent; Rx is what came back through the UPF. Bytes are inner IP packet bytes; rates are over the last second.
      required: [txPackets, txBytes, rxPackets, rxBytes, txBps, rxBps, txPps, rxPps, lossRate, outOfOrder, sendErrors, latency]
      properties:
        txPackets:
          type: integer
          format: int64
        txBytes:
          type: integer
          format: int64
        rxPackets:
          type: integer
          format: int64
        rxBytes:
          type: integer
          format: int64
        txBps:
          type: number
        rxBps:
          type: number
        txPps:
          type: number
        rxPps:
          type: number
        lossRate:
          type: number
          description: 1 - rx/tx so far.
        outOfOrder:
          type: integer
          format: int64
        sendErrors:
          type: integer
          format: int64
        latency:
          $ref: '#/components/schemas/TesterLatency'
    TesterLatency:
      type: object
      description: One-way latency in milliseconds (send and receive share the host clock).
      required: [count, avgMs, p50Ms, p99Ms, maxMs]
      properties:
        count:
          type: integer
          format: int64
        avgMs:
          type: number
        p50Ms:
          type: number
        p99Ms:
          type: number
        maxMs:
          type: number
    TesterGnbTraffic:
      type: object
      required: [ulTxBytes, ulRxBytes, dlTxBytes, dlRxBytes]
      properties:
        ulTxBytes:
          type: integer
          format: int64
        ulRxBytes:
          type: integer
          format: int64
        dlTxBytes:
          type: integer
          format: int64
        dlRxBytes:
          type: integer
          format: int64
    TesterTrafficPoint:
      type: object
      required: [t, ulTxBps, ulRxBps, dlTxBps, dlRxBps]
      properties:
        t:
          type: number
          description: Seconds since traffic started.
        ulTxBps:
          type: number
        ulRxBps:
          type: number
        dlTxBps:
          type: number
        dlRxBps:
          type: number''')
open(p,'w').write(s)
```

### Task 8: Frontend

- [ ] Save as `/tmp/frontend_phase3.py`, then run `python3 /tmp/frontend_phase3.py web/frontend/src/page/tester && (cd web/frontend && yarn build)`. Expected: the build succeeds.

```python
import sys
root=sys.argv[1]  # web/frontend/src/page/tester
def edit(name, pairs):
    p=root+'/'+name; s=open(p).read()
    for old,new in pairs:
        assert s.count(old)==1,(name,old[:80],s.count(old)); s=s.replace(old,new)
    open(p,'w').write(s)
edit('testerDefaults.ts',[
('''  network: {
    n2:''','''  traffic: { ulMbps: 1, dlMbps: 5, packetSize: 1400, port: 9200 },
  network: {
    n2:'''),
('''    n3: { interface: 'docker-cn-ran', cidr: '10.0.1.0/24', startIp: '10.0.1.100', upfIp: '10.0.1.5', upfPort: 2152 },''','''    n3: { interface: 'docker-cn-ran', cidr: '10.0.1.0/24', startIp: '10.0.1.100', upfIp: '10.0.1.5', upfPort: 2152 },
    // fru-lab's UPF has no separate N6 network: decapsulated uplink leaves it
    // on docker-cn-ran too, so the host's own address there is the sink
    n6: { interface: 'docker-cn-ran', sinkIp: '10.0.1.1', upfIp: '10.0.1.5', uePool: '10.60.0.0/16' },'''),
])
edit('testerFormat.ts',[
('''// A browser WebSocket can't''','''// formatBps renders a bit rate, e.g. 874.8 Mbps.
export function formatBps(bps: number): string {
  if (!bps) return '0 bps'
  const units = ['bps', 'kbps', 'Mbps', 'Gbps']
  let v = bps
  let i = 0
  while (v >= 1000 && i < units.length - 1) {
    v /= 1000
    i += 1
  }
  return `${v.toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}

// formatBytes renders a byte count in decimal units, e.g. 1.2 GB.
export function formatBytes(bytes: number): string {
  const units = ['B', 'kB', 'MB', 'GB', 'TB']
  let v = bytes
  let i = 0
  while (v >= 1000 && i < units.length - 1) {
    v /= 1000
    i += 1
  }
  return `${v.toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}

// formatLoss renders a loss ratio as a percentage, e.g. 0.49 %.
export function formatLoss(rate: number): string {
  return `${(rate * 100).toFixed(rate > 0 && rate < 0.0001 ? 4 : 2)} %`
}

// A browser WebSocket can't'''),
])
edit('TesterSetupPage.tsx',[
('''              <p className={styles.hint}>Checked and planned now; N3 IPs are not configured until the data-plane phase.</p>
            </section>''','''              <p className={styles.hint}>Each gNB gets its own N3 IP: uplink leaves from it and the UPF sends downlink to it.</p>
            </section>

            <section className={styles.card}>
              <h3 className={styles.cardTitle}>N6 · data network side</h3>
              <div className={styles.fieldGrid}>
                <Field label="Local interface" path="network.n6.interface" {...fieldProps} />
                <Field label="Sink IP" path="network.n6.sinkIp" {...fieldProps} />
                <Field label="UPF N6 IP" path="network.n6.upfIp" {...fieldProps} />
                <Field label="UE IP pool" path="network.n6.uePool" {...fieldProps} />
              </div>
              <p className={styles.hint}>Uplink leaves the UPF addressed to the sink IP (added to the interface if missing). Downlink is sent from it to each UE's IP; the tester routes the UE pool via the UPF N6 IP for the run and removes the route afterwards.</p>
            </section>

            <section className={styles.card}>
              <h3 className={styles.cardTitle}>Traffic per UE</h3>
              <div className={styles.fieldGrid}>
                <Field label="Uplink (Mbps)" path="traffic.ulMbps" numeric {...fieldProps} />
                <Field label="Downlink (Mbps)" path="traffic.dlMbps" numeric {...fieldProps} />
                <Field label="Packet size (bytes, inner IP)" path="traffic.packetSize" numeric {...fieldProps} />
                <Field label="UDP port" path="traffic.port" numeric {...fieldProps} />
              </div>
              <p className={styles.hint}>
                Every UE starts sending as soon as its PDU session is up. At full scale: uplink
                {' '}<span className={styles.mono}>{totalRate(profile.traffic.ulMbps, profile.scale.ueCount, profile.traffic.packetSize)}</span>,
                downlink <span className={styles.mono}>{totalRate(profile.traffic.dlMbps, profile.scale.ueCount, profile.traffic.packetSize)}</span>.
                0 turns a direction off.
              </p>
            </section>'''),
('''const PREVIEW_ROWS = 10''','''const PREVIEW_ROWS = 10

// totalRate is the whole run's offered load for one direction.
function totalRate(mbps: number, ues: number, packetSize: number): string {
  if (!mbps || !ues || !packetSize) return 'off'
  const bps = mbps * 1e6 * ues
  return `${formatBps(bps)} · ${Math.round(bps / (packetSize * 8)).toLocaleString()} pps`
}'''),
('''import { DEFAULT_TESTER_PROFILE, normalizeProfile } from './testerDefaults\'''','''import { DEFAULT_TESTER_PROFILE, normalizeProfile } from './testerDefaults'
import { formatBps } from './testerFormat\''''),
])
edit('TesterRunPage.tsx',[
('''              <StageCard title="PDU session" stage={snapshot.pdu} />
            </div>
''','''              <StageCard title="PDU session" stage={snapshot.pdu} />
            </div>

            <DataplaneCard dp={snapshot.dataplane} />
'''),
('''<th>Registered</th><th>PDU sessions</th><th>Last error</th></tr>''','''<th>Registered</th><th>PDU sessions</th><th>UL received</th><th>DL received</th><th>Last error</th></tr>'''),
('''                        <td>{g.established} / {g.ueCount}</td>
                        <td className={styles.causeCell}>{g.cause || '—'}</td>''','''                        <td>{g.established} / {g.ueCount}</td>
                        <td>{gnbTraffic(snapshot.dataplane.gnbs[g.index - 1], 'ul')}</td>
                        <td>{gnbTraffic(snapshot.dataplane.gnbs[g.index - 1], 'dl')}</td>
                        <td className={styles.causeCell}>{g.cause || '—'}</td>'''),
('''const ACTIVE_STATES''','''// gnbTraffic renders one gNB's received bytes and loss for a direction.
function gnbTraffic(t: TesterGnbTraffic | undefined, dir: 'ul' | 'dl'): string {
  if (!t) return '—'
  const tx = dir === 'ul' ? t.ulTxBytes : t.dlTxBytes
  const rx = dir === 'ul' ? t.ulRxBytes : t.dlRxBytes
  if (!tx) return '—'
  return `${formatBytes(rx)} (${formatLoss(rx < tx ? 1 - rx / tx : 0)} lost)`
}

function DirectionStats({ title, d }: { title: string, d: TesterTrafficDirection }) {
  return (
    <div>
      <h4 className={styles.subTitle}>{title}</h4>
      <dl className={styles.kv}>
        <dt>Sent / received</dt><dd>{formatBps(d.txBps)} / {formatBps(d.rxBps)}</dd>
        <dt>Packets per second</dt><dd>{Math.round(d.txPps).toLocaleString()} / {Math.round(d.rxPps).toLocaleString()}</dd>
        <dt>Total sent / received</dt><dd>{formatBytes(d.txBytes)} / {formatBytes(d.rxBytes)}</dd>
        <dt>Loss</dt><dd>{formatLoss(d.lossRate)}</dd>
        <dt>Latency p50 / p99</dt><dd>{formatMs(d.latency.p50Ms)} / {formatMs(d.latency.p99Ms)}</dd>
        <dt>Out of order</dt><dd>{d.outOfOrder}</dd>
        <dt>Send errors</dt><dd>{d.sendErrors}</dd>
      </dl>
    </div>
  )
}

// TrafficChart plots the last seconds of received (solid) and sent
// (dashed) throughput per direction.
function TrafficChart({ series }: { series: TesterTrafficPoint[] }) {
  const W = 600
  const H = 180
  const pad = { l: 64, r: 12, t: 10, b: 22 }
  if (series.length < 2) return <p className={styles.hint}>The chart starts after two seconds of traffic.</p>
  const t0 = series[0].t
  const t1 = series[series.length - 1].t
  const peak = Math.max(1, ...series.flatMap((p) => [p.ulTxBps, p.ulRxBps, p.dlTxBps, p.dlRxBps]))
  const x = (t: number) => pad.l + ((t - t0) / Math.max(1, t1 - t0)) * (W - pad.l - pad.r)
  const y = (v: number) => H - pad.b - (v / peak) * (H - pad.t - pad.b)
  const line = (k: keyof TesterTrafficPoint) => series.map((p) => `${x(p.t).toFixed(1)},${y(p[k]).toFixed(1)}`).join(' ')
  return (
    <div>
      <div className={styles.legend}>
        <span><i className={styles.swatchDl} />Downlink received</span>
        <span><i className={styles.swatchUl} />Uplink received</span>
        <span className={styles.hint}>dashed = sent</span>
      </div>
      <svg viewBox={`0 0 ${W} ${H}`} className={styles.chart} role="img" aria-label="Throughput over time">
        {[0, 0.5, 1].map((f) => (
          <g key={f}>
            <line x1={pad.l} x2={W - pad.r} y1={y(peak * f)} y2={y(peak * f)} className={styles.chartGrid} />
            <text x={pad.l - 6} y={y(peak * f) + 4} textAnchor="end" className={styles.chartAxis}>{formatBps(peak * f)}</text>
          </g>
        ))}
        <text x={pad.l} y={H - 4} className={styles.chartAxis}>{Math.round(t0)} s</text>
        <text x={W - pad.r} y={H - 4} textAnchor="end" className={styles.chartAxis}>{Math.round(t1)} s</text>
        <polyline points={line('dlTxBps')} className={styles.lineDl} strokeDasharray="4 3" />
        <polyline points={line('ulTxBps')} className={styles.lineUl} strokeDasharray="4 3" />
        <polyline points={line('dlRxBps')} className={styles.lineDl} />
        <polyline points={line('ulRxBps')} className={styles.lineUl} />
      </svg>
    </div>
  )
}

function DataplaneCard({ dp }: { dp: TesterDataplaneSnapshot }) {
  return (
    <section className={styles.card}>
      <div className={styles.stageTop}>
        <h3 className={styles.cardTitle}>Data plane</h3>
        <span className={`${styles.pill} ${dp.activeUes ? styles.pillActive : styles.pillMuted}`}>{dp.activeUes} UEs sending</span>
      </div>
      <TrafficChart series={dp.series} />
      <div className={styles.dirRow}>
        <DirectionStats title="Uplink · UE → N3 → UPF → N6" d={dp.ul} />
        <DirectionStats title="Downlink · N6 → UPF → N3 → UE" d={dp.dl} />
      </div>
    </section>
  )
}

const ACTIVE_STATES'''),
('''import type { TesterRunSnapshot, TesterStageSnapshot, TesterUeSummary } from '../../api\'''','''import type {
  TesterDataplaneSnapshot, TesterGnbTraffic, TesterRunSnapshot, TesterStageSnapshot, TesterTrafficDirection,
  TesterTrafficPoint, TesterUeSummary,
} from '../../api\''''),
('''import { buildTesterStreamUrl, formatMs } from './testerFormat\'''','''import { buildTesterStreamUrl, formatBps, formatBytes, formatLoss, formatMs } from './testerFormat\''''),
])
p=root+'/tester.module.css'; s=open(p).read()
old='''.runError {'''
new='''.dirRow { display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: 1.5rem; margin-top: 0.5rem; }
.chart { width: 100%; height: auto; display: block; margin-top: 0.5rem; }
.chartGrid { stroke: #e2e8f0; stroke-width: 1; }
.chartAxis { fill: #64748b; font-size: 10px; }
.lineDl, .lineUl { fill: none; stroke-width: 2; }
.lineDl { stroke: #0f766e; }
.lineUl { stroke: #4338ca; }
.legend { display: flex; flex-wrap: wrap; gap: 1rem; font-size: 0.78rem; color: #475569; align-items: center; }
.legend i { display: inline-block; width: 14px; height: 3px; border-radius: 2px; margin-right: 0.4rem; vertical-align: middle; }
.swatchDl { background: #0f766e; }
.swatchUl { background: #4338ca; }
.runError {'''
assert s.count(old)==1; open(p,'w').write(s.replace(old,new))
```

- [ ] Re-run the `normalizeProfile` scratch assertions. A Phase 2 profile (no `traffic`, no `network.n6`) must normalize to the new defaults.
- [ ] Commit Tasks 7 and 8 together: `feat: data plane settings and live traffic on the tester pages`.

### Task 9: free5GC templates — usage report threshold

- [ ] Save as `/tmp/smf_urr_threshold.py`, then run `python3 /tmp/smf_urr_threshold.py .`. Expected: 4 files updated (basic `smfcfg`, ulcl `smfcfg`, ulcl-2slice `smf1cfg` and `smf2cfg`).

```python
import glob, re, sys
# usage: python3 smf_urr_threshold.py <repo root>
# Raises the SMF's default usage-report threshold in every free5GC template
# that sets one (basic: 1000 bytes, ulcl: 300 bytes).
root = sys.argv[1]
pat = re.compile(r'^  urrThreshold: \d+ # default usage report threshold in bytes\n', re.M)
new = ('  # A threshold of a few hundred bytes made the UPF send a usage report\n'
       '  # per packet or two: at a few hundred Mbps the PFCP control plane\n'
       '  # starves, session modifications time out and downlink never reaches\n'
       '  # the gNB. 10 GB keeps reports rare.\n'
       '  urrThreshold: 10000000000 # default usage report threshold in bytes\n')
changed = 0
for p in sorted(glob.glob(root + '/web/backend/internal/context/templates/*/config/smf*cfg.yaml')):
    s = open(p).read()
    s2, n = pat.subn(new, s)
    if n:
        open(p, 'w').write(s2)
        changed += 1
        print('updated', p)
    else:
        print('no urrThreshold in', p)
assert changed, 'nothing updated'
```

- [ ] Run `make backend`; the Makefile now rebuilds on template changes. Commit `fix: raise the smf usage report threshold in the free5gc templates`.

### Task 10: Guide

- [ ] Update `docs/tester-guide.md`:
  - **Intro:** a run now also sends traffic.
  - **New "Data plane" section:**
    - UL and DL paths;
    - the N6 settings and why the sink is the host's `docker-cn-ran` address for fru-lab's core;
    - the tester adds and removes N3 IPs, the sink IP and the route;
    - fixed per-UE rates;
    - each UE starts as soon as its PDU session is up.
  - **"What the numbers mean":** sent vs received, inner-IP bytes, loss so far, one-way latency, out of order, send errors, and the chart (solid = received, dashed = sent).
  - **Known limitations:** the three from this plan's Review Focus footer. Also note that the core needs `urrThreshold` raised for high rates (done in fru-lab's templates).
- [ ] Commit `docs: tester guide for the data plane`.

### Task 11: Acceptance against the real core, and the artifact record

- [ ] Run against fru-lab's free5GC with 2 gNBs and 4 UEs:
  1. 1/5 Mbps per UE: expect UL loss 0, DL loss only the start-up packets (the count stays fixed), p99 < 1 ms.
  2. 50/250 Mbps per UE: expect UL lossless, DL close to the sender ceiling, and no `retry-out` in the SMF log.
  3. Stop: expect the route and every added IP removed, and the pre-existing `10.0.1.1` kept.
- [ ] Record Phase 3 in section 13 of `docs/superpowers/throughput-tester-design.html` (and republish to the artifact when its org is reachable). Mark the "第 3 期" card 已完成.
