# Throughput Tester Phase 4 (Stop, cleanup and history) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stop ends a run cleanly — traffic stops, every registered UE deregisters, every gNB's SCTP association closes, all with stage statistics — an optional maximum run time triggers the same Stop, and fru-lab keeps the last 50 finished runs with CSV/JSON export.

**Architecture:** fru-tester gets a UE-originating Deregistration in `ue` and `procedure`, and a cleanup sequence in `run` (deregistration stage paced like registration, then a timed SCTP-close stage). The run snapshot gains `deregistration`, `n2Release` and `stopReason`. fru-tester exposes the finished run as `GET /api/run/report`; fru-lab's backend polls it, stores each report once in bbolt (newest 50), and serves list, JSON and CSV. The frontend adds the cleanup cards, the new settings and a History page.

**Tech Stack:** Go (gin, bbolt, free5gc/nas v1.3.0, free5gc/ngap v1.2.0), React + Vite, openapi-generator typescript-axios.

**Spec:** docs/superpowers/throughput-tester-design.html (sections 5, 10, 11 Q12/Q13/Q17/N4/N5/N6/N7, 12) and 5g-throughput-tester.md. **User change (2026-10-02): no PDU Session Release.** Cleanup is UE Deregistration → gNB SCTP disconnect; the core releases the PDU sessions as part of deregistration (as free-ran-ue does). This overrides N5.

## Global Constraints

- One run at a time; a new run can start only after cleanup finished (Q17).
- Stop: traffic stops at once; queued UEs are cancelled; in-flight procedures finish or time out; only what was actually established is torn down (Q12, N4).
- Every per-UE cleanup step has rate, max in flight, timeout and retries, like registration (Q13). Retries go to the back of the queue (N3).
- Max run time: minutes, 0 = unlimited (default); when it expires the run stops exactly as if Stop was pressed (N6).
- History: stored in fru-lab's bbolt, newest 50 kept, CSV (time series) and JSON (report) export (N7).
- fru-tester never provisions subscribers (Q7).
- Go: gofmt, `go vet`, `golangci-lint run ./...` clean in `tester/`; `go test -race ./...` green.
- Frontend: `npx tsc --noEmit -p .` and `npm run build` clean; regenerate the client with `make openapi` after editing `web/openapi.yaml`.
- Commit messages: imperative, lower-case type prefix (`feat:`, `fix:`, `docs:`), ending with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## Review Focus

1. Stop pressed while UEs are still registering: in-flight ones finish, then those that registered must deregister too (not only Established UEs) — covered in Task 4 `TestStopDeregistersRegisteredButNotEstablishedUes`.
2. A gNB lost before Stop: its UEs cannot deregister and must be counted skipped, not timed out, and cleanup must not hang — Task 4 `TestUesOfALostGnbAreSkippedAtCleanup`.
3. Stop pressed a second time during cleanup must abort the rest of cleanup but still close SCTP and remove IPs — Task 4 `TestSecondStopAbortsCleanup`.
4. Max run time expiring while N2 is still in progress must stop the run the same way — Task 4 `TestMaxDurationStopsTheRun` (with a 1-minute profile minimum, tested via an injected duration).
5. fru-lab polling the same finished run repeatedly must store it once, and the 51st run must evict the oldest — Task 7 `TestWatcherStoresEachRunOnce`, `TestHistoryKeepsNewest50`.

---

### Task 1: UE deregistration in `ue` and the fake core

**Files:**
- Modify: `tester/ue/ue.go` (new `DeregistrationRequest`, `EventDeregistered`, handle `DeregAcceptUEOrig`)
- Modify: `tester/internal/fakecore/nas.go` (state after PDU accept, deregistration handling, `IgnoreDeregistration`)
- Modify: `tester/internal/fakecore/amf.go` (UE Context Release Command after Deregistration Accept, `Deregistrations()` counter)
- Test: `tester/ue/ue_test.go`

**Interfaces:**
- Produces: `func (u *UE) DeregistrationRequest() ([]byte, error)`; `ue.EventDeregistered`; `fakecore.Behavior.IgnoreDeregistration bool`; `fakecore.Downlink.Release bool`; `func (a *AMF) Deregistrations() int`.

- [ ] **Step 1: Write the failing tests** in `tester/ue/ue_test.go`:

