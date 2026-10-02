package run

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"sync"
	"time"

	loggergoModel "github.com/Alonza0314/logger-go/v2/model"

	"tester/gnb"
	"tester/metrics"
	"tester/netcfg"
	"tester/procedure"
	"tester/profile"
	"tester/ue"
)

var (
	ErrRunActive  = errors.New("a run is already active; stop it first")
	ErrNotRunning = errors.New("no active run to stop")
)

// Deps are the controller's side effects, injected so tests can fake them.
type Deps struct {
	Addrs  netcfg.AddrManager
	Dialer gnb.Dialer
	Log    loggergoModel.LoggerInterface
	Now    func() time.Time
	NewID  func() string
}

type Controller struct {
	deps Deps

	mu      sync.Mutex
	current *run
	changed chan struct{} // closed and replaced on every state change
}

func NewController(deps Deps) *Controller {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	if deps.NewID == nil {
		deps.NewID = func() string { return deps.Now().UTC().Format("20060102-150405") }
	}
	return &Controller{deps: deps, changed: make(chan struct{})}
}

// Changed returns a channel that is closed at the next state change. The
// stream handler waits on it so clients see transitions immediately.
func (c *Controller) Changed() <-chan struct{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.changed
}

func (c *Controller) notify() {
	c.mu.Lock()
	defer c.mu.Unlock()
	close(c.changed)
	c.changed = make(chan struct{})
}

// Validate expands p against this host without touching it, and also
// checks that the named interfaces exist here.
func (c *Controller) Validate(p profile.Profile) (*profile.Plan, error) {
	hostIPs, err := c.deps.Addrs.HostIPv4s()
	if err != nil {
		return nil, err
	}
	ifaces, err := c.deps.Addrs.Interfaces()
	if err != nil {
		return nil, err
	}
	plan, err := profile.Expand(p, hostIPs)

	verr := &profile.ValidationError{}
	if !errors.As(err, &verr) && err != nil {
		return nil, err
	}
	for _, f := range []struct{ field, name string }{
		{"network.n2.interface", p.Network.N2.Interface},
		{"network.n3.interface", p.Network.N3.Interface},
	} {
		field, name := f.field, f.name
		if name != "" && !slices.Contains(ifaces, name) && !hasField(verr, field) {
			verr.Errors = append(verr.Errors, profile.FieldError{Field: field, Message: fmt.Sprintf("no interface named %q on this host", name)})
		}
	}
	if len(verr.Errors) > 0 {
		return nil, verr
	}
	return plan, nil
}

func hasField(verr *profile.ValidationError, field string) bool {
	for _, fe := range verr.Errors {
		if fe.Field == field {
			return true
		}
	}
	return false
}

// Start validates p and launches a run in the background. It returns a
// *profile.ValidationError for bad input and ErrRunActive if a run is
// still going.
func (c *Controller) Start(p profile.Profile) (Snapshot, error) {
	c.mu.Lock()
	if c.current != nil && !c.current.state().Finished() {
		c.mu.Unlock()
		return Snapshot{}, ErrRunActive
	}
	c.mu.Unlock()

	plan, err := c.Validate(p)
	if err != nil {
		return Snapshot{}, err
	}
	r := newRun(c.deps.NewID(), p, plan, c.deps, c.notify)

	c.mu.Lock()
	if c.current != nil && !c.current.state().Finished() {
		c.mu.Unlock()
		return Snapshot{}, ErrRunActive
	}
	c.current = r
	c.mu.Unlock()

	go r.execute()
	c.notify()
	return r.snapshot(), nil
}

// Stop asks the active run to tear down. It returns immediately; watch
// the snapshot for StateStopped.
func (c *Controller) Stop() (Snapshot, error) {
	c.mu.Lock()
	r := c.current
	c.mu.Unlock()
	if r == nil || r.state().Finished() || r.state() == StateStopping {
		return Snapshot{}, ErrNotRunning
	}
	r.requestStop()
	return r.snapshot(), nil
}

func (c *Controller) Snapshot() Snapshot {
	c.mu.Lock()
	r := c.current
	c.mu.Unlock()
	if r == nil {
		empty := metrics.NewStage("", 0).Snapshot()
		return Snapshot{State: StateIdle, Gnbs: []GnbStatus{}, FailedUes: []UeFailure{},
			N2: empty, Registration: empty, Pdu: empty}
	}
	return r.snapshot()
}

