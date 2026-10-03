package netcfg

import (
	"net/netip"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// feature reads one ethtool feature ("generic-receive-offload",
// "tcp-segmentation-offload") of dev, in netns ns if not empty.
func feature(t *testing.T, ns, dev, name string) string {
	t.Helper()
	args := []string{"ethtool", "-k", dev}
	if ns != "" {
		args = append([]string{"ip", "netns", "exec", ns}, args...)
	}
	out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
	require.NoError(t, err, string(out))
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, name+":") {
			return strings.Fields(line)[1]
		}
	}
	t.Fatalf("%s has no %s", dev, name)
	return ""
}

func run(t *testing.T, cmds ...string) {
	t.Helper()
	for _, c := range cmds {
		f := strings.Fields(c)
		out, err := exec.Command(f[0], f[1:]...).CombinedOutput()
		require.NoError(t, err, "%s: %s", c, out)
	}
}

// A bridge with two containers: only the one holding the UPF's IP gets
// GRO on its host side and TSO off inside, and Undo puts both back.
func TestTuneVethGROChangesOnlyTheUPFsPort(t *testing.T) {
	if os.Getenv("FRU_TESTER_NETNS") == "" {
		t.Skip("set FRU_TESTER_NETNS=1 and run as root to create namespaces, a bridge and veths")
	}
	t.Cleanup(func() {
		for _, c := range []string{"ip link del frt-br0", "ip netns del frt-upf", "ip netns del frt-amf"} {
			_ = exec.Command("sh", "-c", c+" 2>/dev/null").Run()
		}
	})
	run(t,
		"ip netns add frt-upf", "ip netns add frt-amf",
		"ip link add frt-br0 type bridge", "ip link set frt-br0 up",
		"ip link add frt-vupf type veth peer name eth0 netns frt-upf",
		"ip link add frt-vamf type veth peer name eth0 netns frt-amf",
		"ip link set frt-vupf master frt-br0", "ip link set frt-vamf master frt-br0",
		"ip link set frt-vupf up", "ip link set frt-vamf up",
		"ip netns exec frt-upf ip addr add 10.252.0.5/24 dev eth0", "ip netns exec frt-upf ip link set eth0 up",
		"ip netns exec frt-amf ip addr add 10.252.0.3/24 dev eth0", "ip netns exec frt-amf ip link set eth0 up",
		"ethtool -K frt-vupf gro off", "ethtool -K frt-vamf gro off")

	tuning, err := Netlink{}.TuneVethGRO("frt-br0", []netip.Addr{netip.MustParseAddr("10.252.0.5")})
	require.NoError(t, err)
	require.Equal(t, []string{"frt-vupf (GRO on) and eth0 in its container (TSO off)"}, tuning.Links)
	require.Equal(t, "on", feature(t, "", "frt-vupf", "generic-receive-offload"))
	require.Equal(t, "off", feature(t, "frt-upf", "eth0", "tcp-segmentation-offload"))
	require.Equal(t, "off", feature(t, "", "frt-vamf", "generic-receive-offload"), "the AMF's port is left alone")
	require.Equal(t, "on", feature(t, "frt-amf", "eth0", "tcp-segmentation-offload"))

	require.NoError(t, tuning.Undo())
	require.Equal(t, "off", feature(t, "", "frt-vupf", "generic-receive-offload"))
	require.Equal(t, "on", feature(t, "frt-upf", "eth0", "tcp-segmentation-offload"))
}

func TestTuneVethGROLeavesOtherInterfacesAlone(t *testing.T) {
	tuning, err := Netlink{}.TuneVethGRO("lo", []netip.Addr{netip.MustParseAddr("127.0.0.1")})
	require.NoError(t, err)
	require.Empty(t, tuning.Links)
	require.NoError(t, tuning.Undo())
}
