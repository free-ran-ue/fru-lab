// Package hoststat reports the state of the host fru-tester runs on and of
// the fru-tester process itself: what the Status page shows next to the
// CPU allocation.
package hoststat

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"tester/cpuset"
)

// Status is one reading. Byte counts are bytes.
type Status struct {
	// Name is the tester's configured name (testerName); the API fills it.
	Name         string       `json:"name"`
	Hostname     string       `json:"hostname"`
	Version      string       `json:"version"` // VCS revision of the build, "" when unknown
	GoVersion    string       `json:"goVersion"`
	Kernel       string       `json:"kernel"`
	Pid          int          `json:"pid"`
	StartedAt    time.Time    `json:"startedAt"`
	UptimeSec    int64        `json:"uptimeSec"`
	Memory       Memory       `json:"memory"`
	Cpu          Cpu          `json:"cpu"`
	Capabilities Capabilities `json:"capabilities"`
}

// Memory is the host's memory, the process's, and the limit of the cgroup
// fru-tester is in (a container's), if it has one.
type Memory struct {
	HostTotal     uint64 `json:"hostTotal"`
	HostAvailable uint64 `json:"hostAvailable"`
	ProcessRss    uint64 `json:"processRss"`
	GoHeap        uint64 `json:"goHeap"`
	// CgroupLimit is nil when the cgroup sets no memory limit.
	CgroupLimit *uint64 `json:"cgroupLimit"`
	// CgroupUsage is nil when fru-tester's cgroup cannot be read.
	CgroupUsage *uint64 `json:"cgroupUsage"`
}

// Cpu counts the host's CPUs and the ones fru-tester may use, with the
// host's load average.
type Cpu struct {
	Online     int     `json:"online"`
	Allowed    int     `json:"allowed"`
	Load1      float64 `json:"load1"`
	Load5      float64 `json:"load5"`
	Load15     float64 `json:"load15"`
	Goroutines int     `json:"goroutines"`
}

// Capabilities are what a run needs: NET_ADMIN for the gNB IPs and the UE
// route, BPF and IPC_LOCK for the AF_XDP engine, and the sctp module for N2.
type Capabilities struct {
	NetAdmin bool `json:"netAdmin"`
	Bpf      bool `json:"bpf"`
	IpcLock  bool `json:"ipcLock"`
	Sctp     bool `json:"sctp"`
}

// root is where /proc and /sys are read from; tests point it elsewhere.
var root = "/"

var started = time.Now()

// Read takes a reading; what cannot be read is left zero.
func Read() Status {
	host, _ := os.Hostname()
	s := Status{
		Hostname:  host,
		Version:   revision(),
		GoVersion: runtime.Version(),
		Pid:       os.Getpid(),
		StartedAt: started,
		UptimeSec: int64(time.Since(started).Seconds()),
	}
	var u unix.Utsname
	if unix.Uname(&u) == nil {
		s.Kernel = unix.ByteSliceToString(u.Release[:])
	}
	s.Memory = readMemory()
	s.Cpu = readCpu()
	s.Capabilities = readCapabilities()
	return s
}

func revision() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	rev, dirty := "", false
	for _, kv := range info.Settings {
		switch kv.Key {
		case "vcs.revision":
			rev = kv.Value
		case "vcs.modified":
			dirty = kv.Value == "true"
		}
	}
	if len(rev) > 12 {
		rev = rev[:12]
	}
	if rev != "" && dirty {
		rev += "-dirty"
	}
	return rev
}

func read(path string) string {
	b, err := os.ReadFile(filepath.Join(root, path))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// fields returns "Key: value ..." lines of a /proc file as key → first
// value field.
func fields(path string) map[string]string {
	out := map[string]string{}
	f, err := os.Open(filepath.Join(root, path))
	if err != nil {
		return out
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		k, v, ok := strings.Cut(sc.Text(), ":")
		if !ok {
			continue
		}
		if f := strings.Fields(v); len(f) > 0 {
			out[k] = f[0]
		}
	}
	return out
}

func kb(m map[string]string, key string) uint64 {
	n, _ := strconv.ParseUint(m[key], 10, 64)
	return n * 1024
}

func readMemory() Memory {
	info := fields("proc/meminfo")
	status := fields("proc/self/status")
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	m := Memory{
		HostTotal:     kb(info, "MemTotal"),
		HostAvailable: kb(info, "MemAvailable"),
		ProcessRss:    kb(status, "VmRSS"),
		GoHeap:        ms.HeapAlloc,
	}
	m.CgroupLimit, m.CgroupUsage = cgroupMemory()
	return m
}

// unlimited is where cgroup v1 reports "no limit" (a page-rounded
// LONG_MAX); anything this large is no limit.
const unlimited = 1 << 62

// cgroupMemory reads the memory limit and usage of fru-tester's cgroup:
// cgroup v2 under the path /proc/self/cgroup gives (or the namespace root,
// as a container sees it), else cgroup v1's memory controller.
func cgroupMemory() (limit, usage *uint64) {
	parse := func(v string) *uint64 {
		n, err := strconv.ParseUint(v, 10, 64)
		if err != nil || n >= unlimited {
			return nil
		}
		return &n
	}
	var dirs []string
	for line := range strings.SplitSeq(read("proc/self/cgroup"), "\n") {
		if p, ok := strings.CutPrefix(line, "0::"); ok {
			dirs = append(dirs, filepath.Join("sys/fs/cgroup", p))
		}
	}
	dirs = append(dirs, "sys/fs/cgroup")
	for _, d := range dirs {
		cur := read(filepath.Join(d, "memory.current"))
		if cur == "" {
			continue
		}
		return parse(read(filepath.Join(d, "memory.max"))), parse(cur)
	}
	if cur := read("sys/fs/cgroup/memory/memory.usage_in_bytes"); cur != "" {
		return parse(read("sys/fs/cgroup/memory/memory.limit_in_bytes")), parse(cur)
	}
	return nil, nil
}

func readCpu() Cpu {
	c := Cpu{Allowed: len(cpuset.Allowed()), Goroutines: runtime.NumGoroutine()}
	if online, err := cpuset.Parse(read("sys/devices/system/cpu/online")); err == nil {
		c.Online = len(online)
	}
	if f := strings.Fields(read("proc/loadavg")); len(f) >= 3 {
		c.Load1, _ = strconv.ParseFloat(f[0], 64)
		c.Load5, _ = strconv.ParseFloat(f[1], 64)
		c.Load15, _ = strconv.ParseFloat(f[2], 64)
	}
	return c
}

// Capability bits (linux/capability.h). Kernels before 5.8 have no
// CAP_BPF; CAP_SYS_ADMIN covers it there.
const (
	capNetAdmin = 12
	capIpcLock  = 14
	capSysAdmin = 21
	capBpf      = 39
)

func readCapabilities() Capabilities {
	eff, _ := strconv.ParseUint(fields("proc/self/status")["CapEff"], 16, 64)
	has := func(bit uint) bool { return eff&(1<<bit) != 0 }
	_, err := os.Stat(filepath.Join(root, "proc/net/sctp"))
	return Capabilities{
		NetAdmin: has(capNetAdmin),
		Bpf:      has(capBpf) || has(capSysAdmin),
		IpcLock:  has(capIpcLock),
		Sctp:     err == nil,
	}
}
