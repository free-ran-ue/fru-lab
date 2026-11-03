package hoststat

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// fakeRoot lays files out under a temp dir and points root at it.
func fakeRoot(t *testing.T, files map[string]string) {
	t.Helper()
	dir := t.TempDir()
	for p, body := range files {
		full := filepath.Join(dir, p)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(body), 0o644))
	}
	old := root
	root = dir
	t.Cleanup(func() { root = old })
}

func TestMemoryReadsTheHostTheProcessAndTheCgroupV2Limit(t *testing.T) {
	fakeRoot(t, map[string]string{
		"proc/meminfo":     "MemTotal:       16384 kB\nMemFree:  1 kB\nMemAvailable:    8192 kB\n",
		"proc/self/status": "Name:\tfru-tester\nVmRSS:\t    2048 kB\n",
		"proc/self/cgroup": "0::/kubepods/pod1/c1\n",
		"sys/fs/cgroup/kubepods/pod1/c1/memory.max":     "4294967296\n",
		"sys/fs/cgroup/kubepods/pod1/c1/memory.current": "1073741824\n",
	})
	m := readMemory()
	require.Equal(t, uint64(16384*1024), m.HostTotal)
	require.Equal(t, uint64(8192*1024), m.HostAvailable)
	require.Equal(t, uint64(2048*1024), m.ProcessRss)
	require.NotNil(t, m.CgroupLimit)
	require.Equal(t, uint64(4<<30), *m.CgroupLimit)
	require.Equal(t, uint64(1<<30), *m.CgroupUsage)
}

func TestCgroupWithoutALimitHasNone(t *testing.T) {
	// a container sees its own cgroup at the namespace root
	fakeRoot(t, map[string]string{
		"proc/self/cgroup":             "0::/\n",
		"sys/fs/cgroup/memory.max":     "max\n",
		"sys/fs/cgroup/memory.current": "123\n",
	})
	limit, usage := cgroupMemory()
	require.Nil(t, limit)
	require.Equal(t, uint64(123), *usage)
}

func TestCgroupV1(t *testing.T) {
	fakeRoot(t, map[string]string{
		"sys/fs/cgroup/memory/memory.limit_in_bytes": "9223372036854771712\n",
		"sys/fs/cgroup/memory/memory.usage_in_bytes": "456\n",
	})
	limit, usage := cgroupMemory()
	require.Nil(t, limit, "v1 reports no limit as a huge number")
	require.Equal(t, uint64(456), *usage)
}

func TestCapabilitiesDecodeCapEff(t *testing.T) {
	// NET_ADMIN (12) and IPC_LOCK (14) only
	fakeRoot(t, map[string]string{"proc/self/status": "CapEff:\t0000000000005000\n"})
	c := readCapabilities()
	require.True(t, c.NetAdmin)
	require.True(t, c.IpcLock)
	require.False(t, c.Bpf)
	require.False(t, c.Sctp)

	// root: every capability; the sctp module is loaded
	fakeRoot(t, map[string]string{"proc/self/status": "CapEff:\t000001ffffffffff\n", "proc/net/sctp/assocs": ""})
	c = readCapabilities()
	require.True(t, c.Bpf)
	require.True(t, c.Sctp)
}

func TestCpuReadsTheLoadAverage(t *testing.T) {
	fakeRoot(t, map[string]string{
		"sys/devices/system/cpu/online": "0-7\n",
		"proc/loadavg":                  "1.50 0.75 0.25 2/300 4242\n",
	})
	c := readCpu()
	require.Equal(t, 8, c.Online)
	require.InDelta(t, 1.5, c.Load1, 1e-9)
	require.InDelta(t, 0.25, c.Load15, 1e-9)
	require.Positive(t, c.Goroutines)
}

func TestReadOnThisHost(t *testing.T) {
	s := Read()
	require.NotEmpty(t, s.Hostname)
	require.NotZero(t, s.Memory.HostTotal)
	require.NotZero(t, s.Memory.ProcessRss)
	require.NotZero(t, s.Cpu.Online)
	require.NotEmpty(t, s.Kernel)
}
