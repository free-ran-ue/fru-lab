// Package bench measures how fast this host's data plane can send, with
// no core network: uplink packets go over loopback to a socket that never
// reads them, with 1, 2, 4 … senders up to one per CPU. It answers "how
// many packets per second per core does this machine give the tester",
// and whether adding cores scales.
package bench

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/unix"

	"tester/dataplane"
)

var (
	ErrInvalidSettings = errors.New("invalid bench settings")
	ErrBenchRunning    = errors.New("a bench is already running")
	ErrRunActive       = errors.New("a run is active; a bench would compete with it for the CPUs")
)

// Settings is what the Bench page sets.
type Settings struct {
	PacketSize  int `json:"packetSize"`  // inner IP packet bytes, 64..1400 like a run
	StepSeconds int `json:"stepSeconds"` // measuring time per sender count, 1..30
	// Gso sends with UDP GSO, as runs do; off measures plain sendmmsg, as
	// on a kernel without GSO.
	Gso bool `json:"gso"`
}

func (s Settings) validate() error {
	if s.PacketSize < 64 || s.PacketSize > 1400 {
		return fmt.Errorf("%w: packetSize must be between 64 and 1400", ErrInvalidSettings)
	}
	if s.StepSeconds < 1 || s.StepSeconds > 30 {
		return fmt.Errorf("%w: stepSeconds must be between 1 and 30", ErrInvalidSettings)
	}
	return nil
}

// Step is one sender count's result.
type Step struct {
	Senders    int     `json:"senders"`
	Pps        float64 `json:"pps"`
	Bps        float64 `json:"bps"` // inner IP bits per second, like a run's rates
	SendErrors uint64  `json:"sendErrors"`
	Gso        bool    `json:"gso"` // the senders did use UDP GSO
}

const (
	StateIdle    = "idle"
	StateRunning = "running"
	StateDone    = "done"
	StateFailed  = "failed"
)

// Result is the latest bench, as GET /api/bench returns it.
type Result struct {
	State          string     `json:"state"`
	Error          string     `json:"error"`
	Cpus           int        `json:"cpus"`
	Kernel         string     `json:"kernel"`
	Settings       Settings   `json:"settings"`
	PlannedSenders []int      `json:"plannedSenders"`
	Steps          []Step     `json:"steps"`
	StartedAt      *time.Time `json:"startedAt"`
	FinishedAt     *time.Time `json:"finishedAt"`
}

// Runner runs one bench at a time.
type Runner struct {
	runActive func() bool
	cpus      int
	measure   func(senders int, s Settings) (Step, error)

	mu     sync.Mutex
	result Result
}

// NewRunner refuses to start while runActive reports a run.
func NewRunner(runActive func() bool) *Runner {
	r := &Runner{runActive: runActive, cpus: runtime.NumCPU(),
		measure: func(n int, s Settings) (Step, error) {
			return measureSend(n, s, warmup, time.Duration(s.StepSeconds)*time.Second)
		}}
	r.result = Result{State: StateIdle, Cpus: r.cpus, Kernel: kernelRelease(), PlannedSenders: []int{}, Steps: []Step{}}
	return r
}

// warmup lets senders reach full speed before a step is measured.
const warmup = 500 * time.Millisecond

func (r *Runner) Running() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.result.State == StateRunning
}

func (r *Runner) Result() Result {
	r.mu.Lock()
	defer r.mu.Unlock()
	res := r.result
	res.PlannedSenders = append([]int{}, res.PlannedSenders...)
	res.Steps = append([]Step{}, res.Steps...)
	return res
}

// Start begins a bench in the background and returns at once.
func (r *Runner) Start(s Settings) (Result, error) {
	if err := s.validate(); err != nil {
		return Result{}, err
	}
	r.mu.Lock()
	if r.result.State == StateRunning {
		r.mu.Unlock()
		return Result{}, ErrBenchRunning
	}
	if r.runActive() {
		r.mu.Unlock()
		return Result{}, ErrRunActive
	}
	now := time.Now()
	counts := senderCounts(r.cpus)
	r.result = Result{State: StateRunning, Cpus: r.cpus, Kernel: kernelRelease(), Settings: s,
		PlannedSenders: counts, Steps: []Step{}, StartedAt: &now}
	r.mu.Unlock()
	go r.execute(s, counts)
	return r.Result(), nil
}

