package cpuset

import (
	"bufio"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// CpuLoad is how busy one CPU was over the last interval, in percent: all
// time not idle, the kernel's (interrupts, softirq) included.
type CpuLoad struct {
	Cpu  int     `json:"cpu"`
	Busy float64 `json:"busy"`
}

// Load is every online CPU's load over one interval. It is the whole
// host's, not only fru-tester's: /proc/stat is not per container.
type Load struct {
	IntervalMs int64     `json:"intervalMs"`
	Cpus       []CpuLoad `json:"cpus"`
}

// cpuTimes is one CPU's counters from /proc/stat, in ticks.
type cpuTimes struct{ busy, total uint64 }

// Sampler measures CPU load between reads of /proc/stat. It keeps no
// goroutine: each Read takes a new sample when the last is older than
// minGap and otherwise returns the last result, so several pollers share
// one measurement.
type Sampler struct {
	mu     sync.Mutex
	prev   map[int]cpuTimes
	prevAt time.Time
	last   Load
}

// minGap is the shortest interval measured; staleAfter is how old a
// sample may be before Read starts over with a short fresh interval.
const (
	minGap     = 500 * time.Millisecond
	staleAfter = 5 * time.Second
	firstGap   = 250 * time.Millisecond
)

func NewSampler() *Sampler { return &Sampler{} }

// Read returns the load since the previous sample. With no recent sample
// (the first call, or nobody polled for a while) it measures firstGap.
func (s *Sampler) Read() (Load, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.prev == nil || time.Since(s.prevAt) > staleAfter {
		if err := s.sample(); err != nil {
			return Load{}, err
		}
		time.Sleep(firstGap)
	} else if time.Since(s.prevAt) < minGap {
		return s.last, nil
	}
	prev, prevAt := s.prev, s.prevAt
	if err := s.sample(); err != nil {
		return Load{}, err
	}
	load := Load{IntervalMs: s.prevAt.Sub(prevAt).Milliseconds(), Cpus: []CpuLoad{}}
	for c, now := range s.prev {
		was, ok := prev[c]
		if !ok || now.total <= was.total {
			continue
		}
		busy := float64(now.busy-was.busy) / float64(now.total-was.total) * 100
		load.Cpus = append(load.Cpus, CpuLoad{Cpu: c, Busy: min(100, max(0, busy))})
	}
	slices.SortFunc(load.Cpus, func(a, b CpuLoad) int { return a.Cpu - b.Cpu })
	s.last = load
	return load, nil
}

func (s *Sampler) sample() error {
	t, err := readStat()
	if err != nil {
		return err
	}
	s.prev, s.prevAt = t, time.Now()
	return nil
}

// readStat reads the per-CPU lines of /proc/stat: "cpuN user nice system
// idle iowait irq softirq steal guest guest_nice". Idle and iowait are not
// busy; guest time is already counted in user.
func readStat() (map[int]cpuTimes, error) {
	f, err := os.Open(filepath.Join(root, "proc/stat"))
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	out := map[int]cpuTimes{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 5 || !strings.HasPrefix(fields[0], "cpu") || fields[0] == "cpu" {
			continue
		}
		c, err := strconv.Atoi(fields[0][3:])
		if err != nil {
			continue
		}
		var t cpuTimes
		for i, v := range fields[1:min(len(fields), 9)] {
			n, _ := strconv.ParseUint(v, 10, 64)
			t.total += n
			if i != 3 && i != 4 { // idle, iowait
				t.busy += n
			}
		}
		out[c] = t
	}
	return out, sc.Err()
}