// Shutdown stops any active run and waits for teardown, for process exit.
func (c *Controller) Shutdown() {
	c.mu.Lock()
	r := c.current
	c.mu.Unlock()
	if r == nil {
		return
	}
	if !r.state().Finished() {
		r.requestStop()
	}
	<-r.done
}

type run struct {
	id      string
	profile profile.Profile
	plan    *profile.Plan
	deps    Deps
	notify  func()

	n2, reg, pdu *metrics.Stage
	regStage     *procStage
	pduStage     *procStage
	teids        gnb.TeidAllocator

	stop context.CancelFunc
	ctx  context.Context
	done chan struct{}

	mu        sync.Mutex
	st        State
	errMsg    string
	startedAt time.Time
	stoppedAt time.Time
	gnbs      []GnbStatus
	conns     []gnb.Conn
	assocs    []*gnb.Association
	ues       []ueRun
	summary   UeSummary
	failures  []UeFailure
	added     []addedAddr // in the order they were added
}

// ueRun is one UE's progress; guarded by run.mu except nas and link,
// which only the UE's current attempt touches.
type ueRun struct {
	spec        profile.UeSpec
	state       UeState
	regAttempts int
	pduAttempts int
	regStart    time.Time
	pduStart    time.Time
	regDone     bool // has a final registration outcome (or was skipped)
	pduDone     bool
	ueIP        string
	pduSetup    *gnb.PduSetup

	nas  *ue.UE
	link *gnb.UeLink
}

type addedAddr struct {
	iface  string
	prefix netip.Prefix
}

func newRun(id string, p profile.Profile, plan *profile.Plan, deps Deps, notify func()) *run {
	ctx, cancel := context.WithCancel(context.Background())
	r := &run{
		id: id, profile: p, plan: plan, deps: deps, notify: notify,
		n2:        metrics.NewStage("n2", len(plan.Gnbs)),
		reg:       metrics.NewStage("registration", len(plan.Ues)),
		pdu:       metrics.NewStage("pdu", len(plan.Ues)),
		ctx:       ctx,
		stop:      cancel,
		done:      make(chan struct{}),
		st:        StateConfiguring,
		startedAt: deps.Now(),
		gnbs:      make([]GnbStatus, len(plan.Gnbs)),
		conns:     make([]gnb.Conn, len(plan.Gnbs)),
		assocs:    make([]*gnb.Association, len(plan.Gnbs)),
		ues:       make([]ueRun, len(plan.Ues)),
		failures:  []UeFailure{},
	}
	for i, spec := range plan.Gnbs {
		r.gnbs[i] = GnbStatus{GnbSpec: spec, State: GnbPending}
	}
	for i, spec := range plan.Ues {
		r.ues[i] = ueRun{spec: spec, state: UePending}
	}
	r.summary.Pending = len(plan.Ues)
	r.regStage = newProcStage(p.Rates.Registration, len(plan.Ues), r.attemptRegistration)
	r.pduStage = newProcStage(p.Rates.Pdu, len(plan.Ues), r.attemptPdu)
	return r
}

func (r *run) state() State {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.st
}

func (r *run) setState(s State) {
	r.mu.Lock()
	r.st = s
	if s == StateStopped || s == StateFailed {
		r.stoppedAt = r.deps.Now()
	}
	r.mu.Unlock()
	r.deps.Log.Infof("run %s: %s", r.id, s)
	r.notify()
}

func (r *run) updateGnb(i int, f func(*GnbStatus)) {
	r.mu.Lock()
	f(&r.gnbs[i])
	r.mu.Unlock()
	r.notify()
}

// setUeState must be called with r.mu held.
func (r *run) setUeState(i int, st UeState) {
	r.summary.add(r.ues[i].state, -1)
	r.summary.add(st, 1)
	r.ues[i].state = st
}

func (r *run) requestStop() { r.stop() }

func (r *run) snapshot() Snapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	snap := Snapshot{
		RunID: r.id, ProfileName: r.profile.Name, State: r.st, Error: r.errMsg,
		N2: r.n2.Snapshot(), Registration: r.reg.Snapshot(), Pdu: r.pdu.Snapshot(),
		Gnbs:      append([]GnbStatus(nil), r.gnbs...),
		Ues:       r.summary,
		FailedUes: append([]UeFailure(nil), r.failures...),
	}
	started := r.startedAt
	snap.StartedAt = &started
	if !r.stoppedAt.IsZero() {
		stopped := r.stoppedAt
		snap.StoppedAt = &stopped
	}
	return snap
}

