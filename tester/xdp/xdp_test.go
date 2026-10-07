package xdp

import (
	"encoding/binary"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

// These tests need root: set FRU_TESTER_NETNS=1 and run with sudo. They
// build a veth pair, frx-host here and frx-peer in namespace frx-ns:
// 10.98.0.1/24 here, 10.98.0.2/24 there.
func vethPair(t *testing.T) (ifindex int) {
	t.Helper()
	if os.Getenv("FRU_TESTER_NETNS") == "" {
		t.Skip("set FRU_TESTER_NETNS=1 and run as root")
	}
	t.Cleanup(func() {
		_ = exec.Command("ip", "link", "del", "frx-host").Run()
		_ = exec.Command("ip", "netns", "del", "frx-ns").Run()
	})
	for _, c := range []string{
		"ip netns add frx-ns",
		"ip link add frx-host type veth peer name frx-peer netns frx-ns",
		"ip addr add 10.98.0.1/24 dev frx-host", "ip link set frx-host up",
		"ip netns exec frx-ns ip addr add 10.98.0.2/24 dev frx-peer",
		"ip netns exec frx-ns ip link set frx-peer up",
		"ip netns exec frx-ns ip link set lo up",
	} {
		f := strings.Fields(c)
		out, err := exec.Command(f[0], f[1:]...).CombinedOutput()
		require.NoError(t, err, "%s: %s", c, out)
	}
	l, err := netlink.LinkByName("frx-host")
	require.NoError(t, err)
	return l.Attrs().Index
}

// inPeer runs f with the calling thread in frx-ns, so sockets f opens
// belong to the peer side.
func inPeer(t *testing.T, f func()) {
	t.Helper()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	orig, err := netns.Get()
	require.NoError(t, err)
	defer func() { _ = orig.Close() }()
	ns, err := netns.GetFromName("frx-ns")
	require.NoError(t, err)
	defer func() { _ = ns.Close() }()
	require.NoError(t, netns.Set(ns))
	defer func() { require.NoError(t, netns.Set(orig)) }()
	f()
}

func TestSteeringSendsOnlyAllowedUDPToTheSocket(t *testing.T) {
	ifindex := vethPair(t)
	st, err := Steer(ifindex)
	require.NoError(t, err, "the verifier accepts the program")
	defer st.Close()
	sock, err := Open(ifindex, 0, true)
	require.NoError(t, err)
	defer sock.Close()
	require.NoError(t, st.Register(0, sock))
	require.NoError(t, st.Allow(netip.MustParseAddr("10.98.0.1"), 4000))

	kernel, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("10.98.0.1"), Port: 4001})
	require.NoError(t, err)
	defer func() { _ = kernel.Close() }()

	var peer *net.UDPConn
	inPeer(t, func() {
		peer, err = net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("10.98.0.2")})
		require.NoError(t, err)
	})
	defer func() { _ = peer.Close() }()
	_, err = peer.WriteToUDP([]byte("to-xdp"), &net.UDPAddr{IP: net.ParseIP("10.98.0.1"), Port: 4000})
	require.NoError(t, err)
	_, err = peer.WriteToUDP([]byte("to-kernel"), &net.UDPAddr{IP: net.ParseIP("10.98.0.1"), Port: 4001})
	require.NoError(t, err)

	var got []string
	deadline := time.Now().Add(2 * time.Second)
	for len(got) == 0 && time.Now().Before(deadline) {
		sock.Receive(func(f []byte) {
			if len(f) >= 42 && binary.BigEndian.Uint16(f[36:38]) == 4000 {
				got = append(got, string(f[42:]))
			}
		})
		if len(got) == 0 {
			sock.Wait(100)
		}
	}
	require.Equal(t, []string{"to-xdp"}, got)

	buf := make([]byte, 64)
	require.NoError(t, kernel.SetReadDeadline(time.Now().Add(2*time.Second)))
	n, err := kernel.Read(buf)
	require.NoError(t, err, "other UDP still reaches the kernel")
	require.Equal(t, "to-kernel", string(buf[:n]))
}

func TestSocketSendsWholeFrames(t *testing.T) {
	ifindex := vethPair(t)
	st, err := Steer(ifindex) // a veth only transmits AF_XDP frames with XDP on
	require.NoError(t, err)
	defer st.Close()
	sock, err := Open(ifindex, 0, true)
	require.NoError(t, err)
	defer sock.Close()
	require.NoError(t, st.Register(0, sock))

	var peer *net.UDPConn
	inPeer(t, func() {
		peer, err = net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("10.98.0.2"), Port: 5000})
		require.NoError(t, err)
	})
	defer func() { _ = peer.Close() }()

	hop, err := Resolve(netip.MustParseAddr("10.98.0.2"), 2*time.Second)
	require.NoError(t, err)
	require.Equal(t, ifindex, hop.Ifindex)

	payload := []byte("from-xdp")
	require.True(t, sock.Queue(func(frame []byte) int {
		return buildUDP(frame, hop, netip.MustParseAddr("10.98.0.1"), netip.MustParseAddr("10.98.0.2"), 6000, 5000, payload)
	}))
	require.Equal(t, 1, sock.Flush())

	buf := make([]byte, 64)
	require.NoError(t, peer.SetReadDeadline(time.Now().Add(2*time.Second)))
	got, from, err := peer.ReadFromUDP(buf)
	require.NoError(t, err)
	require.Equal(t, "from-xdp", string(buf[:got]))
	require.Equal(t, 6000, from.Port)
}

// buildUDP writes an Ethernet/IPv4/UDP frame (UDP checksum 0) and returns
// its length.
func buildUDP(f []byte, hop NextHop, src, dst netip.Addr, sport, dport uint16, payload []byte) int {
	copy(f[0:6], hop.Dst)
	copy(f[6:12], hop.Src)
	binary.BigEndian.PutUint16(f[12:14], 0x0800)
	ip := f[14:34]
	clear(ip)
	ip[0], ip[6], ip[8], ip[9] = 0x45, 0x40, 64, 17
	binary.BigEndian.PutUint16(ip[2:4], uint16(20+8+len(payload)))
	s, d := src.As4(), dst.As4()
	copy(ip[12:16], s[:])
	copy(ip[16:20], d[:])
	var sum uint32
	for i := 0; i < 20; i += 2 {
		sum += uint32(binary.BigEndian.Uint16(ip[i:]))
	}
	for sum > 0xffff {
		sum = sum>>16 + sum&0xffff
	}
	binary.BigEndian.PutUint16(ip[10:12], ^uint16(sum))
	udp := f[34:42]
	binary.BigEndian.PutUint16(udp[0:2], sport)
	binary.BigEndian.PutUint16(udp[2:4], dport)
	binary.BigEndian.PutUint16(udp[4:6], uint16(8+len(payload)))
	udp[6], udp[7] = 0, 0
	copy(f[42:], payload)
	return 42 + len(payload)
}
