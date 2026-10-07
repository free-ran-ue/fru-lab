package netcfg

import (
	"net"
	"net/netip"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

// The real pinger over loopback; raw ICMP sockets need root.
func TestICMPPingerOnLoopback(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("run as root: a raw ICMP socket needs CAP_NET_RAW")
	}
	lo := netip.MustParseAddr("127.0.0.1")
	rtts, err := ICMPPinger{}.Ping(lo, lo, 2, time.Second)
	require.NoError(t, err)
	require.Len(t, rtts, 2)
}

// Only the peer's echo reply with our ID and sequence number counts: a raw
// ICMP socket also sees other ICMP traffic for its address.
func TestIsReplyMatchesOnlyItsOwnEcho(t *testing.T) {
	peer := netip.MustParseAddr("10.0.0.1")
	from := &net.IPAddr{IP: peer.AsSlice()}
	msg := func(typ icmp.Type, id, seq int) []byte {
		b, err := (&icmp.Message{Type: typ, Body: &icmp.Echo{ID: id, Seq: seq}}).Marshal(nil)
		require.NoError(t, err)
		return b
	}
	require.True(t, isReply(msg(ipv4.ICMPTypeEchoReply, 7, 2), from, peer, 7, 2))
	require.False(t, isReply(msg(ipv4.ICMPTypeEchoReply, 8, 2), from, peer, 7, 2), "another ping's ID")
	require.False(t, isReply(msg(ipv4.ICMPTypeEchoReply, 7, 1), from, peer, 7, 2), "an earlier request")
	require.False(t, isReply(msg(ipv4.ICMPTypeEcho, 7, 2), from, peer, 7, 2), "a request, not a reply")
	require.False(t, isReply(msg(ipv4.ICMPTypeEchoReply, 7, 2), &net.IPAddr{IP: net.ParseIP("10.0.0.9")}, peer, 7, 2), "another host")
	require.False(t, isReply([]byte{0}, from, peer, 7, 2), "garbage")
}