func (r *Runner) execute(s Settings, counts []int) {
	for _, n := range counts {
		step, err := r.measure(n, s)
		r.mu.Lock()
		if err != nil {
			r.result.State, r.result.Error = StateFailed, fmt.Sprintf("%d senders: %v", n, err)
			r.finish()
			r.mu.Unlock()
			return
		}
		r.result.Steps = append(r.result.Steps, step)
		r.mu.Unlock()
	}
	r.mu.Lock()
	r.result.State = StateDone
	r.finish()
	r.mu.Unlock()
}

// finish stamps the end time; r.mu must be held.
func (r *Runner) finish() {
	now := time.Now()
	r.result.FinishedAt = &now
}

// senderCounts is 1, 2, 4 … doubling, and always ends at cpus.
func senderCounts(cpus int) []int {
	var out []int
	for n := 1; n < cpus; n *= 2 {
		out = append(out, n)
	}
	return append(out, max(1, cpus))
}

// Loopback addresses the bench sends between; all of 127/8 is local on
// Linux, so nothing has to be configured.
var (
	benchGnb = netip.MustParseAddr("127.0.0.211")
	benchUpf = netip.MustParseAddr("127.0.0.212")
	benchUe  = func(i int) netip.Addr { return netip.AddrFrom4([4]byte{10, 255, byte(i >> 8), byte(i)}) }
)

// measureSend runs senders uplink senders, one UE each, as fast as they
// go, into a socket that never reads (the kernel drops what overflows its
// buffer, which costs less than an ICMP port unreachable per packet), and
// reports the rate over dur after warm.
func measureSend(senders int, s Settings, warm, dur time.Duration) (Step, error) {
	blackhole, err := net.ListenUDP("udp4", net.UDPAddrFromAddrPort(netip.AddrPortFrom(benchUpf, 0)))
	if err != nil {
		return Step{}, fmt.Errorf("bind the bench's discard socket: %w", err)
	}
	defer func() { _ = blackhole.Close() }()
	_ = blackhole.SetReadBuffer(4096)
	sink, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		return Step{}, err
	}
	sinkPort := uint16(sink.LocalAddr().(*net.UDPAddr).Port)
	_ = sink.Close() // only to pick a free port for the engine's sink

	e := dataplane.New(dataplane.Config{
		RunID: 1, UeCount: senders, GnbN3IPs: []netip.Addr{benchGnb},
		SinkIP: netip.MustParseAddr("127.0.0.1"), Port: sinkPort, PacketSize: s.PacketSize,
		UlBps: 400e9, Senders: senders, Receivers: 1, NoOffload: !s.Gso,
	})
	if err := e.Start(); err != nil {
		return Step{}, err
	}
	defer e.Stop(0)
	upf := blackhole.LocalAddr().(*net.UDPAddr).AddrPort()
	for ue := range senders {
		e.AddUE(ue, 0, benchUe(ue), uint32(ue+1), uint32(ue+1), upf)
	}
	time.Sleep(warm)
	before, t0 := e.Snapshot().Ul, time.Now()
	time.Sleep(dur)
	after, t1 := e.Snapshot().Ul, time.Now()
	pps := float64(after.TxPackets-before.TxPackets) / t1.Sub(t0).Seconds()
	return Step{Senders: senders, Pps: pps, Bps: pps * float64(s.PacketSize*8), SendErrors: after.SendErrors - before.SendErrors,
		Gso: e.GSOSenders() == senders}, nil
}

func kernelRelease() string {
	var u unix.Utsname
	if err := unix.Uname(&u); err != nil {
		return ""
	}
	return strings.TrimRight(string(u.Release[:]), "\x00")
}