```go
func TestDeregistrationAfterPduSession(t *testing.T) {
	u, err := New(testConfig())
	require.NoError(t, err)
	net := fakecore.NewNasSession(testSubscriber(), fakecore.Behavior{}, netip.MustParseAddr("10.60.0.7"))
	r := register(t, u, net)
	exchange(t, u, net, r.Reply)
	req, err := u.PduSessionRequest()
	require.NoError(t, err)
	exchange(t, u, net, req)

	dereg, err := u.DeregistrationRequest()
	require.NoError(t, err)
	results := exchange(t, u, net, dereg)
	require.Equal(t, []Result{{Event: EventDeregistered}}, results)
}

func TestDeregistrationWithoutPduSession(t *testing.T) {
	u, err := New(testConfig())
	require.NoError(t, err)
	net := fakecore.NewNasSession(testSubscriber(), fakecore.Behavior{}, netip.MustParseAddr("10.60.0.7"))
	r := register(t, u, net)
	exchange(t, u, net, r.Reply)

	dereg, err := u.DeregistrationRequest()
	require.NoError(t, err)
	require.Equal(t, []Result{{Event: EventDeregistered}}, exchange(t, u, net, dereg))
}

func TestDeregistrationBeforeRegistrationIsAnError(t *testing.T) {
	u, err := New(testConfig())
	require.NoError(t, err)
	_, err = u.DeregistrationRequest()
	require.Error(t, err)
}
```

- [ ] **Step 2: Run** `cd tester && go test ./ue/ -run Deregistration` — Expected: FAIL, `u.DeregistrationRequest undefined`.

- [ ] **Step 3: Implement in `ue/ue.go`**. Add `EventDeregistered // Deregistration Accept (UE originating)` at the end of the Event consts. Add:

```go
// DeregistrationRequest returns the protected UE-originating
// Deregistration Request (normal, not switch-off, so the AMF answers with
// Deregistration Accept). The core releases the UE's PDU session as part
// of it. Call after EventRegistered.
func (u *UE) DeregistrationRequest() ([]byte, error) {
	if u.kAmf == nil {
		return nil, errors.New("deregistration requested before registration")
	}
	return message.Marshal(&message.DeregReqUEOrig{
		DeregType:   &ie.DeregType{AccessType: ie.AccessType_3gpp},
		Ngksi:       &ie.NASKeySetId{Tsc: ie.SecCtxTypeNative, Ksi: ie.NASKeyNA},
		MobileId5GS: u.mobileID,
	}, u.secCtx, message.SecHdrTypeIntegrityProtectedAndCiphered)
}
```

In `Handle`, before `default`: `case *message.DeregAcceptUEOrig: return Result{Event: EventDeregistered}, nil`.

- [ ] **Step 4: Implement in `fakecore/nas.go`**. Add `IgnoreDeregistration bool // never answer the deregistration request (for timeouts)` to `Behavior`; add `Release bool // follow with a UE Context Release Command (after Deregistration Accept)` to `Downlink`. In `onPduRequest`, set `s.state = "established"` on accept (keep `"done"` on reject). Change `Handle`:

```go
	case "registered":
		if s.isDeregistration(uplink) {
			return s.onDeregistration()
		}
		return s.onPduRequest(uplink)
	case "established":
		if s.isDeregistration(uplink) {
			return s.onDeregistration()
		}
		return nil, errors.New("unexpected uplink after pdu session")
```

```go
// isDeregistration peeks without consuming the uplink count: Parse
// advances the count, so parse a copy of the context.
func (s *NasSession) isDeregistration(b []byte) bool {
	peek := *s.secCtx
	up := *s.secCtx.UplinkCount
	peek.UplinkCount = &up
	m, err := message.Parse(b, &peek)
	_, ok := m.(*message.DeregReqUEOrig)
	return err == nil && ok
}

func (s *NasSession) onDeregistration() ([]Downlink, error) {
	s.state = "deregistered"
	if s.behavior.IgnoreDeregistration {
		return nil, nil
	}
	acc, err := message.Marshal(&message.DeregAcceptUEOrig{}, s.secCtx, message.SecHdrTypeIntegrityProtectedAndCiphered)
	return []Downlink{{Nas: acc, Release: true}}, err
}
```

If `message.Parse` mutates the count through a pointer other than `UplinkCount` (check `Parse`), the peek must still leave `s.secCtx` usable: Step 5's tests prove it (the PDU request after a non-dereg peek still decodes). When the deregistration path is taken, parse for real once so the count advances: in `onDeregistration` accept a `b []byte` and call `message.Parse(b, s.secCtx)` first.

- [ ] **Step 5: Implement in `fakecore/amf.go`**. Add `deregistered int` beside `released`, and `func (a *AMF) Deregistrations() int` (locked read). In `onNas`, after writing a downlink with `dl.Release`, count it and send the release command (reuse the existing UEContextReleaseCommand encoding by extracting `ueContextReleaseCommand(ue *amfUe) ([]byte, error)` from the reject path):

```go
		if err == nil && dl.Release {
			a.mu.Lock()
			a.deregistered++
			a.mu.Unlock()
			raw, err = ueContextReleaseCommand(ue)
			if err == nil {
				err = a.write(conn, raw, nil)
			}
		}
```

- [ ] **Step 6: Run** `cd tester && go test -race ./ue/ ./internal/... ./procedure/ ./run/` — Expected: PASS (existing tests unchanged).

