package dataplane

import (
	"net"
	"net/netip"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netns"
)

// xdpTestbed puts the tester on veth frd-tester (gNB N3 10.97.0.10, sink
// 10.97.0.2, UE pool routed via 10.97.0.1) and a fake UPF on 10.97.0.1 in
// namespace frd-upf, where the UE pool is local so downlink reaches it.
// It needs root: FRU_TESTER_NETNS=1 and sudo.
func xdpTestbed(t *testing.T, port uint16) *fakeUPF {
	t.Helper()
	if os.Getenv("FRU_TESTER_NETNS") == "" {
		t.Skip("set FRU_TESTER_NETNS=1 and run as root")
	}
	t.Cleanup(func() {
		_ = exec.Command("ip", "link", "del", "frd-tester").Run()
		_ = exec.Command("ip", "netns", "del", "frd-upf").Run()
	})
	for _, c := range []string{
		"ip netns add frd-upf",
		"ip link add frd-tester type veth peer name frd-upf netns frd-upf",
		"ip addr add 10.97.0.10/24 dev frd-tester", "ip addr add 10.97.0.2/24 dev frd-tester",
		"ip link set frd-tester up",
		"ip route add 10.60.0.0/16 via 10.97.0.1 dev frd-tester",
		"ip netns exec frd-upf ip addr add 10.97.0.1/24 dev frd-upf",
		"ip netns exec frd-upf ip link set frd-upf up",
		"ip netns exec frd-upf ip link set lo up",
		"ip netns exec frd-upf ip route add local 10.60.0.0/16 dev lo",
	} {
		f := strings.Fields(c)
		out, err := exec.Command(f[0], f[1:]...).CombinedOutput()
		require.NoError(t, err, "%s: %s", c, out)
	}
	var n3, n6 *net.UDPConn
	func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		orig, err := netns.Get()
		require.NoError(t, err)
		defer func() { _ = orig.Close() }()
		ns, err := netns.GetFromName("frd-upf")
		require.NoError(t, err)
		defer func() { _ = ns.Close() }()
		require.NoError(t, netns.Set(ns))
		defer func() { require.NoError(t, netns.Set(orig)) }()
		n3, err = net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("10.97.0.1"), Port: GtpPort})
		require.NoError(t, err)
		n6, err = net.ListenUDP("udp4", &net.UDPAddr{Port: int(port)}) // every UE IP is local here
		require.NoError(t, err)
	}()
	return runFakeUPF(t, n3, n6, netip.AddrPortFrom(netip.MustParseAddr("10.97.0.2"), port), 0)
}

// Both directions go through AF_XDP sockets and a UPF in another
// namespace, and every packet comes back, counted per gNB as with sockets.
func TestXDPEngineCarriesUplinkAndDownlink(t *testing.T) {
	const port = 9300
	upf := xdpTestbed(t, port)
	gnb := netip.MustParseAddr("10.97.0.10")
	e := NewXDP(Config{RunID: 77, UeCount: 4, GnbN3IPs: []netip.Addr{gnb}, SinkIP: netip.MustParseAddr("10.97.0.2"),
		Port: port, PacketSize: 500, UlBps: 2e6, DlBps: 2e6, N3Interface: "frd-tester", N6Interface: "frd-tester"})
	require.NoError(t, e.Start())
	for ue := range 4 {
		ueIP := netip.AddrFrom4([4]byte{10, 60, 0, byte(ue + 1)})
		upf.route(ue, ueRoute{dlTeid: uint32(100 + ue), gnb: netip.AddrPortFrom(gnb, GtpPort), ueIP: ueIP})
		e.AddUE(ue, 0, ueIP, uint32(0x1000+ue), uint32(100+ue), netip.MustParseAddrPort("10.97.0.1:2152"))
	}
	time.Sleep(1500 * time.Millisecond)
	e.Stop(200 * time.Millisecond)
	s := e.Snapshot()

	require.Contains(t, s.Engine, "af_xdp (frd-tester")
	require.Equal(t, 4, s.ActiveUes)
	for name, d := range map[string]DirSnapshot{"ul": s.Ul, "dl": s.Dl} {
		require.Positive(t, d.TxPackets, name)
		require.Equal(t, d.TxPackets, d.RxPackets, "%s: every packet came back", name)
		require.Zero(t, d.OutOfOrder, name)
		require.Zero(t, d.SendErrors, name)
		require.Positive(t, d.Latency.Count, name)
	}
	require.Equal(t, s.Gnbs[0].UlTxBytes, s.Gnbs[0].UlRxBytes)
	require.Equal(t, s.Gnbs[0].DlTxBytes, s.Gnbs[0].DlRxBytes)

	// the program is gone and the interface is the kernel's again
	out, err := exec.Command("ip", "-d", "link", "show", "frd-tester").CombinedOutput()
	require.NoError(t, err)
	require.NotContains(t, string(out), "prog/xdp")
}

// auto falls back to sockets on a veth, saying why.
func TestAutoUsesSocketsOffANIC(t *testing.T) {
	r := Select(Config{Engine: EngineAuto, UeCount: 1, GnbN3IPs: []netip.Addr{netip.MustParseAddr("127.0.0.11")},
		SinkIP: netip.MustParseAddr("127.0.0.1"), Port: freePort(t, "127.0.0.1"), PacketSize: 500,
		N3Interface: "lo", N6Interface: "lo"})
	require.NoError(t, r.Start())
	defer r.Stop(0)
	require.Equal(t, "socket (auto: lo is a device, not a NIC)", r.Snapshot().Engine)
}