func (r *run) execute() {
	defer close(r.done)

	if err := r.configureIPs(); err != nil {
		r.mu.Lock()
		r.errMsg = err.Error()
		r.mu.Unlock()
		r.removeIPs()
		r.setState(StateFailed)
		return
	}

	var pipelines sync.WaitGroup
	if r.ctx.Err() == nil { // Stop may arrive while IPs are being added
		pipelines.Add(2)
		go func() { defer pipelines.Done(); r.regStage.run(r.ctx) }()
		go func() { defer pipelines.Done(); r.pduStage.run(r.ctx) }()
		r.setState(StateN2)
		r.runN2()
	}
	if r.ctx.Err() == nil {
		r.setState(StateRunning)
		<-r.ctx.Done()
	}

	r.setState(StateStopping)
	pipelines.Wait() // in-flight procedures finish or time out (design N4)
	r.skipUnfinished()
	r.closeConns()
	r.removeIPs()
	r.setState(StateStopped)
}

// skipUnfinished closes every stage after Stop: gNBs still waiting for
// N2 and UEs still queued are counted as skipped, so each stage is Done
// and its total time stops growing.
func (r *run) skipUnfinished() {
	r.regStage.drain()
	r.pduStage.drain()
	r.mu.Lock()
	for i := range r.gnbs {
		if s := r.gnbs[i].State; s == GnbPending || s == GnbConnecting {
			r.n2.Skip()
		}
	}
	for i := range r.ues {
		u := &r.ues[i]
		if !u.regDone {
			u.regDone = true
			r.reg.Skip()
		}
		if !u.pduDone {
			u.pduDone = true
			r.pdu.Skip()
		}
		switch u.state {
		case UePending, UeRegistering, UeRegistered, UeEstablishing:
			r.setUeState(i, UeCancelled)
		}
	}
	r.mu.Unlock()
	r.notify()
}

func (r *run) configureIPs() error {
	n2 := r.profile.Network.N2
	for _, g := range r.plan.Gnbs {
		if r.ctx.Err() != nil {
			return nil // stopping; execute() skips N2 and removes what was added
		}
		prefix := netip.PrefixFrom(netip.MustParseAddr(g.N2IP), r.plan.N2Prefix)
		if err := r.deps.Addrs.Add(n2.Interface, prefix); err != nil {
			return fmt.Errorf("configure %s: %w", g.Name, err)
		}
		r.mu.Lock()
		r.added = append(r.added, addedAddr{iface: n2.Interface, prefix: prefix})
		r.mu.Unlock()
	}
	return nil
}

// removeIPs walks backwards so a primary address (added first) goes last;
// removing it first would make the kernel drop the secondaries with it.
func (r *run) removeIPs() {
	r.mu.Lock()
	added := r.added
	r.added = nil
	r.mu.Unlock()
	var errs []error
	for i := len(added) - 1; i >= 0; i-- {
		if err := r.deps.Addrs.Remove(added[i].iface, added[i].prefix); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		r.mu.Lock()
		if r.errMsg != "" {
			errs = append([]error{errors.New(r.errMsg)}, errs...)
		}
		r.errMsg = errors.Join(errs...).Error()
		r.mu.Unlock()
	}
}

// maxConcurrentCloses bounds teardown goroutines. Each SCTP Close may
// block up to 1 s (free5gc/sctp sets SO_LINGER=1s), so closing serially
// would outlast `docker stop`'s grace period and leak gNB IPs.
const maxConcurrentCloses = 256

func (r *run) closeConns() {
	sem := make(chan struct{}, maxConcurrentCloses)
	var wg sync.WaitGroup
	for i := range r.conns {
		r.mu.Lock()
		conn := r.conns[i]
		r.conns[i] = nil
		r.mu.Unlock()
		if conn == nil {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer func() { <-sem; wg.Done() }()
			_ = conn.Close()
			r.updateGnb(i, func(g *GnbStatus) { g.State = GnbClosed })
		}()
	}
	wg.Wait()
}

// runN2 brings every gNB up concurrently. A failed attempt with retries
// left goes to the back of the queue (design N3). It returns when every
// gNB has a final outcome, or when Stop is requested (queued gNBs are
// then left pending; in-flight attempts finish within their timeout).
func (r *run) runN2() {
	count := len(r.plan.Gnbs)
	n := &n2Round{
		maxAttempts:  r.profile.Rates.N2.Retries + 1,
		timeout:      time.Duration(r.profile.Rates.N2.TimeoutMs) * time.Millisecond,
		queue:        make(chan int, count),
		firstAttempt: make([]time.Time, count),
		left:         count,
		allDone:      make(chan struct{}),
	}
	for i := range count {
		n.queue <- i
	}

	var workers sync.WaitGroup
	for range count {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for {
				select {
				case <-r.ctx.Done():
					return
				case <-n.allDone:
					return
				case i := <-n.queue:
					if r.ctx.Err() != nil { // select picks randomly among ready cases
						return
					}
					r.attemptN2(n, i)
				}
			}
		}()
	}
	workers.Wait()
}