- [ ] **Step 7: Commit** `feat: ue-originating deregistration in the ue and the fake core`.

### Task 2: `procedure.Deregister`

**Files:**
- Modify: `tester/procedure/procedure.go`
- Test: `tester/procedure/procedure_test.go`

**Interfaces:**
- Consumes: Task 1.
- Produces: `func Deregister(assoc *gnb.Association, link *gnb.UeLink, u *ue.UE, timeout time.Duration) Outcome` — Accepted on Deregistration Accept, with `DoneAt` set when the accept arrived; never detaches `link` (a retry reuses it), the caller detaches when it is done with the UE.

- [ ] **Step 1: Write the failing tests**:

```go
func TestDeregisterAfterPdu(t *testing.T) {
	assoc, amf, _ := setup(t, func(string) fakecore.Behavior { return fakecore.Behavior{} })
	u := newUE(t, "0000000001")
	link, out := Register(assoc, u, time.Second)
	require.Equal(t, metrics.Accepted, out.Result)
	require.Equal(t, metrics.Accepted, EstablishPdu(assoc, link, u, time.Second).Result)

	out = Deregister(assoc, link, u, time.Second)
	require.Equal(t, metrics.Accepted, out.Result, out.Cause)
	require.Equal(t, 1, amf.Deregistrations())
	require.Eventually(t, func() bool { return amf.ReleaseCompletes() == 1 }, time.Second, 5*time.Millisecond,
		"the gNB answers the UE Context Release that follows")
}

func TestDeregisterTimeout(t *testing.T) {
	assoc, _, _ := setup(t, func(string) fakecore.Behavior { return fakecore.Behavior{IgnoreDeregistration: true} })
	u := newUE(t, "0000000001")
	link, _ := Register(assoc, u, time.Second)

	out := Deregister(assoc, link, u, 100*time.Millisecond)
	require.Equal(t, metrics.TimedOut, out.Result)
}

func TestDeregisterWhenTheAssociationIsLost(t *testing.T) {
	assoc, _, amfEnd := setup(t, func(string) fakecore.Behavior { return fakecore.Behavior{IgnoreDeregistration: true} })
	u := newUE(t, "0000000001")
	link, _ := Register(assoc, u, time.Second)
	_ = amfEnd.Close()

	out := Deregister(assoc, link, u, time.Second)
	require.Equal(t, metrics.Failed, out.Result)
}
```

- [ ] **Step 2: Run** `go test ./procedure/ -run Deregister` — Expected: FAIL, `undefined: Deregister`.

- [ ] **Step 3: Implement**:

```go
// releaseWait bounds how long Deregister waits, after the Deregistration
// Accept, for the AMF's UE Context Release Command, so the AMF has
// finished with the UE before cleanup closes the association.
var releaseWait = time.Second

// Deregister runs one UE-originating deregistration attempt. Timing: from
// sending the request to receiving Deregistration Accept (DoneAt). The
// link stays attached so a retry can reuse it; the caller detaches it.
func Deregister(assoc *gnb.Association, link *gnb.UeLink, u *ue.UE, timeout time.Duration) Outcome {
	deadline := time.Now().Add(timeout)
	req, err := u.DeregistrationRequest()
	if err != nil {
		return failed(err)
	}
	if err := assoc.SendUplinkNas(link, req); err != nil {
		return failed(err)
	}
	for {
		d, err := next(assoc, link, deadline)
		if err != nil {
			return failed(err)
		}
		switch d.Kind {
		case gnb.DownlinkLost:
			return failed(fmt.Errorf("association lost: %w", d.Err))
		case gnb.DownlinkReleased:
			return Outcome{Result: metrics.Rejected, Cause: "ue context released without deregistration accept"}
		}
		if d.Nas == nil {
			continue
		}
		r, err := u.Handle(d.Nas)
		if err != nil {
			return failed(err)
		}
		if r.Event == ue.EventDeregistered {
			done := time.Now()
			awaitRelease(assoc, link, done.Add(releaseWait))
			return Outcome{Result: metrics.Accepted, DoneAt: done}
		}
	}
}

func awaitRelease(assoc *gnb.Association, link *gnb.UeLink, deadline time.Time) {
	for {
		d, err := next(assoc, link, deadline)
		if err != nil || d.Kind != gnb.DownlinkNas {
			return
		}
	}
}
```

Update the `Outcome.DoneAt` comment to "registration and deregistration: when the procedure's last message went out or came in"; the run times deregistration to `DoneAt`, not to the end of `awaitRelease`.

- [ ] **Step 4: Run** `go test -race ./procedure/` — Expected: PASS.
- [ ] **Step 5: Commit** `feat: deregister a ue`.

### Task 3: Profile — deregistration pacing and max run time

