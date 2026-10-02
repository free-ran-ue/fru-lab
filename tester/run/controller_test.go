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

	loggergo "github.com/Alonza0314/logger-go/v2"
	"github.com/stretchr/testify/require"

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
func (f *fakeAddrs) Interfaces() ([]string, error)    { return []string{"lo", "eth-n2", "eth-n3"}, nil }

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
		Network: profile.Network{
			N2: profile.N2Network{Interface: "eth-n2", Cidr: "10.0.1.0/24", StartIP: "10.0.1.10", AmfIP: "10.0.1.1", AmfPort: 38412},
			N3: profile.N3Network{Interface: "eth-n3", Cidr: "10.0.2.0/24", StartIP: "10.0.2.10", UpfIP: "10.0.2.1", UpfPort: 2152},
		},
		Rates: profile.Rates{N2: profile.StageRate{TimeoutMs: 1000, Retries: 1}},
	}
}

func newTestController(addrs *fakeAddrs, dialer *fakeDialer) *Controller {
	lg := loggergo.NewLogger("", true) // debugMode=true logs to stdout instead of a file
	lg.SetLevel("error")
	return NewController(Deps{Addrs: addrs, Dialer: dialer, Log: lg.WithTags("TEST"), NewID: func() string { return "run-1" }})
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
	require.Len(t, present, 3)

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
		"add eth-n2 10.0.1.10/24", "add eth-n2 10.0.1.11/24", "add eth-n2 10.0.1.12/24",
		"remove eth-n2 10.0.1.12/24", "remove eth-n2 10.0.1.11/24", "remove eth-n2 10.0.1.10/24",
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
	addrs := &fakeAddrs{failAdd: 3}
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