// n2Round is the shared state of one runN2 call. queue never holds more
// than count items because each gNB is in it at most once.
type n2Round struct {
	maxAttempts  int
	timeout      time.Duration
	queue        chan int
	firstAttempt []time.Time // only touched by the worker holding index i

	mu      sync.Mutex
	left    int
	allDone chan struct{}
}

func (n *n2Round) finished() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.left--
	if n.left == 0 {
		close(n.allDone)
	}
}

func (r *run) attemptN2(n *n2Round, i int) {
	r.mu.Lock()
	r.gnbs[i].Attempts++
	attempt := r.gnbs[i].Attempts
	r.gnbs[i].State = GnbConnecting
	spec := r.gnbs[i].GnbSpec
	r.mu.Unlock()
	if attempt == 1 {
		n.firstAttempt[i] = r.deps.Now()
	}
	r.n2.Begin(attempt > 1)
	r.notify()

	conn, id, err := r.connectGnb(spec, n.timeout)
	latency := r.deps.Now().Sub(n.firstAttempt[i])
	if err == nil {
		r.n2.Finish(metrics.Accepted, latency, "")
		r.updateGnb(i, func(g *GnbStatus) {
			g.State, g.LatencyMs, g.Cause = GnbUp, float64(latency)/float64(time.Millisecond), ""
		})
		r.startGnb(i, conn, id)
		n.finished()
		return
	}

	r.deps.Log.Warnf("run %s: %s attempt %d: %v", r.id, spec.Name, attempt, err)
	if attempt < n.maxAttempts && r.ctx.Err() == nil {
		r.n2.Retrying()
		r.updateGnb(i, func(g *GnbStatus) { g.State, g.Cause = GnbPending, err.Error() })
		n.queue <- i
		return
	}
	r.n2.Finish(Classify(err), latency, CauseOf(err))
	r.updateGnb(i, func(g *GnbStatus) { g.State, g.Cause = GnbFailed, err.Error() })
	r.skipGnbUes(i)
	n.finished()
}

// startGnb wraps an up gNB's association and releases its UEs into the
// registration stage.
func (r *run) startGnb(i int, conn gnb.Conn, id gnb.Identity) {
	n3, _ := netip.ParseAddr(r.plan.Gnbs[i].N3IP)
	assoc := gnb.NewAssociation(conn, id, n3, &r.teids)
	r.mu.Lock()
	r.conns[i] = conn
	r.assocs[i] = assoc
	r.mu.Unlock()
	go r.watchAssociation(i, conn, assoc)
	for u := range r.ues {
		if r.ues[u].spec.Gnb == i {
			r.regStage.enqueue(u)
		}
	}
}

// watchAssociation runs the gNB's NGAP read loop. If it ends while the
// run is not stopping, the AMF side went away: the gNB is marked lost,
// and its UEs' procedures fail through DownlinkLost.
func (r *run) watchAssociation(i int, conn gnb.Conn, assoc *gnb.Association) {
	err := assoc.Run()
	if r.ctx.Err() != nil {
		return // our own Close during Stop
	}
	r.mu.Lock()
	if r.conns[i] == conn {
		r.conns[i] = nil
	}
	r.mu.Unlock()
	_ = conn.Close()
	r.updateGnb(i, func(g *GnbStatus) { g.State, g.Cause = GnbLost, "association lost: "+err.Error() })
}

// skipGnbUes marks every UE of a gNB that never came up.
func (r *run) skipGnbUes(gi int) {
	r.mu.Lock()
	for i := range r.ues {
		u := &r.ues[i]
		if u.spec.Gnb != gi {
			continue
		}
		u.regDone, u.pduDone = true, true
		r.reg.Skip()
		r.pdu.Skip()
		r.setUeState(i, UeSkipped)
	}
	r.mu.Unlock()
	r.notify()
}

func (r *run) recordFailure(i int, stage, cause string, attempts int) {
	if len(r.failures) < maxFailuresListed {
		r.failures = append(r.failures, UeFailure{
			Supi: r.ues[i].spec.Supi, Gnb: r.gnbs[r.ues[i].spec.Gnb].Name,
			Stage: stage, Cause: cause, Attempts: attempts,
		})
	}
}