**Files:**
- Modify: `tester/profile/profile.go`, `tester/profile/expand.go`
- Test: `tester/profile/*_test.go` (the file that tests validation; find `validateProcedureRate` tests)

**Interfaces:**
- Produces: `Rates.Deregistration ProcedureRate` (`json:"deregistration"`); `Traffic.MaxDurationMin int` (`json:"maxDurationMin"`, 0 = unlimited, max 10080 = 7 days).

- [ ] **Step 1: Write failing tests**: a valid profile with `Rates.Deregistration = {50, 200, 5000, 1}` and `MaxDurationMin: 0` expands; `Rates.Deregistration.RatePerSec = 0` gives a field error on `rates.deregistration.ratePerSec`; `MaxDurationMin = -1` and `10081` give an error on `traffic.maxDurationMin`. Update the shared valid-profile helper(s) in profile, run and api tests to set `Deregistration` (otherwise they fail validation).
- [ ] **Step 2: Run** `go test ./profile/` — Expected: FAIL (unknown field / no error).
- [ ] **Step 3: Implement**: add the fields with comments; in expand.go call `validateProcedureRate(verr, "rates.deregistration", p.Rates.Deregistration)` and

```go
	if t.MaxDurationMin < 0 || t.MaxDurationMin > 7*24*60 {
		verr.add("traffic.maxDurationMin", "must be between 0 (no limit) and 10080 (7 days)")
	}
```
- [ ] **Step 4: Run** `go test ./...` in tester — Expected: PASS.
- [ ] **Step 5: Commit** `feat: deregistration pacing and max run time in the profile`.

### Task 4: Run cleanup — deregistration, SCTP close stage, max run time, abort

**Files:**
- Modify: `tester/metrics/stage.go` (`SetExpected`)
- Modify: `tester/run/controller.go`, `tester/run/snapshot.go`
- Test: `tester/run/pipeline_e2e_test.go`, `tester/metrics/stage_test.go`

**Interfaces:**
- Consumes: `procedure.Deregister`, `Rates.Deregistration`, `Traffic.MaxDurationMin`, `fakecore.AMF.Deregistrations()`.
- Produces: `Snapshot.Deregistration`, `Snapshot.N2Release metrics.StageSnapshot` (`json:"deregistration"`, `json:"n2Release"`); `Snapshot.StopReason string` (`json:"stopReason"`: `""`, `"user"`, `"maxDuration"`); `UeDeregistering`, `UeDeregistered` states and `UeSummary.Deregistering/Deregistered`; `UeFailure.Stage` may be `"deregistration"`; `Controller.Stop` during `StateStopping` aborts the rest of cleanup (returns the snapshot, no error); `Deps.MaxDurationUnit time.Duration` (default `time.Minute`, tests use milliseconds).

- [ ] **Step 1: Write failing tests** in `pipeline_e2e_test.go` (set `p.Rates.Deregistration = profile.ProcedureRate{RatePerSec: 1000, MaxInFlight: 100, TimeoutMs: 2000, Retries: 1}` in `e2eProfile`):

