package netcfg

import (
	"fmt"
	"math/rand/v2"
	"net"
	"net/netip"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

// Pinger sends ICMP echo requests; the run package depends on this
// interface so tests can fake it.
type Pinger interface {
	// Ping sends count echo requests from src to dst, waiting up to
	// timeout for each reply, and returns the round-trip time of every
	// reply that came back. err is set only if a request could not be
	// sent at all; missing replies are just missing from the result.
	Ping(src, dst netip.Addr, count int, timeout time.Duration) ([]time.Duration, error)
}

// ICMPPinger is the real Pinger. It opens a raw ICMP socket bound to src,
// so it needs root or CAP_NET_RAW (fru-tester runs as root).
type ICMPPinger struct{}

// pingGap spaces echo requests that were answered quickly.
const pingGap = 200 * time.Millisecond

func (ICMPPinger) Ping(src, dst netip.Addr, count int, timeout time.Duration) ([]time.Duration, error) {
	c, err := icmp.ListenPacket("ip4:icmp", src.String())
	if err != nil {
		return nil, fmt.Errorf("open an ICMP socket on %s: %w", src, err)
	}
	defer func() { _ = c.Close() }()
	id := rand.IntN(0xffff)
	to := &net.IPAddr{IP: dst.AsSlice()}
	buf := make([]byte, 1500)
	var rtts []time.Duration
	for seq := range count {
		if seq > 0 {
			time.Sleep(pingGap)
		}
		msg, _ := (&icmp.Message{Type: ipv4.ICMPTypeEcho, Body: &icmp.Echo{ID: id, Seq: seq, Data: []byte("fru-tester ping test")}}).Marshal(nil)
		sent := time.Now()
		if _, err := c.WriteTo(msg, to); err != nil {
			return rtts, fmt.Errorf("send to %s: %w", dst, err)
		}
		_ = c.SetReadDeadline(sent.Add(timeout))
		for {
			n, from, err := c.ReadFrom(buf)
			if err != nil {
				break // timed out: no reply to this one
			}
			if isReply(buf[:n], from, dst, id, seq) {
				rtts = append(rtts, time.Since(sent))
				break
			}
		}
	}
	return rtts, nil
}

// isReply reports whether b, received from from, is dst's echo reply to
// request id/seq.
func isReply(b []byte, from net.Addr, dst netip.Addr, id, seq int) bool {
	m, err := icmp.ParseMessage(1, b) // 1 = ICMP for IPv4
	if err != nil || m.Type != ipv4.ICMPTypeEchoReply {
		return false
	}
	e, ok := m.Body.(*icmp.Echo)
	a, isIP := from.(*net.IPAddr)
	if !ok || !isIP || e.ID != id || e.Seq != seq {
		return false
	}
	got, _ := netip.AddrFromSlice(a.IP.To4())
	return got == dst
}