// attemptRegistration is the registration stage's callback; it returns
// true when the UE should be requeued for another attempt.
func (r *run) attemptRegistration(i int) bool {
	rates := r.profile.Rates.Registration
	r.mu.Lock()
	u := &r.ues[i]
	u.regAttempts++
	attempt := u.regAttempts
	if attempt == 1 {
		u.regStart = r.deps.Now()
	}
	assoc := r.assocs[u.spec.Gnb]
	r.setUeState(i, UeRegistering)
	r.mu.Unlock()
	r.reg.Begin(attempt > 1)
	r.notify()

	var out procedure.Outcome
	if u.nas == nil {
		var err error
		if u.nas, err = ue.New(ue.ConfigFrom(u.spec, r.profile.Gnb, r.profile.Ue)); err != nil {
			out = procedure.Outcome{Result: metrics.Failed, Cause: err.Error()}
		}
	}
	if u.nas != nil {
		u.link, out = procedure.Register(assoc, u.nas, time.Duration(rates.TimeoutMs)*time.Millisecond)
	}
	latency := r.deps.Now().Sub(u.regStart)

	if out.Result == metrics.Accepted {
		r.reg.Finish(metrics.Accepted, latency, "")
		r.mu.Lock()
		u.regDone = true
		r.setUeState(i, UeRegistered)
		r.gnbs[u.spec.Gnb].Registered++
		r.mu.Unlock()
		r.pduStage.enqueue(i)
		r.notify()
		return false
	}
	if attempt <= rates.Retries && r.ctx.Err() == nil {
		r.reg.Retrying()
		r.mu.Lock()
		r.setUeState(i, UePending)
		r.mu.Unlock()
		r.notify()
		return true
	}
	r.reg.Finish(out.Result, latency, out.Cause)
	r.pdu.Skip()
	r.mu.Lock()
	u.regDone, u.pduDone = true, true
	r.setUeState(i, UeFailed)
	r.recordFailure(i, "registration", out.Cause, attempt)
	r.mu.Unlock()
	r.notify()
	return false
}

// attemptPdu is the PDU stage's callback.
func (r *run) attemptPdu(i int) bool {
	rates := r.profile.Rates.Pdu
	r.mu.Lock()
	u := &r.ues[i]
	u.pduAttempts++
	attempt := u.pduAttempts
	if attempt == 1 {
		u.pduStart = r.deps.Now()
	}
	assoc := r.assocs[u.spec.Gnb]
	r.setUeState(i, UeEstablishing)
	r.mu.Unlock()
	r.pdu.Begin(attempt > 1)
	r.notify()

	out := procedure.EstablishPdu(assoc, u.link, u.nas, time.Duration(rates.TimeoutMs)*time.Millisecond)
	latency := r.deps.Now().Sub(u.pduStart)

	if out.Result == metrics.Accepted {
		r.pdu.Finish(metrics.Accepted, latency, "")
		r.mu.Lock()
		u.pduDone = true
		u.ueIP, u.pduSetup = out.UeIP.String(), out.Pdu
		r.setUeState(i, UeEstablished)
		r.gnbs[u.spec.Gnb].Established++
		r.mu.Unlock()
		r.notify()
		return false
	}
	if attempt <= rates.Retries && r.ctx.Err() == nil {
		r.pdu.Retrying()
		r.mu.Lock()
		r.setUeState(i, UeRegistered)
		r.mu.Unlock()
		r.notify()
		return true
	}
	r.pdu.Finish(out.Result, latency, out.Cause)
	r.mu.Lock()
	u.pduDone = true
	r.setUeState(i, UeFailed)
	r.recordFailure(i, "pdu", out.Cause, attempt)
	r.mu.Unlock()
	r.notify()
	return false
}

func (r *run) connectGnb(spec profile.GnbSpec, timeout time.Duration) (gnb.Conn, gnb.Identity, error) {
	id, err := gnb.NewIdentity(spec, r.profile.Gnb)
	if err != nil {
		return nil, id, err
	}
	req, err := id.NGSetupRequest()
	if err != nil {
		return nil, id, fmt.Errorf("encode ng setup request: %w", err)
	}
	n2 := r.profile.Network.N2
	conn, err := r.deps.Dialer.Dial(spec.N2IP, n2.AmfIP, n2.AmfPort, timeout)
	if err != nil {
		return nil, id, err
	}
	if err := gnb.ExchangeNGSetup(conn, req); err != nil {
		_ = conn.Close()
		return nil, id, err
	}
	return conn, id, nil
}