```go
func TestStopDeregistersEveryUeThenClosesN2(t *testing.T) {
	amf := newFakeAMF(nil)
	c, addrs := newE2EController(amfDialer{amf: amf})
	_, err := c.Start(e2eProfile())
	require.NoError(t, err)
	waitFor(t, c, "10 UEs established", func(s Snapshot) bool { return s.Ues.Established == 10 })

	snap := stopAndWait(t, c)
	require.Equal(t, int64(10), snap.Deregistration.Accepted)
	require.True(t, snap.Deregistration.Done)
	require.Equal(t, UeSummary{Deregistered: 10}, snap.Ues)
	require.Equal(t, 10, amf.Deregistrations())
	require.Equal(t, int64(3), snap.N2Release.Accepted)
	require.True(t, snap.N2Release.Done)
	require.Equal(t, "user", snap.StopReason)
	present, _ := addrs.snapshot()
	require.Empty(t, present)
}

func TestStopDeregistersRegisteredButNotEstablishedUes(t *testing.T) {
	amf := newFakeAMF(func(string) fakecore.Behavior { return fakecore.Behavior{RejectPdu: 27} })
	c, _ := newE2EController(amfDialer{amf: amf})
	p := e2eProfile()
	p.Rates.Pdu.Retries = 0
	_, err := c.Start(p)
	require.NoError(t, err)
	waitFor(t, c, "every PDU rejected", func(s Snapshot) bool { return s.Pdu.Done })

	snap := stopAndWait(t, c)
	require.Equal(t, int64(10), snap.Deregistration.Accepted, "registered UEs deregister even without a PDU session")
}

func TestDeregistrationTimeoutIsCountedAndListed(t *testing.T) {
	amf := newFakeAMF(func(string) fakecore.Behavior { return fakecore.Behavior{IgnoreDeregistration: true} })
	c, _ := newE2EController(amfDialer{amf: amf})
	p := e2eProfile()
	p.Rates.Deregistration = profile.ProcedureRate{RatePerSec: 1000, MaxInFlight: 100, TimeoutMs: 100, Retries: 0}
	_, err := c.Start(p)
	require.NoError(t, err)
	waitFor(t, c, "10 UEs established", func(s Snapshot) bool { return s.Ues.Established == 10 })

	snap := stopAndWait(t, c)
	require.Equal(t, int64(10), snap.Deregistration.TimedOut)
	require.Equal(t, 10, snap.Ues.Failed)
	require.Equal(t, "deregistration", snap.FailedUes[0].Stage)
}

func TestUesOfALostGnbAreSkippedAtCleanup(t *testing.T) { /* see step 1 note */ }

func TestSecondStopAbortsCleanup(t *testing.T) {
	amf := newFakeAMF(func(string) fakecore.Behavior { return fakecore.Behavior{IgnoreDeregistration: true} })
	c, addrs := newE2EController(amfDialer{amf: amf})
	p := e2eProfile()
	p.Rates.Deregistration = profile.ProcedureRate{RatePerSec: 1000, MaxInFlight: 100, TimeoutMs: 60000, Retries: 0}
	_, err := c.Start(p)
	require.NoError(t, err)
	waitFor(t, c, "10 UEs established", func(s Snapshot) bool { return s.Ues.Established == 10 })
	_, err = c.Stop()
	require.NoError(t, err)
	waitFor(t, c, "deregistering", func(s Snapshot) bool { return s.Deregistration.InFlight > 0 })

	_, err = c.Stop() // abort cleanup
	require.NoError(t, err)
	snap := waitState(t, c, StateStopped)
	require.True(t, snap.Deregistration.Done)
	require.True(t, snap.N2Release.Done)
	present, _ := addrs.snapshot()
	require.Empty(t, present)
}

func TestMaxDurationStopsTheRun(t *testing.T) {
	amf := newFakeAMF(nil)
	c, _ := newE2EController(amfDialer{amf: amf})
	c.deps.MaxDurationUnit = time.Millisecond
	p := e2eProfile()
	p.Traffic.MaxDurationMin = 300 // 300 ms with the test unit
	_, err := c.Start(p)
	require.NoError(t, err)
	snap := waitState(t, c, StateStopped)
	require.Equal(t, "maxDuration", snap.StopReason)
}
```

`TestUesOfALostGnbAreSkippedAtCleanup`: start, wait for 10 established, close the AMF side of gNB-1's pipe (extend `amfDialer` to remember the `*fakecore.PipeEnd` per local IP: field `ends *sync.Map`), wait for `Gnbs[0].State == GnbLost`, stop; expect `Deregistration.Skipped == 4`, `Deregistration.Accepted == 6`, `N2Release.Accepted == 2`, `N2Release.Skipped == 1`, stopped within the test's 5 s wait.

Update `TestRunRegistersAndEstablishesEveryUe` (expects `UeSummary{Established: 10}` after stop → now `UeSummary{Deregistered: 10}`) and `TestStopDuringRegistrationCancelsQueuedUes` as needed for the new final states. Add to `metrics/stage_test.go`: `SetExpected(3)` then three `Skip()` makes `Done`.

- [ ] **Step 2: Run** `go test ./run/ ./metrics/` — Expected: FAIL to compile (`Deregistration`, `N2Release`, `StopReason`, `MaxDurationUnit`, `SetExpected` undefined).

- [ ] **Step 3: Implement**.

`metrics/stage.go`:
```go
// SetExpected sets how many items the stage will see, for stages whose
// size is only known when they start (cleanup).
func (s *Stage) SetExpected(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expected = n
}
```

`snapshot.go`: add states `UeDeregistering UeState = "deregistering"` and `UeDeregistered UeState = "deregistered"` with `UeSummary` fields `Deregistering`, `Deregistered` and their `add` cases; update the `UeFailure.Stage` comment; add `Deregistration`, `N2Release metrics.StageSnapshot` and `StopReason string` to `Snapshot`; add `Deregistration` and `N2Release` to the idle snapshot in `Controller.Snapshot`.

`controller.go`:
- `Deps.MaxDurationUnit time.Duration` (default `time.Minute` in `NewController`).
- `run` gains `dereg, n2Release *metrics.Stage` (created in `newRun` with expected `len(plan.Ues)` and `len(plan.Gnbs)`), `stopReason string`, `cleanupCtx context.Context`, `abortCleanup context.CancelFunc` (from `context.WithCancel(context.Background())`), `ueRun.deregAttempts int`, `ueRun.deregStart time.Time`.
- `requestStop(reason string)`: under `r.mu`, set `stopReason` if empty; then `r.stop()`. Controller.Stop: if finished → `ErrNotRunning`; if `StateStopping` → `r.abortCleanup()` and return the snapshot; else `r.requestStop("user")`. `Shutdown` calls `requestStop("user")` and `abortCleanup()` before waiting (process exit must not wait for a slow core).
- In `execute`, right after `defer close(r.done)`:
```go
	if m := r.profile.Traffic.MaxDurationMin; m > 0 {
		t := time.AfterFunc(time.Duration(m)*r.deps.MaxDurationUnit, func() { r.requestStop("maxDuration") })
		defer t.Stop()
	}
```
- Teardown order in `execute` becomes: `StateStopping` → stop data plane → `pipelines.Wait()` → `skipUnfinished()` → `r.deregisterAll()` → `r.closeConns()` → `removeIPs()` → `StateStopped`. Also call `r.abortCleanup()` at the end of `execute` (release the context).
- `deregisterAll`:
```go
// deregisterAll deregisters every UE that registered (design Q12, N4),
// paced like registration, then returns. UEs that never registered, or
// whose gNB association is gone, are skipped. A second Stop
// (abortCleanup) skips whatever is still queued.
func (r *run) deregisterAll() {
	var todo []int
	r.mu.Lock()
	for i := range r.ues {
		u := &r.ues[i]
		assoc := r.assocs[u.spec.Gnb]
		if u.link == nil || assoc == nil || assoc.Err() != nil || r.cleanupCtx.Err() != nil {
			r.dereg.Skip()
			continue
		}
		todo = append(todo, i)
	}
	r.mu.Unlock()
	if len(todo) == 0 {
		return
	}
	ctx, done := context.WithCancel(r.cleanupCtx)
	defer done()
	left := atomic.Int64{}
	left.Store(int64(len(todo)))
	stage := newProcStage(r.profile.Rates.Deregistration, len(r.ues), func(i int) bool {
		retry := r.attemptDeregistration(i)
		if !retry && left.Add(-1) == 0 {
			done()
		}
		return retry
	})
	for _, i := range todo {
		stage.enqueue(i)
	}
	stage.run(ctx)
	for range stage.drain() { // aborted: still queued
		r.dereg.Skip()
	}
	r.notify()
}
```
`u.link` is non-nil exactly for UEs whose registration was accepted (Register returns nil otherwise); keep it set after a failed PDU. Read it under `r.mu` (it is written by the registration goroutine, which has finished by now because `pipelines.Wait()` returned).
- `attemptDeregistration(i int) bool` mirrors `attemptPdu`: `deregAttempts++`, first attempt sets `deregStart`, state `UeDeregistering`, `r.dereg.Begin(attempt > 1)`, call `procedure.Deregister(assoc, u.link, u.nas, timeout)`; latency to `out.DoneAt` if set. Accepted → `r.dereg.Finish(Accepted, …)`, state `UeDeregistered`, `u.link = nil`. Accepted also detaches the link (`assoc.Detach(u.link)`). Retry (the same link, which still carries the AMF UE NGAP ID) only if `attempt <= rates.Retries && r.cleanupCtx.Err() == nil`. Final failure → `Finish(out.Result, latency, out.Cause)`, state `UeFailed`, `recordFailure(i, "deregistration", …)`, detach.
- `closeConns` becomes the timed `n2Release` stage: for each gNB, if `r.conns[i] == nil` → `r.n2Release.Skip()`; else `Begin(false)`, time `conn.Close()`, `Finish(Accepted or Failed, latency, err.Error())`, gNB state `GnbClosed`.
- `snapshot()` copies `Deregistration: r.dereg.Snapshot(), N2Release: r.n2Release.Snapshot(), StopReason: r.stopReason`.
- `TestStopDuringRegistrationCancelsQueuedUes`: UEs that registered before Stop now deregister; assert on `Deregistration.Done` and that `Ues.Cancelled + Ues.Deregistered + …` add up.

- [ ] **Step 4: Run** `go test -race -count=3 ./run/ ./metrics/ ./procedure/` — Expected: PASS.
- [ ] **Step 5: Run** the whole module: `go test -race ./... && go vet ./... && golangci-lint run ./...` — Expected: PASS, 0 issues.
- [ ] **Step 6: Commit** `feat: deregister ues and time the sctp close when a run stops` and, separately if it grew, `feat: max run time`.

### Task 5: fru-tester report endpoint

**Files:**
- Modify: `tester/run/controller.go` (`Report`), `tester/run/snapshot.go` (`Report` type)
- Modify: `tester/api/api.go` (`GET /api/run/report`, `Controller` interface)
- Test: `tester/api/api_test.go`, `tester/run/pipeline_e2e_test.go`

**Interfaces:**
- Produces: `type Report struct { Profile profile.Profile `json:"profile"`; Snapshot Snapshot `json:"snapshot"` }`; `func (c *Controller) Report() (Report, error)` returning `ErrNoReport` (`errors.New("no finished run")`) unless the current run's state is stopped or failed; `GET /api/run/report` → 200 Report, 404 `{"message": ...}` on `ErrNoReport`.

- [ ] **Step 1: Write failing tests**: e2e — before Start `Report()` is `ErrNoReport`; while running it is `ErrNoReport`; after stop it returns the profile name and a snapshot with `State == StateStopped`. api — with a fake controller returning `ErrNoReport`, `GET /api/run/report` answers 404; with a report, 200 and JSON containing `"profile"` and `"snapshot"`.
- [ ] **Step 2: Run** — Expected: FAIL (undefined).
- [ ] **Step 3: Implement** and add the method to the api `Controller` interface and the api test fake.
- [ ] **Step 4: Run** `go test -race ./...` — Expected: PASS.
- [ ] **Step 5: Commit** `feat: serve the finished run's report`.

### Task 6: OpenAPI, Setup and Run pages

**Files:**
- Modify: `web/openapi.yaml`; regenerate with `make openapi`
- Modify: `web/frontend/src/page/tester/testerDefaults.ts`, `TesterSetupPage.tsx`, `TesterRunPage.tsx`, `tester.module.css` (only if needed)
- Test: `web/frontend` type check and build; manual screenshot against the real core in Task 9

**Interfaces:**
- Consumes: the JSON from Tasks 3–5.
- Produces: `TesterRates.deregistration`, `TesterTraffic.maxDurationMin`, `TesterRunSnapshot.deregistration`, `.n2Release`, `.stopReason`, `TesterUeSummary.deregistering`, `.deregistered`, `TesterUeFailure.stage` enum + `deregistration`, path `/tester/run/report` (proxied like `/tester/run`).

- [ ] **Step 1: openapi.yaml**: add the fields above (required, like their siblings); add `deregistration` to the stage enum; add the report path with schema `TesterRunReport {profile: TesterProfile, snapshot: TesterRunSnapshot}`. Run `make openapi`; Expected: `api.ts` contains `deregistration`, `n2Release`, `maxDurationMin`.
- [ ] **Step 2: Defaults**: `rates.deregistration: { ratePerSec: 50, maxInFlight: 200, timeoutMs: 5000, retries: 1 }`, `traffic.maxDurationMin: 0`. `normalizeProfile` fills both into older saved profiles automatically.
- [ ] **Step 3: Setup page**: in the pacing section add a "Deregistration (on Stop)" block using the same per-stage field rows as registration and PDU (`rates.deregistration.*`); in the traffic card add `Field label="Max run time (min, 0 = no limit)" path="traffic.maxDurationMin" numeric`.
- [ ] **Step 4: Run page**:
  - `STATE_LABELS.stopping = 'Cleaning up: deregistering UEs, closing N2'`.
  - When the state is `stopping` or `stopped`, render a "Cleanup" row with `StageCard title="UE deregistration" stage={snapshot.deregistration}` and `StageCard title="gNB SCTP close" stage={snapshot.n2Release}` above the existing stage cards.
  - Stop button: while `stopping`, label "Skip cleanup" and enabled (calls the same stop API); confirm text "Skip the remaining deregistrations? SCTP is still closed and IPs removed."
  - If `stopReason === 'maxDuration'`, show a muted pill "Stopped at max run time".
  - UE chips: add deregistering and deregistered.
- [ ] **Step 5: Run** `cd web/frontend && npx tsc --noEmit -p . && npm run build` — Expected: clean.
- [ ] **Step 6: Commit** `feat: cleanup cards, deregistration pacing and max run time on the tester pages`.

### Task 7: fru-lab history store and watcher

**Files:**
- Modify: `web/backend/internal/context/db.go`, `dbBbolt.go` (`List`, `Delete`)
- Modify: `web/backend/internal/context/dbContext.go` (`SaveTesterRun`, `ListTesterRuns`, `GetTesterRun`)
- Create: `web/backend/internal/testerHistory.go` (watcher + handlers + CSV)
- Modify: `web/backend/internal/api_tester.go` (routes), `web/backend/internal/backend.go` (start/stop the watcher)
- Test: `web/backend/internal/context/dbBbolt_test.go`, `web/backend/internal/testerHistory_test.go`

**Interfaces:**
- Produces: `DbIf.List(bucket string) ([]KV, error)` (ascending key order; `type KV struct{ Key string; Value []byte }`), `DbIf.Delete(bucket, key string) error`; `(*dbContext).SaveTesterRun(runID string, report []byte, keep int) (saved bool, err error)` (false if already stored; evicts oldest beyond keep); `ListTesterRuns() ([]KV, error)` newest first; `GetTesterRun(runID) ([]byte, error)`; routes `GET /api/tester/history`, `GET /api/tester/history/:runId`, `GET /api/tester/history/:runId/series.csv`.

- [ ] **Step 1: Failing tests** for bbolt `List`/`Delete`; for `SaveTesterRun` storing once and keeping the newest 50 (`TestHistoryKeepsNewest50`: save 51 ids `run-000`…`run-050`, expect `run-000` gone, 50 left); for the watcher (`TestWatcherStoresEachRunOnce`): an `httptest.Server` serving `/api/run/report` with the api token check, returning a report for runId `r1`; call `w.poll(ctx)` three times; expect one stored record. Second case: server answers 404 → nothing stored, no error logged above debug. Handler tests: list returns newest first with summary fields; CSV has header `t,ulTxBps,ulRxBps,dlTxBps,dlRxBps` and one line per series point; unknown id → 404.
- [ ] **Step 2: Run** `cd web/backend && go test ./internal/...` — Expected: FAIL (undefined).
- [ ] **Step 3: Implement**.
  - The watcher polls every 3 s while fru-lab runs: `GET {tester.url}/api/run/report` with `Authorization: Bearer {apiToken}`; on 200, decode just `{snapshot: {runId}}`, then `SaveTesterRun(runId, body, 50)`; log Info once per newly stored run.
  - `historySummary` (list item) is decoded from the stored report: `runId, profileName, state, error, stopReason, startedAt, stoppedAt, gnbCount (n2.expected), ueCount (registration.expected), registration {accepted, p95Ms}, pdu {accepted}, deregistration {accepted}, ul/dl {txBytes, rxBytes, lossRate}`.
  - `GET /history/:runId` returns the stored JSON with `Content-Disposition: attachment; filename="tester-<runId>.json"`; CSV likewise `tester-<runId>.csv`.
  - Bucket name `tester-history`. Keys are runIds (`20261002-150405`, sortable by time).
- [ ] **Step 4: Run** `go test -race ./...` in `web/backend` — Expected: PASS.
- [ ] **Step 5: Commit** `feat: keep the last 50 tester runs in fru-lab`.

### Task 8: History page

**Files:**
- Modify: `web/openapi.yaml` (history paths and `TesterHistorySummary`), `make openapi`
- Create: `web/frontend/src/page/tester/TesterHistoryPage.tsx`
- Modify: `web/frontend/src/App.tsx` (route `/tester/history`), `web/frontend/src/components/sidebar/Sidebar.tsx` (link "History")

- [ ] **Step 1**: openapi: `GET /tester/history` → `TesterHistorySummary[]`; `GET /tester/history/{runId}` → `TesterRunReport`; `GET /tester/history/{runId}/series.csv` → `text/csv` string. Run `make openapi`.
- [ ] **Step 2**: page: table of runs (time, profile, state/stop reason, registration success %, registration p95, PDU success %, deregistration success %, DL received, UL received, loss DL/UL), a checkbox per row; with exactly two selected, a side-by-side comparison card of those numbers; per-row "JSON" and "CSV" buttons that download through the authenticated client (`responseType: 'blob'`, then an object URL). Empty state: "No finished runs yet. Runs are saved here when they stop."
- [ ] **Step 3: Run** `npx tsc --noEmit -p . && npm run build` — Expected: clean.
- [ ] **Step 4: Commit** `feat: tester run history page with csv and json export`.

### Task 9: Docs, real-core acceptance, phase record

**Files:**
- Modify: `docs/tester-guide.md`, `docs/superpowers/throughput-tester-design.html`

- [ ] **Step 1: Guide**: Stop now deregisters every registered UE, then closes SCTP; second Stop skips the rest; max run time; History page; remove the "Stop does not release…" and "Back-to-back runs… duplicate PDU sessions" limitations if acceptance confirms they are gone.
- [ ] **Step 2: Real-core acceptance** (fru-lab free5GC, restart NFs NRF first and SMF after UPF): profile basic, 2 gNB, 4 UE, 1/1 Mbps, run 30 s, Stop → expect 4/4 deregistered, 2/2 SCTP closed, AMF log has no errors for these SUPIs, no route or gNB IP left. Then immediately run the same profile again → 4/4 registration and PDU (no `Duplicated PDU session ID`). Then a run with `maxDurationMin: 1` stops itself after ~60 s. Check fru-lab History lists all three and both exports download.
- [ ] **Step 3: Design record**: section 13 phase 4 entry (delivered, verification, deviations incl. "no PDU Release, per user", review fixes, leftovers); section 12 phase 4 card 已完成; note in section 11 N5 that the user dropped PDU Release; header badge 第 1–4 期已完成; add a 待優化 card in section 12 pointing at the two improvement items. Publish to the artifact URL.
- [ ] **Step 4: Commit** `docs: phase 4 in the tester guide and design record`.
