# Throughput Tester Phase 2 (Registration + PDU Session) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Every UE whose gNB is up registers with the core and establishes one PDU session. Two paced stages run back to back, registration → PDU, each with a rate, an in-flight cap, a timeout, and retries. The Run page shows per-stage statistics, a UE state summary, and the failed UEs with their causes.

**Architecture:**
- New package `ue` is one UE's NAS layer and does no I/O. It copies free-ran-ue's `ue/nas.go` and `ue/security.go`, reshaped into "downlink bytes in → reply + event out".
- In `gnb`, a new `Association` multiplexes all of a gNB's UEs over its one SCTP association. It stands in for the Uu link and answers Initial Context Setup, PDU Session Resource Setup and UE Context Release itself.
- New package `procedure` drives one registration or one PDU attempt for one UE and classifies the result.
- `run` gets a paced `procStage` per stage and wires N2 → registration → PDU: a UE starts as soon as its own gNB is up.
- `internal/fakecore` is a test-only network side (NAS + NGAP AMF over a buffered message pipe). It lets every layer be tested end to end without a core.

**Tech Stack:** Go 1.26.2, free5gc/nas v1.3.0, free5gc/util v1.4.0 (milenage, ueauth), free5gc/ngap v1.2.0, React + generated OpenAPI client.

**Spec:**
- Design artifact https://claude.ai/artifact/JmvGVKAnieV1CvNAbnkf9f (read with the Artifact tool): sections 4 (gNB↔UE direct link), 5 (UE pipeline), 6 (UE template, pacing), 9 (metric definitions), 12 "第 2 期", 13 (Phase 1 record).
- Answers: `/home/alonza/fru-lab/5g-throughput-tester.md` `## Q` / `## N`. This file is **untracked**, so read it from the main checkout.
- Base branch: `feat/throughput-tester-phase1`. Phase 2 stacks on it because Phase 1 is not merged yet.

## Global Constraints

- Everything in the Phase 1 plan's Global Constraints still holds (`docs/superpowers/plans/2026-10-02-throughput-tester-phase1.md`).
- New dependencies, pinned to free-ran-ue's versions: `github.com/free5gc/nas v1.3.0`, `github.com/free5gc/util v1.4.0`.
- The tester does **not** create subscribers (Q7). The setup page shows the SUPI range the user must provision.
- One PDU session per UE (`PduSessionID = 1`), one DNN (Q14). GTP-U carries no QFI (Q15). The QFI from the AMF is still recorded for Phase 3.
- A UE enters registration as soon as its gNB's NG Setup succeeds. It does not wait for the other gNBs (design §5). It enters PDU as soon as it is registered (spec flow step 6).
- Per-stage pacing: at most `ratePerSec` starts per second (token bucket of depth 1) and at most `maxInFlight` at once. Each attempt is bounded by `timeoutMs` (1..60000). A failure with retries left is requeued at the back (Q13, N3).
- Timing (design §9):
  - **Registration:** from Initial UE Message sent to Registration Complete sent.
  - **PDU:** from the request sent to the accept received; the gNB has answered PDU Session Resource Setup by then.
  - **Latency** is measured from a UE's first attempt.
  - **Total time** runs from the first start to the last finish (Q8).
- Stop (N4): queued UEs are cancelled, and in-flight attempts finish or time out. Every unfinished item is counted as `skipped`, so every stage ends `Done` and its total time freezes. PDU release and deregistration are Phase 4.
- A UE whose gNB never came up is `skipped` (shown as "gNB down"). A UE whose registration fails skips the PDU stage.
- The failed-UE list in the snapshot is capped at 200 rows; `ues.failed` is the total.
- The stream sends at most one frame every 200 ms. This is deferred minor M4 from Phase 1, and it becomes necessary now that every UE step is a state change.
- UI copy is in English. Verify the frontend with `yarn build`; eslint's config is still broken repo-wide.

## Review Focus

1. **A UE whose subscriber is on a slice the gNB does not advertise** (real free5GC answers 5GMM cause 62): it is counted as `rejected`, appears in the failed list with `5gmm(62)`, and the other UEs carry on. This was seen against the user's core during prototyping. Pinned by `TestRegistrationRejectIsRetriedThenListed` (Task 6, injected via fakecore).
2. **Stop while UEs are still queued for registration**: queued UEs become `cancelled`, every stage reports `Done`, and total time stops growing. Pinned by `TestStopDuringRegistrationCancelsQueuedUes` (Task 6).
3. **The AMF sends a Configuration Update Command right after Registration Complete** (free5GC does): the UE ignores it, and its PDU request still succeeds. Pinned by `TestRegistrationAndPduSession` (Task 3) and `TestRegisterThenEstablishPdu` (Task 4).
4. **Many UEs interleaving on one association**: every UE gets its own RAN UE NGAP ID, UE IP and DL TEID, and no downlink reaches the wrong UE. Pinned by `TestManyUesInterleaveOnOneAssociation` (Task 4).
5. **The association drops mid-registration**: the attempt fails with "association lost" and does not hang until its timeout. Pinned by `TestAssociationLostMidRegistration` (Task 4).

Known Phase 2 limitations, documented in Task 11:
- UEs are not deregistered on Stop; the AMF drops their contexts when the SCTP association closes (Phase 4 adds Release/Dereg).
- The SQN in AUTN is accepted without a freshness check, as in free-ran-ue.
- A UE that becomes `established` on a gNB that is later `lost` stays counted as established.

## File Structure

| Path | Responsibility |
|---|---|
| `tester/profile/profile.go` | + `UeTemplate`, `ProcedureRate`, `Rates.Registration/Pdu` |
| `tester/profile/increment.go` | + `IncrementDecimal` (MSIN) |
| `tester/profile/expand.go` | + UE expansion (`Plan.Ues`, per-gNB SUPI range), UE/rate validation |
| `tester/metrics/stage.go` | + `Skip()`, `skipped`, Done counts skipped |
| `tester/ue/security.go`, `ue.go` | UE NAS layer: 5G-AKA, security mode, registration, PDU request/accept |
| `tester/internal/fakecore/nas.go` | Test network side of NAS (independent key derivation) |
| `tester/internal/fakecore/pipe.go`, `amf.go` | Buffered message pipe; NGAP fake AMF |
| `tester/gnb/ngapmsg.go`, `association.go` | UE-associated NGAP messages; per-gNB multiplexer with per-UE mailboxes |
| `tester/procedure/procedure.go` | One registration / one PDU attempt → `Outcome` |
| `tester/run/pipeline.go` | `procStage`: rate + in-flight cap + requeue + drain |
| `tester/run/snapshot.go`, `controller.go` | UE states, summary, failed list; N2 → registration → PDU wiring |
| `tester/api/api.go` | Stream frame throttle |
| `web/openapi.yaml` + client | New schemas/fields |
| `web/frontend/src/page/tester/*` | UE template + pacing on Setup; stage cards, UE summary and failed list on Run |
| `docs/tester-guide.md` | Phase 2 usage, subscribers, numbers, limitations |

---

### Task 0: Branch

- [ ] **Step 1:** `git checkout feat/throughput-tester-phase1 && git checkout -b feat/throughput-tester-phase2`

---

### Task 1: Profile — UE template, procedure rates, UE expansion

**Files:** Modify `tester/profile/profile.go`, `tester/profile/increment.go`, `tester/profile/increment_test.go`, `tester/profile/expand.go`, `tester/profile/expand_test.go`

**Interfaces:**
- Produces:
  - `profile.UeTemplate{MsinStart, Key, Opc, Amf, Sqn, Integrity, Ciphering, Dnn string; Sst int; Sd string}`, reached as `Profile.Ue`.
  - `profile.ProcedureRate{RatePerSec, MaxInFlight, TimeoutMs, Retries int}`, reached as `Rates.Registration` and `Rates.Pdu`.
  - `profile.IncrementDecimal(start string, step int) (string, error)`.
  - `profile.UeSpec{Index, Gnb int; Msin, Supi string}`, in `Plan.Ues` (`json:"-"`).
  - `GnbSpec.FirstSupi` and `GnbSpec.LastSupi`.
  - New field-error paths `ue.*`, `rates.registration.*`, `rates.pdu.*`.

- [ ] **Step 1: Write the failing tests.** Replace `tester/profile/expand_test.go` with:

```go
package profile

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
)

func sampleProfile() Profile {
	return Profile{
		Name:  "baseline",
		Scale: Scale{GnbCount: 3, UeCount: 10},
		Gnb: GnbTemplate{
			GnbIDStart: "000314", NamePattern: "gNB-{i}",
			Mcc: "208", Mnc: "93", Tac: "000001", Sst: 1, Sd: "010203",
		},
		Ue: UeTemplate{
			MsinStart: "0000000001", Key: "8baf473f2f8fd09487cccbd7097c6862",
			Opc: "8e27b6af0e692e750f32667a3b14605d", Amf: "8000", Sqn: "000000000023",
			Integrity: "nia2", Ciphering: "nea0", Dnn: "internet", Sst: 1, Sd: "010203",
		},
		Network: Network{
			N2: N2Network{Interface: "ens19", Cidr: "10.0.1.0/24", StartIP: "10.0.1.1", AmfIP: "10.0.1.1", AmfPort: 38412},
			N3: N3Network{Interface: "ens20", Cidr: "10.0.2.0/24", StartIP: "10.0.2.2", UpfIP: "10.0.2.1", UpfPort: 2152},
		},
		Rates: Rates{
			N2:           StageRate{TimeoutMs: 5000, Retries: 1},
			Registration: ProcedureRate{RatePerSec: 50, MaxInFlight: 200, TimeoutMs: 10000, Retries: 1},
			Pdu:          ProcedureRate{RatePerSec: 50, MaxInFlight: 200, TimeoutMs: 10000, Retries: 1},
		},
	}
}

func TestExpandFillsGnbsInOrder(t *testing.T) {
	plan, err := Expand(sampleProfile(), []netip.Addr{netip.MustParseAddr("10.0.1.3")})
	require.NoError(t, err)
	require.Equal(t, 4, plan.UesPerGnb)
	require.Equal(t, 24, plan.N2Prefix)
	require.Equal(t, []GnbSpec{
		// .1 is the AMF, .3 is already on the host
		{Index: 1, Name: "gNB-1", GnbID: "000314", N2IP: "10.0.1.2", N3IP: "10.0.2.2", UeCount: 4, UeFirst: 1, UeLast: 4,
			FirstSupi: "imsi-208930000000001", LastSupi: "imsi-208930000000004"},
		{Index: 2, Name: "gNB-2", GnbID: "000315", N2IP: "10.0.1.4", N3IP: "10.0.2.3", UeCount: 4, UeFirst: 5, UeLast: 8,
			FirstSupi: "imsi-208930000000005", LastSupi: "imsi-208930000000008"},
		{Index: 3, Name: "gNB-3", GnbID: "000316", N2IP: "10.0.1.5", N3IP: "10.0.2.4", UeCount: 2, UeFirst: 9, UeLast: 10,
			FirstSupi: "imsi-208930000000009", LastSupi: "imsi-208930000000010"},
	}, plan.Gnbs)
	require.Len(t, plan.Ues, 10)
	require.Equal(t, UeSpec{Index: 10, Gnb: 2, Msin: "0000000010", Supi: "imsi-208930000000010"}, plan.Ues[9])
}

func TestExpandMoreGnbsThanUes(t *testing.T) {
	p := sampleProfile()
	p.Scale = Scale{GnbCount: 3, UeCount: 2}
	plan, err := Expand(p, nil)
	require.NoError(t, err)
	require.Equal(t, []int{1, 1, 0}, []int{plan.Gnbs[0].UeCount, plan.Gnbs[1].UeCount, plan.Gnbs[2].UeCount})
	require.Equal(t, 0, plan.Gnbs[2].UeFirst)
}

func TestExpandReportsEveryFieldError(t *testing.T) {
	p := sampleProfile()
	p.Name = " "
	p.Scale.GnbCount = 0
	p.Gnb.NamePattern = "gNB"
	p.Gnb.Tac = "1"
	p.Network.N2.AmfIP = "amf"
	p.Rates.N2.TimeoutMs = 0
	_, err := Expand(p, nil)
	var verr *ValidationError
	require.ErrorAs(t, err, &verr)
	fields := []string{}
	for _, fe := range verr.Errors {
		fields = append(fields, fe.Field)
	}
	require.ElementsMatch(t, []string{"name", "scale.gnbCount", "gnb.namePattern", "gnb.tac", "network.n2.amfIp", "rates.n2.timeoutMs"}, fields)
}

func TestExpandReportsCidrShortfallPerInterface(t *testing.T) {
	p := sampleProfile()
	p.Scale.GnbCount = 10
	p.Network.N2.Cidr = "10.0.1.0/29"
	_, err := Expand(p, nil)
	var verr *ValidationError
	require.ErrorAs(t, err, &verr)
	require.Equal(t, []FieldError{{Field: "network.n2.cidr", Message: "10.0.1.0/29 from 10.0.1.1 has only 5 usable IPs, need 10 (short by 5)"}}, verr.Errors)
}

func TestExpandReportsGnbIDOverflow(t *testing.T) {
	p := sampleProfile()
	p.Gnb.GnbIDStart = "fffffe"
	_, err := Expand(p, nil)
	var verr *ValidationError
	require.ErrorAs(t, err, &verr)
	require.Equal(t, "gnb.gnbIdStart", verr.Errors[0].Field)
}

func TestExpandSharedN2N3CidrNeverReusesAnIP(t *testing.T) {
	p := sampleProfile()
	p.Network.N3 = N3Network{Interface: "ens19", Cidr: "10.0.1.0/24", StartIP: "10.0.1.1", UpfIP: "10.0.1.6", UpfPort: 2152}
	plan, err := Expand(p, nil)
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, g := range plan.Gnbs {
		require.False(t, seen[g.N2IP], "duplicate %s", g.N2IP)
		seen[g.N2IP] = true
	}
	for _, g := range plan.Gnbs {
		require.False(t, seen[g.N3IP], "N3 IP %s reuses an N2 IP", g.N3IP)
		seen[g.N3IP] = true
	}
	for _, core := range []string{"10.0.1.1", "10.0.1.6"} { // AMF, UPF
		require.False(t, seen[core], "core IP %s was handed to a gNB", core)
	}
	// N2 takes .2-.4 (AMF is .1); N3 skips those and the UPF at .6
	require.Equal(t, []string{"10.0.1.5", "10.0.1.7", "10.0.1.8"},
		[]string{plan.Gnbs[0].N3IP, plan.Gnbs[1].N3IP, plan.Gnbs[2].N3IP})
}

func TestExpandBoundsN2Timeout(t *testing.T) {
	p := sampleProfile()
	p.Rates.N2.TimeoutMs = 60001
	_, err := Expand(p, nil)
	var verr *ValidationError
	require.ErrorAs(t, err, &verr)
	require.Equal(t, []FieldError{{Field: "rates.n2.timeoutMs", Message: "must be between 1 and 60000"}}, verr.Errors)
}

func TestExpandValidatesUeTemplateAndRates(t *testing.T) {
	p := sampleProfile()
	p.Ue.MsinStart = "000001" // 3+2+6 = 11 digits
	p.Ue.Key = "xyz"
	p.Ue.Integrity = "nia9"
	p.Ue.Dnn = ""
	p.Rates.Registration.RatePerSec = 0
	p.Rates.Pdu.TimeoutMs = 60001
	_, err := Expand(p, nil)
	var verr *ValidationError
	require.ErrorAs(t, err, &verr)
	require.ElementsMatch(t, []FieldError{
		{Field: "ue.msinStart", Message: "must be 10 digits so MCC+MNC+MSIN is 15"},
		{Field: "ue.key", Message: "must be 32 hex digits"},
		{Field: "ue.integrity", Message: "must be one of nia0, nia1, nia2, nia3"},
		{Field: "ue.dnn", Message: "must not be empty"},
		{Field: "rates.registration.ratePerSec", Message: "must be between 1 and 100000"},
		{Field: "rates.pdu.timeoutMs", Message: "must be between 1 and 60000"},
	}, verr.Errors)
}

func TestExpandReportsMsinOverflow(t *testing.T) {
	p := sampleProfile()
	p.Ue.MsinStart = "9999999995" // 10 UEs need ...04
	_, err := Expand(p, nil)
	var verr *ValidationError
	require.ErrorAs(t, err, &verr)
	require.Equal(t, "ue.msinStart", verr.Errors[0].Field)
	require.Contains(t, verr.Errors[0].Message, "overflows")
}
```

Append to `tester/profile/increment_test.go`:

```go
func TestIncrementDecimal(t *testing.T) {
	got, err := IncrementDecimal("0000000009", 1)
	require.NoError(t, err)
	require.Equal(t, "0000000010", got)
	got, err = IncrementDecimal("0000000001", 999)
	require.NoError(t, err)
	require.Equal(t, "0000001000", got)

	_, err = IncrementDecimal("9999999999", 1)
	require.ErrorContains(t, err, "overflows")
	_, err = IncrementDecimal("12a", 0)
	require.ErrorContains(t, err, "decimal")
	_, err = IncrementDecimal("", 0)
	require.ErrorContains(t, err, "decimal")
}
```

- [ ] **Step 2: Run them to make sure they fail.** Run: `cd tester && go test ./profile/`. Expected: FAIL to build (`unknown field Ue`, `undefined: IncrementDecimal`, `UeSpec`, …).

- [ ] **Step 3: Implement.** Replace `tester/profile/profile.go`:

```go
// Package profile holds the user-facing description of a throughput test
// and turns it into the concrete per-gNB settings a run needs.
package profile

// Profile is what the setup page edits and what a run starts from. Field
// names are the JSON contract shared with fru-lab's frontend.
type Profile struct {
	Name    string      `json:"name"`
	Scale   Scale       `json:"scale"`
	Gnb     GnbTemplate `json:"gnb"`
	Ue      UeTemplate  `json:"ue"`
	Network Network     `json:"network"`
	Rates   Rates       `json:"rates"`
}

type Scale struct {
	GnbCount int `json:"gnbCount"`
	UeCount  int `json:"ueCount"`
}

// GnbTemplate is expanded once per gNB. GnbIDStart is hex and keeps its
// width when incremented; NamePattern must contain "{i}" (1-based index).
type GnbTemplate struct {
	GnbIDStart  string `json:"gnbIdStart"`
	NamePattern string `json:"namePattern"`
	Mcc         string `json:"mcc"`
	Mnc         string `json:"mnc"`
	Tac         string `json:"tac"`
	Sst         int    `json:"sst"`
	Sd          string `json:"sd"`
}

// UeTemplate is expanded once per UE. MsinStart is decimal and keeps its
// width when incremented; MCC+MNC+MSIN must be 15 digits. The UE uses the
// gNB template's PLMN. Key/Opc/Amf/Sqn must match the subscriber data the
// user created in the core (the tester does not provision subscribers).
type UeTemplate struct {
	MsinStart string `json:"msinStart"`
	Key       string `json:"key"`
	Opc       string `json:"opc"`
	Amf       string `json:"amf"`
	Sqn       string `json:"sqn"`
	Integrity string `json:"integrity"` // nia0..nia3
	Ciphering string `json:"ciphering"` // nea0..nea3
	Dnn       string `json:"dnn"`
	Sst       int    `json:"sst"`
	Sd        string `json:"sd"`
}

type Network struct {
	N2 N2Network `json:"n2"`
	N3 N3Network `json:"n3"`
}

// N2Network gives each gNB its own local IP from Cidr, starting at StartIP.
type N2Network struct {
	Interface string `json:"interface"`
	Cidr      string `json:"cidr"`
	StartIP   string `json:"startIp"`
	AmfIP     string `json:"amfIp"`
	AmfPort   int    `json:"amfPort"`
}

// N3Network is validated and allocated now so the setup page can show the
// full per-gNB plan, but nothing binds to these IPs until phase 3.
type N3Network struct {
	Interface string `json:"interface"`
	Cidr      string `json:"cidr"`
	StartIP   string `json:"startIp"`
	UpfIP     string `json:"upfIp"`
	UpfPort   int    `json:"upfPort"`
}

type Rates struct {
	N2           StageRate     `json:"n2"`
	Registration ProcedureRate `json:"registration"`
	Pdu          ProcedureRate `json:"pdu"`
}

// ProcedureRate paces a per-UE stage: at most RatePerSec new attempts per
// second (token bucket) and at most MaxInFlight attempts at once
// (semaphore), each bounded by TimeoutMs; Retries as in StageRate.
type ProcedureRate struct {
	RatePerSec  int `json:"ratePerSec"`
	MaxInFlight int `json:"maxInFlight"`
	TimeoutMs   int `json:"timeoutMs"`
	Retries     int `json:"retries"`
}

// StageRate bounds one attempt with TimeoutMs; Retries is how many more
// attempts a failed item gets (it is requeued at the back each time).
type StageRate struct {
	TimeoutMs int `json:"timeoutMs"`
	Retries   int `json:"retries"`
}
```

Replace `tester/profile/increment.go`:

```go
package profile

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

// IncrementHex adds step to a hex string and keeps its width, so "000314"
// + 2 is "000316". It fails instead of growing the string on overflow.
func IncrementHex(start string, step int) (string, error) {
	if _, err := hex.DecodeString(start); err != nil || start == "" {
		return "", fmt.Errorf("%q is not an even-length hex string", start)
	}
	v, _ := new(big.Int).SetString(start, 16)
	v.Add(v, big.NewInt(int64(step)))
	out := fmt.Sprintf("%0*x", len(start), v)
	if len(out) > len(start) {
		return "", fmt.Errorf("%q + %d overflows %d hex digits", start, step, len(start))
	}
	return out, nil
}

// IncrementDecimal adds step to a decimal string and keeps its width, so
// "0000000009" + 1 is "0000000010". It fails instead of growing.
func IncrementDecimal(start string, step int) (string, error) {
	if start == "" || strings.Trim(start, "0123456789") != "" {
		return "", fmt.Errorf("%q is not a decimal string", start)
	}
	v, _ := new(big.Int).SetString(start, 10)
	v.Add(v, big.NewInt(int64(step)))
	out := fmt.Sprintf("%0*s", len(start), v.String())
	if len(out) > len(start) {
		return "", fmt.Errorf("%q + %d overflows %d digits", start, step, len(start))
	}
	return out, nil
}

// RenderName replaces every "{i}" in pattern with the 1-based index.
func RenderName(pattern string, index int) string {
	return strings.ReplaceAll(pattern, "{i}", strconv.Itoa(index))
}
```

Replace `tester/profile/expand.go`:

```go
package profile

import (
	"fmt"
	"net/netip"
	"regexp"
	"strings"
)

// GnbSpec is one concrete gNB a run will bring up. UeFirst/UeLast are the
// 1-based UE indexes it owns (both 0 when it owns none); FirstSupi and
// LastSupi are those UEs' SUPIs, so the user can check them against the
// subscribers in the core.
type GnbSpec struct {
	Index     int    `json:"index"`
	Name      string `json:"name"`
	GnbID     string `json:"gnbId"`
	N2IP      string `json:"n2Ip"`
	N3IP      string `json:"n3Ip"`
	UeCount   int    `json:"ueCount"`
	UeFirst   int    `json:"ueFirst"`
	UeLast    int    `json:"ueLast"`
	FirstSupi string `json:"firstSupi"`
	LastSupi  string `json:"lastSupi"`
}

// UeSpec is one concrete UE. Gnb is the 0-based index into Plan.Gnbs.
type UeSpec struct {
	Index int    // 1-based
	Gnb   int    // 0-based index into Plan.Gnbs
	Msin  string // incremented from UeTemplate.MsinStart
	Supi  string // "imsi-" + MCC + MNC + MSIN
}

// Plan is a profile expanded against a specific host. Ues is left out of
// the JSON: the setup page only needs the per-gNB SUPI ranges, and a full
// UE list would make every validate response O(UE count).
type Plan struct {
	Gnbs      []GnbSpec `json:"gnbs"`
	Ues       []UeSpec  `json:"-"`
	UesPerGnb int       `json:"uesPerGnb"`
	N2Prefix  int       `json:"n2Prefix"`
	N3Prefix  int       `json:"n3Prefix"`
}

var (
	reMcc  = regexp.MustCompile(`^[0-9]{3}$`)
	reMnc  = regexp.MustCompile(`^[0-9]{2,3}$`)
	reHex6 = regexp.MustCompile(`^[0-9a-fA-F]{6}$`)
	reHex  = func(n int) *regexp.Regexp { return regexp.MustCompile(fmt.Sprintf(`^[0-9a-fA-F]{%d}$`, n)) }
	reKey  = reHex(32)
	reAmf  = reHex(4)
	reSqn  = reHex(12)
	reNia  = regexp.MustCompile(`^nia[0-3]$`)
	reNea  = regexp.MustCompile(`^nea[0-3]$`)
)

// Expand validates p and, if it is valid, assigns every gNB its ID, name,
// N2/N3 IPs and UE range. hostIPs are addresses already configured on this
// machine; they are never handed out. All problems are reported together
// in a *ValidationError.
func Expand(p Profile, hostIPs []netip.Addr) (*Plan, error) {
	verr := &ValidationError{}
	validateFields(p, verr)
	if len(verr.Errors) > 0 {
		return nil, verr
	}

	count := p.Scale.GnbCount
	// Neither core IP may be handed out on either side: N2 and N3 may share
	// one interface and CIDR.
	exclude := append([]netip.Addr{
		netip.MustParseAddr(p.Network.N2.AmfIP),
		netip.MustParseAddr(p.Network.N3.UpfIP),
	}, hostIPs...)
	n2IPs, err := AllocateIPs(p.Network.N2.Cidr, p.Network.N2.StartIP, count, exclude)
	if err != nil {
		verr.add("network.n2.cidr", err.Error())
	}
	// ...and N3 must never get an IP already given to a gNB's N2.
	n3Exclude := append(append([]netip.Addr{}, exclude...), n2IPs...)
	n3IPs, err := AllocateIPs(p.Network.N3.Cidr, p.Network.N3.StartIP, count, n3Exclude)
	if err != nil {
		verr.add("network.n3.cidr", err.Error())
	}
	if _, err := IncrementHex(p.Gnb.GnbIDStart, count-1); err != nil {
		verr.add("gnb.gnbIdStart", err.Error())
	}
	if _, err := IncrementDecimal(p.Ue.MsinStart, p.Scale.UeCount-1); err != nil {
		verr.add("ue.msinStart", err.Error())
	}
	if len(verr.Errors) > 0 {
		return nil, verr
	}

	perGnb := (p.Scale.UeCount + count - 1) / count
	plan := &Plan{
		Gnbs:      make([]GnbSpec, 0, count),
		UesPerGnb: perGnb,
		N2Prefix:  netip.MustParsePrefix(p.Network.N2.Cidr).Bits(),
		N3Prefix:  netip.MustParsePrefix(p.Network.N3.Cidr).Bits(),
	}
	plan.Ues = make([]UeSpec, 0, p.Scale.UeCount)
	nextUe := 1
	for i := 0; i < count; i++ {
		id, _ := IncrementHex(p.Gnb.GnbIDStart, i)
		ues := min(perGnb, p.Scale.UeCount-nextUe+1)
		spec := GnbSpec{
			Index:   i + 1,
			Name:    RenderName(p.Gnb.NamePattern, i+1),
			GnbID:   strings.ToLower(id),
			N2IP:    n2IPs[i].String(),
			N3IP:    n3IPs[i].String(),
			UeCount: ues,
		}
		for range ues {
			msin, _ := IncrementDecimal(p.Ue.MsinStart, nextUe-1)
			plan.Ues = append(plan.Ues, UeSpec{
				Index: nextUe, Gnb: i, Msin: msin,
				Supi: "imsi-" + p.Gnb.Mcc + p.Gnb.Mnc + msin,
			})
			nextUe++
		}
		if ues > 0 {
			spec.UeFirst, spec.UeLast = nextUe-ues, nextUe-1
			spec.FirstSupi = plan.Ues[spec.UeFirst-1].Supi
			spec.LastSupi = plan.Ues[spec.UeLast-1].Supi
		}
		plan.Gnbs = append(plan.Gnbs, spec)
	}
	return plan, nil
}

func validateFields(p Profile, verr *ValidationError) {
	if strings.TrimSpace(p.Name) == "" {
		verr.add("name", "must not be empty")
	}
	if p.Scale.GnbCount < 1 {
		verr.add("scale.gnbCount", "must be at least 1")
	}
	if p.Scale.UeCount < 1 {
		verr.add("scale.ueCount", "must be at least 1")
	}
	if len(p.Gnb.GnbIDStart) < 6 || len(p.Gnb.GnbIDStart) > 8 || len(p.Gnb.GnbIDStart)%2 != 0 {
		verr.add("gnb.gnbIdStart", "must be 6 or 8 hex digits")
	} else if _, err := IncrementHex(p.Gnb.GnbIDStart, 0); err != nil {
		verr.add("gnb.gnbIdStart", "must be 6 or 8 hex digits")
	}
	if !strings.Contains(p.Gnb.NamePattern, "{i}") {
		verr.add("gnb.namePattern", `must contain "{i}" so every gNB name is unique`)
	}
	if !reMcc.MatchString(p.Gnb.Mcc) {
		verr.add("gnb.mcc", "must be 3 digits")
	}
	if !reMnc.MatchString(p.Gnb.Mnc) {
		verr.add("gnb.mnc", "must be 2 or 3 digits")
	}
	if !reHex6.MatchString(p.Gnb.Tac) {
		verr.add("gnb.tac", "must be 6 hex digits")
	}
	if p.Gnb.Sst < 0 || p.Gnb.Sst > 255 {
		verr.add("gnb.sst", "must be between 0 and 255")
	}
	if p.Gnb.Sd != "" && !reHex6.MatchString(p.Gnb.Sd) {
		verr.add("gnb.sd", "must be empty or 6 hex digits")
	}
	validateUe(p, verr)
	validateEndpoint(verr, "network.n2", p.Network.N2.Interface, p.Network.N2.Cidr, p.Network.N2.StartIP, "amfIp", p.Network.N2.AmfIP, "amfPort", p.Network.N2.AmfPort)
	validateEndpoint(verr, "network.n3", p.Network.N3.Interface, p.Network.N3.Cidr, p.Network.N3.StartIP, "upfIp", p.Network.N3.UpfIP, "upfPort", p.Network.N3.UpfPort)
	// The upper bound keeps Stop prompt: in-flight attempts finish within
	// their timeout before teardown can start.
	if p.Rates.N2.TimeoutMs < 1 || p.Rates.N2.TimeoutMs > 60000 {
		verr.add("rates.n2.timeoutMs", "must be between 1 and 60000")
	}
	if p.Rates.N2.Retries < 0 {
		verr.add("rates.n2.retries", "must not be negative")
	}
	validateProcedureRate(verr, "rates.registration", p.Rates.Registration)
	validateProcedureRate(verr, "rates.pdu", p.Rates.Pdu)
}

func validateUe(p Profile, verr *ValidationError) {
	u := p.Ue
	if u.MsinStart == "" || strings.Trim(u.MsinStart, "0123456789") != "" {
		verr.add("ue.msinStart", "must be decimal digits")
	} else if n := len(p.Gnb.Mcc) + len(p.Gnb.Mnc) + len(u.MsinStart); reMcc.MatchString(p.Gnb.Mcc) && reMnc.MatchString(p.Gnb.Mnc) && n != 15 {
		verr.add("ue.msinStart", fmt.Sprintf("must be %d digits so MCC+MNC+MSIN is 15", 15-len(p.Gnb.Mcc)-len(p.Gnb.Mnc)))
	}
	if !reKey.MatchString(u.Key) {
		verr.add("ue.key", "must be 32 hex digits")
	}
	if !reKey.MatchString(u.Opc) {
		verr.add("ue.opc", "must be 32 hex digits")
	}
	if !reAmf.MatchString(u.Amf) {
		verr.add("ue.amf", "must be 4 hex digits")
	}
	if !reSqn.MatchString(u.Sqn) {
		verr.add("ue.sqn", "must be 12 hex digits")
	}
	if !reNia.MatchString(u.Integrity) {
		verr.add("ue.integrity", "must be one of nia0, nia1, nia2, nia3")
	}
	if !reNea.MatchString(u.Ciphering) {
		verr.add("ue.ciphering", "must be one of nea0, nea1, nea2, nea3")
	}
	if strings.TrimSpace(u.Dnn) == "" {
		verr.add("ue.dnn", "must not be empty")
	}
	if u.Sst < 0 || u.Sst > 255 {
		verr.add("ue.sst", "must be between 0 and 255")
	}
	if u.Sd != "" && !reHex6.MatchString(u.Sd) {
		verr.add("ue.sd", "must be empty or 6 hex digits")
	}
}

func validateProcedureRate(verr *ValidationError, base string, r ProcedureRate) {
	if r.RatePerSec < 1 || r.RatePerSec > 100000 {
		verr.add(base+".ratePerSec", "must be between 1 and 100000")
	}
	if r.MaxInFlight < 1 || r.MaxInFlight > 100000 {
		verr.add(base+".maxInFlight", "must be between 1 and 100000")
	}
	if r.TimeoutMs < 1 || r.TimeoutMs > 60000 {
		verr.add(base+".timeoutMs", "must be between 1 and 60000")
	}
	if r.Retries < 0 {
		verr.add(base+".retries", "must not be negative")
	}
}

func validateEndpoint(verr *ValidationError, base, iface, cidr, start, peerField, peerIP, portField string, port int) {
	if strings.TrimSpace(iface) == "" {
		verr.add(base+".interface", "must not be empty")
	}
	if p, err := netip.ParsePrefix(cidr); err != nil || !p.Addr().Is4() {
		verr.add(base+".cidr", fmt.Sprintf("%q is not an IPv4 CIDR", cidr))
	}
	if a, err := netip.ParseAddr(start); err != nil || !a.Is4() {
		verr.add(base+".startIp", fmt.Sprintf("%q is not an IPv4 address", start))
	}
	if a, err := netip.ParseAddr(peerIP); err != nil || !a.Is4() {
		verr.add(base+"."+peerField, fmt.Sprintf("%q is not an IPv4 address", peerIP))
	}
	if port < 1 || port > 65535 {
		verr.add(base+"."+portField, "must be between 1 and 65535")
	}
}
```

- [ ] **Step 4: Run the tests.** Run: `cd tester && go test ./profile/ && go vet ./...`. Expected: `ok tester/profile`. `go vet ./...` may fail in `run` because its test profile lacks `Ue`. That is fixed in Task 6; only `./profile/` must pass here.

- [ ] **Step 5: Commit** `feat: tester ue template, procedure rates and ue expansion` (with the Co-Authored-By trailer).

---

### Task 2: Metrics — skipped items

**Files:** Modify `tester/metrics/stage.go`, `tester/metrics/stage_test.go`

**Interfaces:**
- Produces: `(*metrics.Stage).Skip()` and `StageSnapshot.Skipped` (`json:"skipped"`).
- `Done` is true when `finished + skipped == expected`.
- If `Done` and no item ever finished, total time is 0.

- [ ] **Step 1: Write the failing tests.** Append to `tester/metrics/stage_test.go`:

```go
func TestStageSkippedItemsCompleteTheStage(t *testing.T) {
	clk := &fakeClock{t: time.Unix(1000, 0)}
	s := NewStage("registration", 3)
	s.now = clk.now
	s.Begin(false)
	clk.advance(5 * time.Millisecond)
	s.Finish(Accepted, 5*time.Millisecond, "")
	s.Skip()
	require.False(t, s.Snapshot().Done)
	s.Skip()
	clk.advance(time.Hour) // a stopped run's stage must not keep counting
	snap := s.Snapshot()
	require.True(t, snap.Done)
	require.Equal(t, int64(2), snap.Skipped)
	require.InDelta(t, 5.0, snap.TotalTimeMs, 0.001)
}

func TestStageAllStartedItemsSkippedAfterRetry(t *testing.T) {
	clk := &fakeClock{t: time.Unix(1000, 0)}
	s := NewStage("pdu", 1)
	s.now = clk.now
	s.Begin(false)
	s.Retrying()
	s.Skip() // stopped while waiting for its retry
	clk.advance(time.Hour)
	snap := s.Snapshot()
	require.True(t, snap.Done)
	require.Equal(t, 0.0, snap.TotalTimeMs)
}
```

- [ ] **Step 2: Run them to make sure they fail.** Run: `cd tester && go test ./metrics/`. Expected: FAIL (`s.Skip undefined`).

- [ ] **Step 3: Implement.** Replace `tester/metrics/stage.go`:

```go
package metrics

import (
	"sort"
	"sync"
	"time"
)

// Outcome is how one item (a gNB, later a UE) finished a stage.
type Outcome int

const (
	Accepted Outcome = iota // core said yes
	Rejected                // core said no (e.g. NGSetupFailure)
	TimedOut                // no answer before the stage timeout
	Failed                  // local or transport error (e.g. SCTP refused)
)

// Stage tracks one pipeline step. Total time is wall clock from the first
// Begin to the last final outcome, per the design's definition.
type Stage struct {
	mu        sync.Mutex
	name      string
	expected  int
	attempted int64
	retries   int64
	inFlight  int64
	skipped   int64
	outcomes  [4]int64
	causes    map[string]int64
	hist      histogram
	firstAt   time.Time
	lastAt    time.Time
	now       func() time.Time
}

func NewStage(name string, expected int) *Stage {
	return &Stage{name: name, expected: expected, causes: map[string]int64{}, now: time.Now}
}

// Begin marks one attempt starting. retry is true for the 2nd+ attempt of
// the same item, which is counted in Retries instead of Attempted.
func (s *Stage) Begin(retry bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.firstAt.IsZero() {
		s.firstAt = s.now()
	}
	if retry {
		s.retries++
	} else {
		s.attempted++
	}
	s.inFlight++
}

// Retrying closes an attempt that failed but will be tried again; it does
// not count as an outcome.
func (s *Stage) Retrying() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inFlight--
}

// Finish records an item's final outcome. latency is measured from that
// item's first attempt, so retries lengthen it. cause is ignored for
// Accepted.
func (s *Stage) Finish(o Outcome, latency time.Duration, cause string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inFlight--
	s.outcomes[o]++
	s.lastAt = s.now()
	if o == Accepted {
		s.hist.record(latency)
		return
	}
	if cause == "" {
		cause = "unknown"
	}
	s.causes[cause]++
}

// Skip records an item that will never be attempted (its gNB never came
// up, an earlier stage failed it, or the run was stopped first). Skipped
// items count toward Done but not toward any outcome or the total time.
func (s *Stage) Skip() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.skipped++
}

type CauseCount struct {
	Cause string `json:"cause"`
	Count int64  `json:"count"`
}

// StageSnapshot is the JSON the UI renders as one stage card. Latencies
// are milliseconds and cover accepted items only.
type StageSnapshot struct {
	Name        string       `json:"name"`
	Expected    int          `json:"expected"`
	Attempted   int64        `json:"attempted"`
	Retries     int64        `json:"retries"`
	InFlight    int64        `json:"inFlight"`
	Accepted    int64        `json:"accepted"`
	Rejected    int64        `json:"rejected"`
	TimedOut    int64        `json:"timedOut"`
	Failed      int64        `json:"failed"`
	Skipped     int64        `json:"skipped"`
	Done        bool         `json:"done"`
	TotalTimeMs float64      `json:"totalTimeMs"`
	AvgMs       float64      `json:"avgMs"`
	P50Ms       float64      `json:"p50Ms"`
	P95Ms       float64      `json:"p95Ms"`
	P99Ms       float64      `json:"p99Ms"`
	MaxMs       float64      `json:"maxMs"`
	Causes      []CauseCount `json:"causes"`
}

func ms(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }

func (s *Stage) Snapshot() StageSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	finished := s.outcomes[Accepted] + s.outcomes[Rejected] + s.outcomes[TimedOut] + s.outcomes[Failed]
	snap := StageSnapshot{
		Name:      s.name,
		Expected:  s.expected,
		Attempted: s.attempted,
		Retries:   s.retries,
		InFlight:  s.inFlight,
		Accepted:  s.outcomes[Accepted],
		Rejected:  s.outcomes[Rejected],
		TimedOut:  s.outcomes[TimedOut],
		Failed:    s.outcomes[Failed],
		Skipped:   s.skipped,
		Done:      int(finished+s.skipped) == s.expected,
		AvgMs:     ms(s.hist.mean()),
		P50Ms:     ms(s.hist.quantile(0.50)),
		P95Ms:     ms(s.hist.quantile(0.95)),
		P99Ms:     ms(s.hist.quantile(0.99)),
		MaxMs:     ms(s.hist.max),
		Causes:    make([]CauseCount, 0, len(s.causes)),
	}
	if !s.firstAt.IsZero() {
		end := s.now()
		if snap.Done {
			end = s.lastAt
			if end.IsZero() { // every started item was later skipped
				end = s.firstAt
			}
		}
		snap.TotalTimeMs = ms(end.Sub(s.firstAt))
	}
	for c, n := range s.causes {
		snap.Causes = append(snap.Causes, CauseCount{Cause: c, Count: n})
	}
	sort.Slice(snap.Causes, func(i, j int) bool {
		if snap.Causes[i].Count != snap.Causes[j].Count {
			return snap.Causes[i].Count > snap.Causes[j].Count
		}
		return snap.Causes[i].Cause < snap.Causes[j].Cause
	})
	return snap
}
```

- [ ] **Step 4: Run the tests.** Run: `cd tester && go test -race ./metrics/`. Expected: `ok`.
- [ ] **Step 5: Commit** `feat: count skipped items so stopped stages complete`.

---

### Task 3: UE NAS layer and fake network NAS

**Files:** Create `tester/ue/security.go`, `tester/ue/ue.go`, `tester/ue/ue_test.go`, `tester/internal/fakecore/nas.go`

**Interfaces:**
- Produces:
  - `ue.Config` and `ue.ConfigFrom(profile.UeSpec, profile.GnbTemplate, profile.UeTemplate) Config`.
  - `ue.New(Config) (*UE, error)`.
  - Methods `(*UE).RegistrationRequest() ([]byte, error)` (a fresh security context, so retries call it again), `PduSessionRequest() ([]byte, error)`, `Handle(nas []byte) (Result, error)` and `Supi() string`.
  - `ue.Result{Reply []byte; Event Event; Cause string; UeIP netip.Addr}`.
  - Events `EventNone`, `EventRegistered`, `EventRegistrationRejected`, `EventPduEstablished`, `EventPduRejected`.
  - Cause text formats: `"5gmm(N)"`, `"5gsm(N)"`, `"authentication reject"`.
- Produces (test-only):
  - `fakecore.Subscriber{Mcc, Mnc, Key, Opc, Amf, Sqn string}`.
  - `fakecore.Behavior{RejectRegistration, RejectPdu uint8; SendConfigUpdate, IgnorePduRequest, BadAutn bool}`.
  - `fakecore.NewNasSession(Subscriber, Behavior, netip.Addr) *NasSession` and `(*NasSession).Handle(uplink []byte) ([]Downlink, error)`.
  - `fakecore.Downlink{Nas []byte; InitialSetup, PduSetup bool}`.

The network side derives K_AUSF → K_SEAF → K_AMF on its own, using SQN⊕AK from AUTN, and checks RES* against XRES*. The UE and network tests therefore cross-check each other's derivations.

- [ ] **Step 1: Write the failing test** `tester/ue/ue_test.go`:

```go
package ue

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"tester/internal/fakecore"
)

func testConfig() Config {
	return Config{
		Supi: "imsi-208930000000001", Mcc: "208", Mnc: "93",
		Key: "8baf473f2f8fd09487cccbd7097c6862", Opc: "8e27b6af0e692e750f32667a3b14605d",
		Integrity: "nia2", Ciphering: "nea0", Dnn: "internet", Sst: 1, Sd: "010203",
	}
}

func testSubscriber() fakecore.Subscriber {
	return fakecore.Subscriber{Mcc: "208", Mnc: "93",
		Key: "8baf473f2f8fd09487cccbd7097c6862", Opc: "8e27b6af0e692e750f32667a3b14605d",
		Amf: "8000", Sqn: "000000000023"}
}

// exchange feeds one uplink to the network and every downlink back to
// the UE, returning the UE's results in order.
func exchange(t *testing.T, u *UE, net *fakecore.NasSession, uplink []byte) []Result {
	t.Helper()
	dls, err := net.Handle(uplink)
	require.NoError(t, err)
	out := []Result{}
	for _, dl := range dls {
		r, err := u.Handle(dl.Nas)
		require.NoError(t, err)
		out = append(out, r)
	}
	return out
}

func register(t *testing.T, u *UE, net *fakecore.NasSession) Result {
	t.Helper()
	req, err := u.RegistrationRequest()
	require.NoError(t, err)
	uplink := req
	for range 5 {
		results := exchange(t, u, net, uplink)
		require.Len(t, results, 1)
		r := results[0]
		if r.Event != EventNone {
			return r
		}
		require.NotNil(t, r.Reply)
		uplink = r.Reply
	}
	t.Fatal("registration did not finish in 5 round trips")
	return Result{}
}

func TestRegistrationAndPduSession(t *testing.T) {
	u, err := New(testConfig())
	require.NoError(t, err)
	net := fakecore.NewNasSession(testSubscriber(), fakecore.Behavior{SendConfigUpdate: true}, netip.MustParseAddr("10.60.0.7"))

	r := register(t, u, net)
	require.Equal(t, EventRegistered, r.Event)
	require.NotNil(t, r.Reply, "registration complete must be sent")

	// Registration Complete triggers a Configuration Update Command, which
	// the UE ignores.
	results := exchange(t, u, net, r.Reply)
	require.Equal(t, []Result{{}}, results)

	req, err := u.PduSessionRequest()
	require.NoError(t, err)
	results = exchange(t, u, net, req)
	require.Len(t, results, 1)
	require.Equal(t, EventPduEstablished, results[0].Event)
	require.Equal(t, netip.MustParseAddr("10.60.0.7"), results[0].UeIP)
}

func TestRegistrationRejected(t *testing.T) {
	u, err := New(testConfig())
	require.NoError(t, err)
	net := fakecore.NewNasSession(testSubscriber(), fakecore.Behavior{RejectRegistration: 7}, netip.Addr{})
	r := register(t, u, net)
	require.Equal(t, Result{Event: EventRegistrationRejected, Cause: "5gmm(7)"}, r)
}

func TestWrongKeyIsRejectedByNetwork(t *testing.T) {
	cfg := testConfig()
	cfg.Opc = "00000000000000000000000000000000"
	u, err := New(cfg)
	require.NoError(t, err)
	net := fakecore.NewNasSession(testSubscriber(), fakecore.Behavior{}, netip.Addr{})
	req, err := u.RegistrationRequest()
	require.NoError(t, err)
	dls, err := net.Handle(req)
	require.NoError(t, err)
	// With a wrong OPc the UE's AUTN MAC check fails locally.
	_, err = u.Handle(dls[0].Nas)
	require.ErrorContains(t, err, "verify autn")
}

func TestPduSessionRejected(t *testing.T) {
	u, err := New(testConfig())
	require.NoError(t, err)
	net := fakecore.NewNasSession(testSubscriber(), fakecore.Behavior{RejectPdu: 27}, netip.Addr{})
	r := register(t, u, net)
	require.Empty(t, exchange(t, u, net, r.Reply))
	req, err := u.PduSessionRequest()
	require.NoError(t, err)
	results := exchange(t, u, net, req)
	require.Equal(t, []Result{{Event: EventPduRejected, Cause: "5gsm(27)"}}, results)
}

func TestPduBeforeRegistrationIsAnError(t *testing.T) {
	u, err := New(testConfig())
	require.NoError(t, err)
	_, err = u.PduSessionRequest()
	require.ErrorContains(t, err, "before registration")
	_, err = u.Handle([]byte{0x7e, 0x00, 0x56})
	require.ErrorContains(t, err, "before registration request")
}

func TestRetryStartsFreshSecurityContext(t *testing.T) {
	u, err := New(testConfig())
	require.NoError(t, err)
	first := fakecore.NewNasSession(testSubscriber(), fakecore.Behavior{}, netip.Addr{})
	require.Equal(t, EventRegistered, register(t, u, first).Event)
	second := fakecore.NewNasSession(testSubscriber(), fakecore.Behavior{}, netip.Addr{})
	require.Equal(t, EventRegistered, register(t, u, second).Event)
}
```

- [ ] **Step 2: Run it to make sure it fails.** Run: `cd tester && go test ./ue/`. Expected: FAIL (`no non-test Go files` / `undefined: Config`).

- [ ] **Step 3: Implement the fake network** `tester/internal/fakecore/nas.go`:

```go
// Package fakecore is a minimal network side for tests: the NAS half of an
// AMF+SMF (5G-AKA, security mode, registration, PDU session) and, in
// amf.go, an NGAP AMF that serves gNB associations. It implements the key
// derivations independently of package ue, so tests cross-check them.
package fakecore

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/netip"

	"github.com/free5gc/nas/ie"
	"github.com/free5gc/nas/message"
	"github.com/free5gc/util/milenage"
	"github.com/free5gc/util/ueauth"
)

// Subscriber is what the fake UDM knows about one UE.
type Subscriber struct {
	Mcc, Mnc string
	Key, Opc string // 32 hex
	Amf      string // 4 hex
	Sqn      string // 12 hex
}

// Behavior injects failures. Zero value = everything succeeds.
type Behavior struct {
	RejectRegistration uint8 // 5GMM cause; 0 = accept
	RejectPdu          uint8 // 5GSM cause; 0 = accept
	SendConfigUpdate   bool  // send Configuration Update Command after Registration Complete
	IgnorePduRequest   bool  // never answer the PDU request (for timeouts)
	BadAutn            bool  // corrupt AUTN so the UE's MAC check fails
}

// NasSession is the network side of one UE's NAS signalling.
type NasSession struct {
	sub      Subscriber
	behavior Behavior
	ueIP     netip.Addr

	supi     string
	xresStar []byte
	kAmf     []byte
	secCtx   *message.SecCtx
	state    string
}

func NewNasSession(sub Subscriber, b Behavior, ueIP netip.Addr) *NasSession {
	return &NasSession{sub: sub, behavior: b, ueIP: ueIP, state: "idle"}
}

// Downlink is one NAS PDU for the UE. Nas is the PDU; PduSetup marks the
// PDU Session Establishment Accept, which the AMF must carry inside a PDU
// Session Resource Setup Request instead of a Downlink NAS Transport.
type Downlink struct {
	Nas          []byte
	InitialSetup bool // carry in Initial Context Setup Request (Registration Accept)
	PduSetup     bool
}

// Handle consumes one uplink NAS PDU and returns the downlink replies.
func (s *NasSession) Handle(uplink []byte) ([]Downlink, error) {
	switch s.state {
	case "idle":
		return s.onRegistrationRequest(uplink)
	case "auth":
		return s.onAuthResponse(uplink)
	case "smc":
		return s.onSecurityModeComplete(uplink)
	case "accepted":
		return s.onRegistrationComplete(uplink)
	case "registered":
		return s.onPduRequest(uplink)
	default:
		return nil, fmt.Errorf("unexpected uplink in state %s", s.state)
	}
}

func (s *NasSession) onRegistrationRequest(b []byte) ([]Downlink, error) {
	m, err := message.ParseGMM(b)
	if err != nil {
		return nil, fmt.Errorf("decode registration request: %w", err)
	}
	req, ok := m.(*message.RegReq)
	if !ok || req.MobileId5GS == nil {
		return nil, fmt.Errorf("expected registration request, got %T", m)
	}
	s.supi = "imsi-" + supiDigits(req.MobileId5GS)
	if s.behavior.RejectRegistration != 0 {
		s.state = "done"
		rej, err := (&message.RegRej{Cause5GMM: &ie.Cause5GMM{Value: s.behavior.RejectRegistration}}).MarshalBinary()
		return []Downlink{{Nas: rej}}, err
	}

	k, _ := hex.DecodeString(s.sub.Key)
	opc, _ := hex.DecodeString(s.sub.Opc)
	sqn, _ := hex.DecodeString(s.sub.Sqn)
	amf, _ := hex.DecodeString(s.sub.Amf)
	r := make([]byte, 16)
	_, _ = rand.Read(r)
	ik, ck, xres, autn, err := milenage.GenerateAKAParameters(opc, k, r, sqn, amf)
	if err != nil {
		return nil, err
	}
	if s.behavior.BadAutn {
		autn[len(autn)-1] ^= 0xff
	}
	sn := snName(s.sub.Mcc, s.sub.Mnc)
	ckik := append(append([]byte{}, ck...), ik...)
	out, err := ueauth.GetKDFValue(ckik, ueauth.FC_FOR_RES_STAR_XRES_STAR_DERIVATION,
		[]byte(sn), ueauth.KDFLen([]byte(sn)), r, ueauth.KDFLen(r), xres, ueauth.KDFLen(xres))
	if err != nil {
		return nil, err
	}
	s.xresStar = out[len(out)/2:]
	// SQN xor AK is the first 6 bytes of AUTN.
	kausf, _ := ueauth.GetKDFValue(ckik, ueauth.FC_FOR_KAUSF_DERIVATION,
		[]byte(sn), ueauth.KDFLen([]byte(sn)), autn[:6], ueauth.KDFLen(autn[:6]))
	kseaf, _ := ueauth.GetKDFValue(kausf, ueauth.FC_FOR_KSEAF_DERIVATION, []byte(sn), ueauth.KDFLen([]byte(sn)))
	p0, p1 := []byte(s.supi[len("imsi-"):]), []byte{0, 0}
	s.kAmf, _ = ueauth.GetKDFValue(kseaf, ueauth.FC_FOR_KAMF_DERIVATION, p0, ueauth.KDFLen(p0), p1, ueauth.KDFLen(p1))

	s.state = "auth"
	req2, err := (&message.AuthReq{
		Ngksi:                   &ie.NASKeySetId{Tsc: ie.SecCtxTypeNative, Ksi: 0},
		ABBA:                    &ie.ABBA{Abba: []byte{0, 0}},
		AuthParamRAND5GAuthChlg: &ie.AuthParamRAND{Rand: r},
		AuthParamAUTN5GAuthChlg: &ie.AuthParamAUTN{Autn: autn},
	}).MarshalBinary()
	return []Downlink{{Nas: req2}}, err
}

func (s *NasSession) onAuthResponse(b []byte) ([]Downlink, error) {
	m, err := message.ParseGMM(b)
	if err != nil {
		return nil, fmt.Errorf("decode authentication response: %w", err)
	}
	rsp, ok := m.(*message.AuthRsp)
	if !ok || rsp.AuthRspParam == nil {
		if _, failed := m.(*message.AuthFailure); failed {
			s.state = "done"
			return nil, errors.New("ue sent authentication failure")
		}
		return nil, fmt.Errorf("expected authentication response, got %T", m)
	}
	if !bytes.Equal(rsp.AuthRspParam.Res, s.xresStar) {
		s.state = "done"
		rej, err := (&message.AuthRej{}).MarshalBinary()
		return []Downlink{{Nas: rej}}, err
	}
	kenc, kint := nasKeys(s.kAmf, message.AlgCiphering128NEA0, message.AlgIntegrity128NIA2)
	s.secCtx = &message.SecCtx{
		Side: message.CoreNetworkSide, Bearer: message.Bearer3GPP,
		UplinkCount: &message.Count{}, DownlinkCount: &message.Count{},
		CipheringAlg: message.AlgCiphering128NEA0, IntegrityAlg: message.AlgIntegrity128NIA2,
		KnasEnc: kenc, KnasInt: kint,
	}
	s.state = "smc"
	smc, err := message.Marshal(&message.SecModeCmd{
		SelectedNASSecAlgos:       &ie.NASSecAlgos{CipheringAlgo: message.AlgCiphering128NEA0, MsgIntAlgo: message.AlgIntegrity128NIA2},
		Ngksi:                     &ie.NASKeySetId{Tsc: ie.SecCtxTypeNative, Ksi: 0},
		ReplayedUESecCapabilities: &ie.UESecCapability{Length: 2, EA05G: true, IA2_128_5G: true},
	}, s.secCtx, message.SecHdrTypeIntegrityProtectedWithNew5gNasSecCtx)
	return []Downlink{{Nas: smc}}, err
}

func (s *NasSession) onSecurityModeComplete(b []byte) ([]Downlink, error) {
	m, err := message.Parse(b, s.secCtx)
	if err != nil {
		return nil, fmt.Errorf("decode security mode complete: %w", err)
	}
	if _, ok := m.(*message.SecModeComplete); !ok {
		return nil, fmt.Errorf("expected security mode complete, got %T", m)
	}
	s.state = "accepted"
	acc, err := message.Marshal(&message.RegAccept{
		RegResult5GS: &ie.RegResult5GS{Value: ie.RegResult_3gpp},
	}, s.secCtx, message.SecHdrTypeIntegrityProtectedAndCiphered)
	return []Downlink{{Nas: acc, InitialSetup: true}}, err
}

func (s *NasSession) onRegistrationComplete(b []byte) ([]Downlink, error) {
	m, err := message.Parse(b, s.secCtx)
	if err != nil {
		return nil, fmt.Errorf("decode registration complete: %w", err)
	}
	if _, ok := m.(*message.RegComplete); !ok {
		return nil, fmt.Errorf("expected registration complete, got %T", m)
	}
	s.state = "registered"
	if !s.behavior.SendConfigUpdate {
		return nil, nil
	}
	cmd, err := message.Marshal(&message.CfgUpdateCmd{}, s.secCtx, message.SecHdrTypeIntegrityProtectedAndCiphered)
	return []Downlink{{Nas: cmd}}, err
}

func (s *NasSession) onPduRequest(b []byte) ([]Downlink, error) {
	m, err := message.Parse(b, s.secCtx)
	if err != nil {
		return nil, fmt.Errorf("decode ul nas transport: %w", err)
	}
	ul, ok := m.(*message.ULNASTransport)
	if !ok || ul.PayloadCntr == nil {
		return nil, fmt.Errorf("expected ul nas transport, got %T", m)
	}
	if _, err := message.ParseGSM(ul.PayloadCntr.Contents); err != nil {
		return nil, fmt.Errorf("decode pdu session establishment request: %w", err)
	}
	if s.behavior.IgnorePduRequest {
		return nil, nil
	}
	s.state = "done"
	var gsm []byte
	pduSetup := false
	if s.behavior.RejectPdu != 0 {
		gsm, err = (&message.PDUSessEstRej{PDUSessId: 1, Cause5GSM: &ie.Cause5GSM{Value: s.behavior.RejectPdu}}).MarshalBinary()
	} else {
		ip := s.ueIP.As4()
		gsm, err = (&message.PDUSessEstAccept{
			PDUSessId:           1,
			SelectedPDUSessType: &ie.PDUSessType{Value: ie.PDUSessType_IPv4},
			SelectedSSCMode:     &ie.SSCMode{Mode: ie.SSCMODE1},
			AuthoQosRules: &ie.QosRules{Rules: []ie.QosRule{{ // default rule, any-to-any, QFI 1
				RuleId: 1, OpCode: ie.OpCode_CreateNewQosRule, IsDefaultDQR: true, Precedence: 255, QFI: 1,
				PktFilterList: []ie.PacketFilter{{Dir: 3, Id: 1, Contents: ie.PacketFilterContents{RemoteAddr: "any", LocalAddr: "any"}}},
			}}},
			SessAMBR: &ie.SessAMBR{UnitDownlink: ie.Rate_1Mbps, ValueDownlink: 100, UnitUplink: ie.Rate_1Mbps, ValueUplink: 100},
			PDUAddr:  &ie.PDUAddr{IPv4: ip[:]},
		}).MarshalBinary()
		pduSetup = true
	}
	if err != nil {
		return nil, fmt.Errorf("encode 5gsm reply: %w", err)
	}
	dl, err := message.Marshal(&message.DLNASTransport{
		PayloadCntrType: &ie.PayloadCntrType{Value: ie.PayloadCntrType_N1SMInfo},
		PayloadCntr:     &ie.PayloadCntr{Pct: ie.PayloadCntrType_N1SMInfo, Contents: gsm},
		PDUSessID:       &ie.PDUSessId2{Value: 1},
	}, s.secCtx, message.SecHdrTypeIntegrityProtectedAndCiphered)
	return []Downlink{{Nas: dl, PduSetup: pduSetup}}, err
}

func nasKeys(kAmf []byte, enc ie.AlgCiphering, integ ie.AlgIntegrity) (kenc, kint [16]byte) {
	p0, p1 := []byte{message.NNASEncAlg}, []byte{byte(enc)}
	e, _ := ueauth.GetKDFValue(kAmf, ueauth.FC_FOR_ALGORITHM_KEY_DERIVATION, p0, ueauth.KDFLen(p0), p1, ueauth.KDFLen(p1))
	p0, p1 = []byte{message.NNASIntAlg}, []byte{byte(integ)}
	i, _ := ueauth.GetKDFValue(kAmf, ueauth.FC_FOR_ALGORITHM_KEY_DERIVATION, p0, ueauth.KDFLen(p0), p1, ueauth.KDFLen(p1))
	copy(kenc[:], e[16:])
	copy(kint[:], i[16:])
	return kenc, kint
}

func snName(mcc, mnc string) string {
	if len(mnc) == 2 {
		mnc = "0" + mnc
	}
	return "5G:mnc" + mnc + ".mcc" + mcc + ".3gppnetwork.org"
}

// supiDigits decodes the SUCI (null scheme) MCC+MNC+MSIN digits.
func supiDigits(id *ie.MobileId5GS) string {
	raw, _ := id.MarshalBinary()
	// raw: type octet, PLMN (3 octets BCD), routing indicator (2), scheme, hnpki, msin BCD
	bcd := func(b byte) string {
		lo, hi := b&0x0f, b>>4
		s := fmt.Sprintf("%d", lo)
		if hi != 0x0f {
			s += fmt.Sprintf("%d", hi)
		}
		return s
	}
	plmn := raw[1:4]
	mcc := fmt.Sprintf("%d%d%d", plmn[0]&0x0f, plmn[0]>>4, plmn[1]&0x0f)
	mnc := fmt.Sprintf("%d%d", plmn[2]&0x0f, plmn[2]>>4)
	if plmn[1]>>4 != 0x0f {
		mnc += fmt.Sprintf("%d", plmn[1]>>4)
	}
	msin := ""
	for _, b := range raw[8:] {
		msin += bcd(b)
	}
	return mcc + mnc + msin
}
```

- [ ] **Step 4: Implement the UE.** `tester/ue/security.go`:

```go
package ue

import (
	"fmt"
	"regexp"

	"github.com/free5gc/nas/ie"
	"github.com/free5gc/nas/message"
	"github.com/free5gc/util/milenage"
	"github.com/free5gc/util/ueauth"
)

// Key derivations copied from free-ran-ue ue/security.go (TS 33.501
// Annex A), reshaped to return values instead of mutating a UE.

var reSupiDigits = regexp.MustCompile(`(?:imsi|supi)-([0-9]{5,15})`)

func deriveKAmf(supi string, ckik []byte, snName string, sqn, ak []byte) ([]byte, error) {
	sqnXorAk := make([]byte, 6)
	for i := range sqnXorAk {
		sqnXorAk[i] = sqn[i] ^ ak[i]
	}
	kausf, err := ueauth.GetKDFValue(ckik, ueauth.FC_FOR_KAUSF_DERIVATION,
		[]byte(snName), ueauth.KDFLen([]byte(snName)), sqnXorAk, ueauth.KDFLen(sqnXorAk))
	if err != nil {
		return nil, fmt.Errorf("derive kausf: %w", err)
	}
	kseaf, err := ueauth.GetKDFValue(kausf, ueauth.FC_FOR_KSEAF_DERIVATION,
		[]byte(snName), ueauth.KDFLen([]byte(snName)))
	if err != nil {
		return nil, fmt.Errorf("derive kseaf: %w", err)
	}
	groups := reSupiDigits.FindStringSubmatch(supi)
	if groups == nil {
		return nil, fmt.Errorf("supi %q has no imsi digits", supi)
	}
	p0, p1 := []byte(groups[1]), []byte{0x00, 0x00} // ABBA = 0x0000
	return ueauth.GetKDFValue(kseaf, ueauth.FC_FOR_KAMF_DERIVATION, p0, ueauth.KDFLen(p0), p1, ueauth.KDFLen(p1))
}

// deriveNasKeys returns the 128-bit KNASenc and KNASint for the given
// algorithms (the low 16 bytes of each 256-bit KDF output).
func deriveNasKeys(kAmf []byte, enc ie.AlgCiphering, integ ie.AlgIntegrity) (kenc, kint [16]byte, err error) {
	p0, p1 := []byte{message.NNASEncAlg}, []byte{byte(enc)}
	e, err := ueauth.GetKDFValue(kAmf, ueauth.FC_FOR_ALGORITHM_KEY_DERIVATION, p0, ueauth.KDFLen(p0), p1, ueauth.KDFLen(p1))
	if err != nil {
		return kenc, kint, fmt.Errorf("derive knas_enc: %w", err)
	}
	p0, p1 = []byte{message.NNASIntAlg}, []byte{byte(integ)}
	i, err := ueauth.GetKDFValue(kAmf, ueauth.FC_FOR_ALGORITHM_KEY_DERIVATION, p0, ueauth.KDFLen(p0), p1, ueauth.KDFLen(p1))
	if err != nil {
		return kenc, kint, fmt.Errorf("derive knas_int: %w", err)
	}
	copy(kenc[:], e[16:32])
	copy(kint[:], i[16:32])
	return kenc, kint, nil
}

// akaResult is what the UE learns from a successful 5G-AKA challenge.
type akaResult struct {
	resStar []byte
	kAmf    []byte
}

// runAKA verifies AUTN (MAC check inside milenage) and derives RES* and
// KAMF. The SQN in AUTN is accepted as-is: like free-ran-ue, the tester
// does not keep SQN state, so re-running the same UEs never needs a resync.
func runAKA(supi string, k, opc, rand, autn []byte, snName string) (akaResult, error) {
	sqn, ak, ik, ck, res, err := milenage.GenerateKeysWithAUTN(opc, k, rand, autn)
	if err != nil {
		return akaResult{}, fmt.Errorf("verify autn: %w", err)
	}
	ckik := append(append([]byte{}, ck...), ik...)
	kAmf, err := deriveKAmf(supi, ckik, snName, sqn, ak)
	if err != nil {
		return akaResult{}, err
	}
	p0 := []byte(snName)
	out, err := ueauth.GetKDFValue(ckik, ueauth.FC_FOR_RES_STAR_XRES_STAR_DERIVATION,
		p0, ueauth.KDFLen(p0), rand, ueauth.KDFLen(rand), res, ueauth.KDFLen(res))
	if err != nil {
		return akaResult{}, fmt.Errorf("derive res*: %w", err)
	}
	return akaResult{resStar: out[len(out)/2:], kAmf: kAmf}, nil
}

// servingNetworkName is "5G:mncXXX.mccYYY.3gppnetwork.org" (TS 24.501).
func servingNetworkName(mcc, mnc string) string {
	if len(mnc) == 2 {
		mnc = "0" + mnc
	}
	return fmt.Sprintf("5G:mnc%s.mcc%s.3gppnetwork.org", mnc, mcc)
}
```

`tester/ue/ue.go`:

```go
// Package ue is one simulated UE's NAS layer: it builds uplink NAS
// messages and reacts to downlink ones, but does no I/O. The gNB layer
// carries the bytes; the procedure layer decides what to send when.
package ue

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net/netip"

	"github.com/free-ran-ue/util"
	"github.com/free5gc/nas/ie"
	"github.com/free5gc/nas/message"

	"tester/profile"
)

// PduSessionID is the one PDU session each UE establishes (spec Q14).
const PduSessionID uint8 = 1

// Config is everything a UE needs, already validated by profile.Expand.
type Config struct {
	Supi      string // "imsi-" + digits
	Mcc, Mnc  string
	Key, Opc  string // 32 hex
	Integrity string // nia0..nia3
	Ciphering string // nea0..nea3
	Dnn       string
	Sst       int
	Sd        string
}

// ConfigFrom combines one UeSpec with the templates it was expanded from.
func ConfigFrom(spec profile.UeSpec, g profile.GnbTemplate, u profile.UeTemplate) Config {
	return Config{
		Supi: spec.Supi, Mcc: g.Mcc, Mnc: g.Mnc, Key: u.Key, Opc: u.Opc,
		Integrity: u.Integrity, Ciphering: u.Ciphering, Dnn: u.Dnn, Sst: u.Sst, Sd: u.Sd,
	}
}

// Event says what a downlink message meant for the procedure in progress.
type Event int

const (
	EventNone                 Event = iota // keep waiting (reply, if any, still has to be sent)
	EventRegistered                        // Registration Accept handled; Reply is Registration Complete
	EventRegistrationRejected              // Registration/Authentication Reject; Cause set
	EventPduEstablished                    // PDU Session Establishment Accept; UeIP set
	EventPduRejected                       // PDU Session Establishment Reject or 5GMM cause; Cause set
)

// Result is the outcome of handling one downlink NAS message.
type Result struct {
	Reply []byte // uplink NAS to send back, nil if none
	Event Event
	Cause string // e.g. "5gmm(7)", "5gsm(27)", "authentication reject"
	UeIP  netip.Addr
}

// UE holds one UE's NAS security context. Not safe for concurrent use:
// a UE is driven by one procedure goroutine at a time.
type UE struct {
	cfg      Config
	digits   string // MCC+MNC+MSIN
	k, opc   []byte
	integ    ie.AlgIntegrity
	enc      ie.AlgCiphering
	secCap   *ie.UESecCapability
	mobileID *ie.MobileId5GS
	secCtx   *message.SecCtx
	kAmf     []byte
}

func New(cfg Config) (*UE, error) {
	k, err := hex.DecodeString(cfg.Key)
	if err != nil {
		return nil, fmt.Errorf("key: %w", err)
	}
	opc, err := hex.DecodeString(cfg.Opc)
	if err != nil {
		return nil, fmt.Errorf("opc: %w", err)
	}
	integ, enc, err := algorithms(cfg.Integrity, cfg.Ciphering)
	if err != nil {
		return nil, err
	}
	digits := cfg.Supi[len("imsi-"):]
	mobileID := new(ie.MobileId5GS)
	if err := mobileID.UnmarshalBinary(util.SupiToBytes(len(cfg.Mcc), len(cfg.Mnc), digits)); err != nil {
		return nil, fmt.Errorf("mobile identity for %s: %w", cfg.Supi, err)
	}
	return &UE{
		cfg: cfg, digits: digits, k: k, opc: opc, integ: integ, enc: enc,
		secCap: securityCapability(enc, integ), mobileID: mobileID,
	}, nil
}

func (u *UE) Supi() string { return u.cfg.Supi }

// RegistrationRequest starts an initial registration with a fresh
// security context, so it is also what a retry sends.
func (u *UE) RegistrationRequest() ([]byte, error) {
	u.secCtx = &message.SecCtx{
		Side: message.UESide, Bearer: message.Bearer3GPP,
		UplinkCount: &message.Count{}, DownlinkCount: &message.Count{},
		CipheringAlg: u.enc, IntegrityAlg: u.integ,
	}
	u.kAmf = nil
	return u.registrationRequest(nil)
}

func (u *UE) registrationRequest(cap5gmm *ie.Capability5GMM) ([]byte, error) {
	return (&message.RegReq{
		RegType5GS:      &ie.RegType5GS{FOR_Pending: true, Value: ie.RegType_InitialReg},
		Ngksi:           &ie.NASKeySetId{Tsc: ie.SecCtxTypeNative, Ksi: ie.NASKeyNA},
		MobileId5GS:     u.mobileID,
		UESecCapability: u.secCap,
		Capability5GMM:  cap5gmm,
	}).MarshalBinary()
}

// PduSessionRequest returns the protected UL NAS Transport carrying a PDU
// Session Establishment Request for PduSessionID. Call after EventRegistered.
func (u *UE) PduSessionRequest() ([]byte, error) {
	if u.kAmf == nil {
		return nil, errors.New("pdu session requested before registration")
	}
	inner, err := (&message.PDUSessEstReq{
		PDUSessId:                      PduSessionID,
		IntegrityProtectionMaxDataRate: &ie.IntegrityProtectionMaxDataRate{Uplink: 0xff, Downlink: 0xff},
		PDUSessType:                    &ie.PDUSessType{Value: ie.PDUSessType_IPv4},
		SSCMode:                        &ie.SSCMode{Mode: ie.SSCMODE1},
		ExtendedProtCfgOpts:            &ie.ExtendedProtCfgOpts{FromMs: &ie.ExtCfgOptFromMs{DNSV4Req: true, DNSV6Req: true}},
	}).MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("encode pdu session establishment request: %w", err)
	}
	ul := &message.ULNASTransport{
		PayloadCntrType: &ie.PayloadCntrType{Value: ie.PayloadCntrType_N1SMInfo},
		PayloadCntr:     &ie.PayloadCntr{Pct: ie.PayloadCntrType_N1SMInfo, Contents: inner},
		PDUSessID:       &ie.PDUSessId2{Value: PduSessionID},
		ReqType:         &ie.ReqType{Value: ie.ReqType_InitialReq},
		DNN:             &ie.DNN{Value: u.cfg.Dnn},
		SNSSAI:          &ie.SNSSAI{SST: uint8(u.cfg.Sst), SD: u.cfg.Sd},
	}
	return message.Marshal(ul, u.secCtx, message.SecHdrTypeIntegrityProtectedAndCiphered)
}

// Handle processes one downlink NAS PDU.
func (u *UE) Handle(pdu []byte) (Result, error) {
	if u.secCtx == nil {
		return Result{}, errors.New("downlink nas before registration request")
	}
	msg, err := message.Parse(pdu, u.secCtx)
	if err != nil {
		return Result{}, fmt.Errorf("decode downlink nas: %w", err)
	}
	switch m := msg.(type) {
	case *message.AuthReq:
		return u.onAuthRequest(m)
	case *message.SecModeCmd:
		return u.onSecurityModeCommand(m)
	case *message.RegAccept:
		reply, err := message.Marshal(&message.RegComplete{}, u.secCtx, message.SecHdrTypeIntegrityProtectedAndCiphered)
		if err != nil {
			return Result{}, fmt.Errorf("encode registration complete: %w", err)
		}
		return Result{Reply: reply, Event: EventRegistered}, nil
	case *message.RegRej:
		return Result{Event: EventRegistrationRejected, Cause: gmmCause(m.Cause5GMM)}, nil
	case *message.AuthRej:
		return Result{Event: EventRegistrationRejected, Cause: "authentication reject"}, nil
	case *message.DLNASTransport:
		return u.onDownlinkTransport(m)
	default:
		// Configuration Update Command, 5GMM Status, ...: nothing the
		// tester's procedures wait for.
		return Result{}, nil
	}
}

func (u *UE) onAuthRequest(m *message.AuthReq) (Result, error) {
	if m.AuthParamRAND5GAuthChlg == nil || m.AuthParamAUTN5GAuthChlg == nil {
		return Result{}, errors.New("authentication request without rand/autn")
	}
	aka, err := runAKA(u.cfg.Supi, u.k, u.opc, m.AuthParamRAND5GAuthChlg.Rand, m.AuthParamAUTN5GAuthChlg.Autn,
		servingNetworkName(u.cfg.Mcc, u.cfg.Mnc))
	if err != nil {
		return Result{}, err
	}
	u.kAmf = aka.kAmf
	if err := u.setNasKeys(u.enc, u.integ); err != nil {
		return Result{}, err
	}
	reply, err := (&message.AuthRsp{AuthRspParam: &ie.AuthRspParam{Res: aka.resStar}}).MarshalBinary()
	if err != nil {
		return Result{}, fmt.Errorf("encode authentication response: %w", err)
	}
	return Result{Reply: reply}, nil
}

func (u *UE) onSecurityModeCommand(m *message.SecModeCmd) (Result, error) {
	if u.kAmf == nil {
		return Result{}, errors.New("security mode command before authentication")
	}
	if m.SelectedNASSecAlgos != nil {
		if err := u.setNasKeys(m.SelectedNASSecAlgos.CipheringAlgo, m.SelectedNASSecAlgos.MsgIntAlgo); err != nil {
			return Result{}, err
		}
	}
	container, err := u.registrationRequest(&ie.Capability5GMM{Length: 1, S1Mode: true, HOAttach: true, LPP: true})
	if err != nil {
		return Result{}, fmt.Errorf("encode registration request container: %w", err)
	}
	imeisv := &ie.MobileId5GS{TypeOfId: ie.IdType_5GS_IMEISV, OddEvenIndic: ie.EvenNumOfIdDigit}
	imeisv.IMEISV[0], imeisv.IMEISV[14], imeisv.IMEISV[15] = 1, 1, 1
	complete := &message.SecModeComplete{IMEISV: imeisv, NASMsgCntr: &ie.NASMsgCntr{Contents: container}}
	reply, err := message.Marshal(complete, u.secCtx, message.SecHdrTypeIntegrityProtectedAndCipheredWithNew5gNasSecCtx)
	if err != nil {
		return Result{}, fmt.Errorf("encode security mode complete: %w", err)
	}
	return Result{Reply: reply}, nil
}

func (u *UE) onDownlinkTransport(m *message.DLNASTransport) (Result, error) {
	if m.PayloadCntr == nil {
		if m.Cause5GMM != nil { // e.g. "payload was not forwarded"
			return Result{Event: EventPduRejected, Cause: gmmCause(m.Cause5GMM)}, nil
		}
		return Result{}, nil
	}
	gsm, err := message.ParseGSM(m.PayloadCntr.Contents)
	if err != nil {
		return Result{}, fmt.Errorf("decode 5gsm payload: %w", err)
	}
	switch g := gsm.(type) {
	case *message.PDUSessEstAccept:
		if g.PDUAddr == nil || len(g.PDUAddr.IPv4) != 4 {
			return Result{Event: EventPduRejected, Cause: "accept without ipv4 address"}, nil
		}
		ip, _ := netip.AddrFromSlice(g.PDUAddr.IPv4)
		return Result{Event: EventPduEstablished, UeIP: ip}, nil
	case *message.PDUSessEstRej:
		return Result{Event: EventPduRejected, Cause: gsmCause(g.Cause5GSM)}, nil
	default:
		return Result{}, nil
	}
}

func (u *UE) setNasKeys(enc ie.AlgCiphering, integ ie.AlgIntegrity) error {
	kenc, kint, err := deriveNasKeys(u.kAmf, enc, integ)
	if err != nil {
		return err
	}
	u.secCtx.CipheringAlg, u.secCtx.IntegrityAlg = enc, integ
	u.secCtx.KnasEnc, u.secCtx.KnasInt = kenc, kint
	return nil
}

func gmmCause(c *ie.Cause5GMM) string {
	if c == nil {
		return "5gmm(unknown)"
	}
	return fmt.Sprintf("5gmm(%d)", c.Value)
}

func gsmCause(c *ie.Cause5GSM) string {
	if c == nil {
		return "5gsm(unknown)"
	}
	return fmt.Sprintf("5gsm(%d)", c.Value)
}

func algorithms(integrity, ciphering string) (ie.AlgIntegrity, ie.AlgCiphering, error) {
	integ := map[string]ie.AlgIntegrity{
		"nia0": message.AlgIntegrity128NIA0, "nia1": message.AlgIntegrity128NIA1,
		"nia2": message.AlgIntegrity128NIA2, "nia3": message.AlgIntegrity128NIA3,
	}
	enc := map[string]ie.AlgCiphering{
		"nea0": message.AlgCiphering128NEA0, "nea1": message.AlgCiphering128NEA1,
		"nea2": message.AlgCiphering128NEA2, "nea3": message.AlgCiphering128NEA3,
	}
	i, ok := integ[integrity]
	if !ok {
		return 0, 0, fmt.Errorf("unknown integrity algorithm %q", integrity)
	}
	e, ok := enc[ciphering]
	if !ok {
		return 0, 0, fmt.Errorf("unknown ciphering algorithm %q", ciphering)
	}
	return i, e, nil
}

func securityCapability(enc ie.AlgCiphering, integ ie.AlgIntegrity) *ie.UESecCapability {
	c := &ie.UESecCapability{Length: 2}
	switch enc {
	case message.AlgCiphering128NEA0:
		c.EA05G = true
	case message.AlgCiphering128NEA1:
		c.EA1_128_5G = true
	case message.AlgCiphering128NEA2:
		c.EA2_128_5G = true
	case message.AlgCiphering128NEA3:
		c.EA3_128_5G = true
	}
	switch integ {
	case message.AlgIntegrity128NIA0:
		c.IA05G = true
	case message.AlgIntegrity128NIA1:
		c.IA1_128_5G = true
	case message.AlgIntegrity128NIA2:
		c.IA2_128_5G = true
	case message.AlgIntegrity128NIA3:
		c.IA3_128_5G = true
	}
	return c
}
```

- [ ] **Step 5: Run the tests.** Run: `cd tester && go get github.com/free5gc/nas@v1.3.0 github.com/free5gc/util@v1.4.0 && go mod tidy && go test ./ue/`. Expected: `ok tester/ue`.
- [ ] **Step 6: Commit** `feat: ue nas layer with fake network for tests`.

---

### Task 4: NGAP association and per-UE procedures

**Files:**
- Create: `tester/gnb/ngapmsg.go`, `tester/gnb/association.go`, `tester/internal/fakecore/pipe.go`, `tester/internal/fakecore/amf.go`, `tester/procedure/procedure.go`, `tester/procedure/procedure_test.go`

**Interfaces:**
- Produces (gnb):
  - `gnb.NewAssociation(conn Conn, id Identity, n3IP netip.Addr, teids *TeidAllocator) *Association`.
  - Association methods: `Attach() (*UeLink, error)` (fresh RAN UE NGAP ID), `Detach(*UeLink)`, `SendInitialUE(*UeLink, []byte) error`, `SendUplinkNas(*UeLink, []byte) error`, `Run() error`.
  - `Run()` returns when the association fails. It treats EAGAIN and EINTR as idle, and every attached UE then gets `DownlinkLost`.
  - `gnb.UeLink{RanUeID int64; Downlinks chan Downlink}`.
  - `gnb.Downlink{Kind DownlinkKind; Nas []byte; Pdu *PduSetup; Err error}`, with kinds `DownlinkNas`, `DownlinkReleased` and `DownlinkLost`.
  - `gnb.PduSetup{UlTeid []byte; UpfIP netip.Addr; DlTeid uint32; Qfi int64}`.
  - `gnb.TeidAllocator` with `Allocate() uint32`.
  - Write failures are wrapped as `"association lost: …"`.
- Produces (procedure):
  - `procedure.Register(assoc, *ue.UE, timeout) (*gnb.UeLink, Outcome)`. The link stays attached only on success.
  - `procedure.EstablishPdu(assoc, link, *ue.UE, timeout) Outcome`.
  - `procedure.Outcome{Result metrics.Outcome; Cause string; UeIP netip.Addr; Pdu *gnb.PduSetup}`.
  - A timeout is reported as `TimedOut` with cause `"timeout"`.
- Produces (test-only):
  - `fakecore.Pipe() (*PipeEnd, *PipeEnd)`: buffered and message-preserving, so it cannot deadlock the way `net.Pipe` does when both sides write first.
  - `fakecore.AMF{Subscriber; Behavior func(supi) Behavior; UpfIP}`.
  - `(*AMF).Serve(io.ReadWriter) error`, `SetupResponses() []SetupResponse` and `ReleaseCompletes() int`.

The gNB answers Initial Context Setup, PDU Session Resource Setup (allocating a DL TEID and advertising its N3 IP) and UE Context Release itself, then hands the embedded NAS to the UE. The builders are copied from free-ran-ue's `gnb/ngapBuilder.go`, and the dispatch logic is adapted from its `gnb/ngapDispatcher.go`.

- [ ] **Step 1: Write the failing test** `tester/procedure/procedure_test.go`. Its last three tests pin Review Focus #3, #4 and #5.

```go
package procedure

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"tester/gnb"
	"tester/internal/fakecore"
	"tester/metrics"
	"tester/profile"
	"tester/ue"
)

var sub = fakecore.Subscriber{Mcc: "208", Mnc: "93",
	Key: "8baf473f2f8fd09487cccbd7097c6862", Opc: "8e27b6af0e692e750f32667a3b14605d",
	Amf: "8000", Sqn: "000000000023"}

// setup starts a fake AMF on one end of a pipe and an association on the
// other, both running until the test ends.
func setup(t *testing.T, behavior func(string) fakecore.Behavior) (*gnb.Association, *fakecore.AMF, *fakecore.PipeEnd) {
	t.Helper()
	gnbEnd, amfEnd := fakecore.Pipe()
	amf := &fakecore.AMF{Subscriber: sub, Behavior: behavior, UpfIP: netip.MustParseAddr("10.0.1.5")}
	go func() { _ = amf.Serve(amfEnd) }()
	id, err := gnb.NewIdentity(profile.GnbSpec{Name: "gNB-1", GnbID: "000314"},
		profile.GnbTemplate{Mcc: "208", Mnc: "93", Tac: "000001", Sst: 1, Sd: "010203"})
	require.NoError(t, err)
	assoc := gnb.NewAssociation(gnbEnd, id, netip.MustParseAddr("10.0.1.100"), &gnb.TeidAllocator{})
	go func() { _ = assoc.Run() }()
	t.Cleanup(func() { _ = gnbEnd.Close() })
	return assoc, amf, amfEnd
}

func newUE(t *testing.T, msin string) *ue.UE {
	t.Helper()
	u, err := ue.New(ue.Config{Supi: "imsi-20893" + msin, Mcc: "208", Mnc: "93",
		Key: sub.Key, Opc: sub.Opc, Integrity: "nia2", Ciphering: "nea0", Dnn: "internet", Sst: 1, Sd: "010203"})
	require.NoError(t, err)
	return u
}

func TestRegisterThenEstablishPdu(t *testing.T) {
	assoc, amf, _ := setup(t, func(string) fakecore.Behavior { return fakecore.Behavior{SendConfigUpdate: true} })
	u := newUE(t, "0000000001")

	link, out := Register(assoc, u, time.Second)
	require.Equal(t, Outcome{Result: metrics.Accepted}, out)
	require.NotNil(t, link)

	out = EstablishPdu(assoc, link, u, time.Second)
	require.Equal(t, metrics.Accepted, out.Result, out.Cause)
	require.Equal(t, netip.MustParseAddr("10.60.0.1"), out.UeIP)
	require.NotNil(t, out.Pdu)
	require.Equal(t, netip.MustParseAddr("10.0.1.5"), out.Pdu.UpfIP)
	require.Equal(t, []byte{0, 0, 0x10, 1}, out.Pdu.UlTeid)
	require.Equal(t, int64(9), out.Pdu.Qfi)

	require.Eventually(t, func() bool { return len(amf.SetupResponses()) == 1 }, time.Second, 5*time.Millisecond)
	rsp := amf.SetupResponses()[0]
	require.Equal(t, out.Pdu.DlTeid, rsp.DlTeid)
	require.Equal(t, netip.MustParseAddr("10.0.1.100"), rsp.GnbIP)
}

func TestRegistrationRejectedAndReleased(t *testing.T) {
	assoc, amf, _ := setup(t, func(string) fakecore.Behavior { return fakecore.Behavior{RejectRegistration: 7} })
	link, out := Register(assoc, newUE(t, "0000000001"), time.Second)
	require.Nil(t, link)
	require.Equal(t, Outcome{Result: metrics.Rejected, Cause: "5gmm(7)"}, out)
	require.Eventually(t, func() bool { return amf.ReleaseCompletes() == 1 }, time.Second, 5*time.Millisecond,
		"the gnb must answer the ue context release command")
}

func TestPduRejected(t *testing.T) {
	assoc, _, _ := setup(t, func(string) fakecore.Behavior { return fakecore.Behavior{RejectPdu: 27} })
	u := newUE(t, "0000000001")
	link, out := Register(assoc, u, time.Second)
	require.Equal(t, metrics.Accepted, out.Result)
	out = EstablishPdu(assoc, link, u, time.Second)
	require.Equal(t, Outcome{Result: metrics.Rejected, Cause: "5gsm(27)"}, out)
}

func TestPduTimeout(t *testing.T) {
	assoc, _, _ := setup(t, func(string) fakecore.Behavior { return fakecore.Behavior{IgnorePduRequest: true} })
	u := newUE(t, "0000000001")
	link, _ := Register(assoc, u, time.Second)
	start := time.Now()
	out := EstablishPdu(assoc, link, u, 100*time.Millisecond)
	require.Equal(t, Outcome{Result: metrics.TimedOut, Cause: "timeout"}, out)
	require.Less(t, time.Since(start), 500*time.Millisecond)
}

func TestAssociationLostMidRegistration(t *testing.T) {
	assoc, _, amfEnd := setup(t, func(string) fakecore.Behavior { return fakecore.Behavior{} })
	_ = amfEnd.Close() // the AMF goes away before anything is answered
	_, out := Register(assoc, newUE(t, "0000000001"), time.Second)
	require.Equal(t, metrics.Failed, out.Result)
	require.Contains(t, out.Cause, "association lost")
}

func TestManyUesInterleaveOnOneAssociation(t *testing.T) {
	assoc, amf, _ := setup(t, nil)
	const n = 50
	done := make(chan Outcome, n)
	for i := range n {
		go func() {
			u := newUE(t, "00000000"+string(rune('0'+i/10))+string(rune('0'+i%10)))
			link, out := Register(assoc, u, 2*time.Second)
			if out.Result != metrics.Accepted {
				done <- out
				return
			}
			done <- EstablishPdu(assoc, link, u, 2*time.Second)
		}()
	}
	ips := map[netip.Addr]bool{}
	for range n {
		out := <-done
		require.Equal(t, metrics.Accepted, out.Result, out.Cause)
		ips[out.UeIP] = true
	}
	require.Len(t, ips, n, "every UE gets its own address")
	require.Eventually(t, func() bool { return len(amf.SetupResponses()) == n }, time.Second, 5*time.Millisecond)
	teids := map[uint32]bool{}
	for _, r := range amf.SetupResponses() {
		teids[r.DlTeid] = true
	}
	require.Len(t, teids, n, "every UE gets its own DL TEID")
}
```

- [ ] **Step 2: Run it to make sure it fails.** Run: `cd tester && go test ./procedure/`. Expected: FAIL to build (`undefined: Register`, `fakecore.Pipe`, `gnb.NewAssociation`).

- [ ] **Step 3: Implement the test fakes.** `tester/internal/fakecore/pipe.go`:

```go
package fakecore

import (
	"io"
	"sync"
)

// Pipe returns two connected ends that keep SCTP's message boundaries
// and, unlike net.Pipe, buffer writes, so two sides that both write
// before reading (as a gNB and an AMF do) cannot deadlock.
func Pipe() (*PipeEnd, *PipeEnd) {
	ab, ba := newQueue(), newQueue()
	return &PipeEnd{in: ba, out: ab}, &PipeEnd{in: ab, out: ba}
}

type PipeEnd struct {
	in, out *queue
}

func (p *PipeEnd) Read(b []byte) (int, error)  { return p.in.pop(b) }
func (p *PipeEnd) Write(b []byte) (int, error) { return p.out.push(b) }

// Close ends both directions: the peer's reads return io.EOF.
func (p *PipeEnd) Close() error {
	p.in.close()
	p.out.close()
	return nil
}

type queue struct {
	mu     sync.Mutex
	cond   *sync.Cond
	msgs   [][]byte
	closed bool
}

func newQueue() *queue {
	q := &queue{}
	q.cond = sync.NewCond(&q.mu)
	return q
}

func (q *queue) push(b []byte) (int, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return 0, io.ErrClosedPipe
	}
	q.msgs = append(q.msgs, append([]byte(nil), b...))
	q.cond.Signal()
	return len(b), nil
}

func (q *queue) pop(b []byte) (int, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.msgs) == 0 && !q.closed {
		q.cond.Wait()
	}
	if len(q.msgs) == 0 {
		return 0, io.EOF
	}
	m := q.msgs[0]
	q.msgs = q.msgs[1:]
	return copy(b, m), nil
}

func (q *queue) close() {
	q.mu.Lock()
	q.closed = true
	q.cond.Broadcast()
	q.mu.Unlock()
}
```

`tester/internal/fakecore/amf.go`:

```go
package fakecore

import (
	"encoding/binary"
	"fmt"
	"io"
	"net/netip"
	"sync"
	"sync/atomic"

	"github.com/free5gc/ngap/aper"
	"github.com/free5gc/ngap/ie"
	"github.com/free5gc/ngap/message"
)

// AMF serves NGAP on gNB associations: NG Setup, then the UE-associated
// procedures, with NAS handled by one NasSession per UE.
type AMF struct {
	Subscriber Subscriber
	// Behavior picks the injected failures per SUPI; nil = all succeed.
	Behavior func(supi string) Behavior
	// UpfIP is put in every PDU Session Resource Setup Request.
	UpfIP netip.Addr

	nextAmfID atomic.Int64
	nextUeIP  atomic.Uint32
	mu        sync.Mutex
	setupRsps []SetupResponse
	released  int
}

// SetupResponse is what a gNB answered to a PDU Session Resource Setup.
type SetupResponse struct {
	RanUeID int64
	DlTeid  uint32
	GnbIP   netip.Addr
}

func (a *AMF) SetupResponses() []SetupResponse {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]SetupResponse(nil), a.setupRsps...)
}

func (a *AMF) ReleaseCompletes() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.released
}

type amfUe struct {
	amfID, ranID int64
	nas          *NasSession
}

// Serve handles one association until the conn is closed.
func (a *AMF) Serve(conn io.ReadWriter) error {
	ues := map[int64]*amfUe{} // by AMF UE NGAP ID
	buf := make([]byte, 65536)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			return err
		}
		msg, err := message.Parse(buf[:n])
		if err != nil {
			return fmt.Errorf("fake amf: decode: %w", err)
		}
		switch m := msg.(type) {
		case *message.NGSetupRequest:
			raw, encErr := ngSetupResponse(m)
			err = a.write(conn, raw, encErr)
		case *message.InitialUEMessage:
			ue := &amfUe{amfID: a.nextAmfID.Add(1), ranID: m.RANUENGAPID.Value}
			ue.nas = NewNasSession(a.Subscriber, Behavior{}, a.allocateUeIP())
			ues[ue.amfID] = ue
			err = a.onNas(conn, ue, m.NASPDU.Value)
		case *message.UplinkNASTransport:
			ue := ues[m.AMFUENGAPID.Value]
			if ue == nil {
				return fmt.Errorf("fake amf: unknown amf ue id %d", m.AMFUENGAPID.Value)
			}
			err = a.onNas(conn, ue, m.NASPDU.Value)
		case *message.PDUSessionResourceSetupResponse:
			a.recordSetupResponse(m)
		case *message.UEContextReleaseComplete:
			a.mu.Lock()
			a.released++
			a.mu.Unlock()
		}
		if err != nil {
			return err
		}
	}
}

func (a *AMF) allocateUeIP() netip.Addr {
	n := a.nextUeIP.Add(1)
	return netip.AddrFrom4([4]byte{10, 60, byte(n >> 8), byte(n)})
}

func (a *AMF) onNas(conn io.Writer, ue *amfUe, nas []byte) error {
	if ue.nas.state == "idle" && a.Behavior != nil {
		// pick the behavior once the SUPI is known (from the first message)
		probe := NewNasSession(a.Subscriber, Behavior{}, netip.Addr{})
		_, _ = probe.onRegistrationRequest(nas)
		ue.nas.behavior = a.Behavior(probe.supi)
	}
	dls, err := ue.nas.Handle(nas)
	if err != nil {
		return fmt.Errorf("fake amf: nas: %w", err)
	}
	for _, dl := range dls {
		var raw []byte
		switch {
		case dl.InitialSetup:
			raw, err = initialContextSetupRequest(ue, dl.Nas)
		case dl.PduSetup:
			raw, err = pduSessionResourceSetupRequest(ue, dl.Nas, a.UpfIP)
		default:
			raw, err = (&message.DownlinkNASTransport{
				AMFUENGAPID: &ie.AMFUENGAPID{Value: ue.amfID},
				RANUENGAPID: &ie.RANUENGAPID{Value: ue.ranID},
				NASPDU:      &ie.NASPDU{Value: aper.OctetString(dl.Nas)},
			}).MarshalBinary()
		}
		if err == nil {
			err = a.write(conn, raw, nil)
		}
		if err != nil {
			return err
		}
	}
	if ue.nas.state == "done" && ue.nas.behavior.RejectRegistration != 0 {
		raw, err := (&message.UEContextReleaseCommand{
			UENGAPIDs: &ie.UENGAPIDs{Choice: &ie.UENGAPIDPair{
				AMFUENGAPID: &ie.AMFUENGAPID{Value: ue.amfID},
				RANUENGAPID: &ie.RANUENGAPID{Value: ue.ranID},
			}},
			Cause: &ie.Cause{Choice: &ie.CauseNas{Value: ie.CauseNasPresentNormalRelease}},
		}).MarshalBinary()
		if err == nil {
			err = a.write(conn, raw, nil)
		}
		return err
	}
	return nil
}

func (a *AMF) write(conn io.Writer, raw []byte, err error) error {
	if err != nil {
		return err
	}
	_, err = conn.Write(raw)
	return err
}

func (a *AMF) recordSetupResponse(m *message.PDUSessionResourceSetupResponse) {
	if m.PDUSessionResourceSetupListSURes == nil || len(m.PDUSessionResourceSetupListSURes.List) == 0 {
		return
	}
	item := m.PDUSessionResourceSetupListSURes.List[0]
	var t ie.PDUSessionResourceSetupResponseTransfer
	if item.PDUSessionResourceSetupResponseTransfer == nil ||
		ie.UnmarshalBinary(*item.PDUSessionResourceSetupResponseTransfer, &t) != nil ||
		t.DLQosFlowPerTNLInformation == nil {
		return
	}
	tun, ok := t.DLQosFlowPerTNLInformation.UPTransportLayerInformation.Choice.(*ie.GTPTunnel)
	if !ok {
		return
	}
	ip, _ := netip.AddrFromSlice(tun.TransportLayerAddress.Value.Bytes)
	a.mu.Lock()
	a.setupRsps = append(a.setupRsps, SetupResponse{
		RanUeID: m.RANUENGAPID.Value, DlTeid: binary.BigEndian.Uint32(tun.GTPTEID.Value), GnbIP: ip,
	})
	a.mu.Unlock()
}

func ngSetupResponse(req *message.NGSetupRequest) ([]byte, error) {
	plmn := *req.GlobalRANNodeID.Choice.(*ie.GlobalGNBID).PLMNIdentity
	snssai := ie.SNSSAI{SST: &ie.SST{Value: aper.OctetString{1}}}
	return (&message.NGSetupResponse{
		AMFName: &ie.AMFName{Value: "fake-amf"},
		ServedGUAMIList: &ie.ServedGUAMIList{List: []ie.ServedGUAMIItem{{GUAMI: &ie.GUAMI{
			PLMNIdentity: &plmn,
			AMFRegionID:  &ie.AMFRegionID{Value: aper.BitString{Bytes: []byte{0xca}, BitLength: 8}},
			AMFSetID:     &ie.AMFSetID{Value: aper.BitString{Bytes: []byte{0xfe, 0x00}, BitLength: 10}},
			AMFPointer:   &ie.AMFPointer{Value: aper.BitString{Bytes: []byte{0x00}, BitLength: 6}},
		}}}},
		RelativeAMFCapacity: &ie.RelativeAMFCapacity{Value: 255},
		PLMNSupportList: &ie.PLMNSupportList{List: []ie.PLMNSupportItem{{
			PLMNIdentity:     &plmn,
			SliceSupportList: &ie.SliceSupportList{List: []ie.SliceSupportItem{{SNSSAI: &snssai}}},
		}}},
	}).MarshalBinary()
}

func initialContextSetupRequest(ue *amfUe, nas []byte) ([]byte, error) {
	plmn := ie.PLMNIdentity{Value: aper.OctetString{0x02, 0xf8, 0x39}}
	return (&message.InitialContextSetupRequest{
		AMFUENGAPID: &ie.AMFUENGAPID{Value: ue.amfID},
		RANUENGAPID: &ie.RANUENGAPID{Value: ue.ranID},
		GUAMI: &ie.GUAMI{
			PLMNIdentity: &plmn,
			AMFRegionID:  &ie.AMFRegionID{Value: aper.BitString{Bytes: []byte{0xca}, BitLength: 8}},
			AMFSetID:     &ie.AMFSetID{Value: aper.BitString{Bytes: []byte{0xfe, 0x00}, BitLength: 10}},
			AMFPointer:   &ie.AMFPointer{Value: aper.BitString{Bytes: []byte{0x00}, BitLength: 6}},
		},
		AllowedNSSAI: &ie.AllowedNSSAI{List: []ie.AllowedNSSAIItem{{SNSSAI: &ie.SNSSAI{SST: &ie.SST{Value: aper.OctetString{1}}}}}},
		UESecurityCapabilities: &ie.UESecurityCapabilities{
			NRencryptionAlgorithms:             &ie.NRencryptionAlgorithms{Value: aper.BitString{Bytes: []byte{0, 0}, BitLength: 16}},
			NRintegrityProtectionAlgorithms:    &ie.NRintegrityProtectionAlgorithms{Value: aper.BitString{Bytes: []byte{0, 0}, BitLength: 16}},
			EUTRAencryptionAlgorithms:          &ie.EUTRAencryptionAlgorithms{Value: aper.BitString{Bytes: []byte{0, 0}, BitLength: 16}},
			EUTRAintegrityProtectionAlgorithms: &ie.EUTRAintegrityProtectionAlgorithms{Value: aper.BitString{Bytes: []byte{0, 0}, BitLength: 16}},
		},
		SecurityKey: &ie.SecurityKey{Value: aper.BitString{Bytes: make([]byte, 32), BitLength: 256}},
		NASPDU:      &ie.NASPDU{Value: aper.OctetString(nas)},
	}).MarshalBinary()
}

func pduSessionResourceSetupRequest(ue *amfUe, nas []byte, upfIP netip.Addr) ([]byte, error) {
	ip := upfIP.As4()
	transfer := ie.PDUSessionResourceSetupRequestTransfer{
		ProtocolIEs: &ie.ProtocolIEContainerPDUSessionResourceSetupRequestTransferIEs{
			List: []ie.PDUSessionResourceSetupRequestTransferIEs{
				{ULNGUUPTNLInformation: &ie.UPTransportLayerInformation{Choice: &ie.GTPTunnel{
					TransportLayerAddress: &ie.TransportLayerAddress{Value: aper.BitString{Bytes: ip[:], BitLength: 32}},
					GTPTEID:               &ie.GTPTEID{Value: aper.OctetString{0, 0, 0x10, byte(ue.amfID)}},
				}}},
				{PDUSessionType: &ie.PDUSessionType{Value: ie.PDUSessionTypePresentIpv4}},
				{QosFlowSetupRequestList: &ie.QosFlowSetupRequestList{List: []ie.QosFlowSetupRequestItem{{
					QosFlowIdentifier: &ie.QosFlowIdentifier{Value: 9},
					QosFlowLevelQosParameters: &ie.QosFlowLevelQosParameters{
						QosCharacteristics: &ie.QosCharacteristics{Choice: &ie.NonDynamic5QIDescriptor{FiveQI: &ie.FiveQI{Value: 9}}},
						AllocationAndRetentionPriority: &ie.AllocationAndRetentionPriority{
							PriorityLevelARP:        &ie.PriorityLevelARP{Value: 8},
							PreEmptionCapability:    &ie.PreEmptionCapability{Value: ie.PreEmptionCapabilityPresentShallNotTriggerPreEmption},
							PreEmptionVulnerability: &ie.PreEmptionVulnerability{Value: ie.PreEmptionVulnerabilityPresentNotPreEmptable},
						},
					},
				}}}},
			},
		},
	}
	encoded, err := ie.MarshalBinary(&transfer)
	if err != nil {
		return nil, fmt.Errorf("fake amf: encode setup request transfer: %w", err)
	}
	octets := aper.OctetString(encoded)
	return (&message.PDUSessionResourceSetupRequest{
		AMFUENGAPID: &ie.AMFUENGAPID{Value: ue.amfID},
		RANUENGAPID: &ie.RANUENGAPID{Value: ue.ranID},
		PDUSessionResourceSetupListSUReq: &ie.PDUSessionResourceSetupListSUReq{List: []ie.PDUSessionResourceSetupItemSUReq{{
			PDUSessionID:                           &ie.PDUSessionID{Value: 1},
			PDUSessionNASPDU:                       &ie.NASPDU{Value: aper.OctetString(nas)},
			SNSSAI:                                 &ie.SNSSAI{SST: &ie.SST{Value: aper.OctetString{1}}},
			PDUSessionResourceSetupRequestTransfer: &octets,
		}}},
	}).MarshalBinary()
}
```

- [ ] **Step 4: Implement the gNB side.** `tester/gnb/ngapmsg.go`:

```go
package gnb

import (
	"encoding/binary"
	"fmt"
	"net/netip"

	"github.com/free5gc/ngap/aper"
	"github.com/free5gc/ngap/ie"
	"github.com/free5gc/ngap/message"
)

// UE-associated NGAP messages the gNB sends, copied from free-ran-ue
// gnb/ngapBuilder.go and trimmed to what the tester uses.

func (id Identity) userLocation() *ie.UserLocationInformation {
	plmn := id.PlmnID
	return &ie.UserLocationInformation{
		Choice: &ie.UserLocationInformationNR{
			NRCGI: &ie.NRCGI{
				PLMNIdentity:   &plmn,
				NRCellIdentity: &ie.NRCellIdentity{Value: nrCellIdentity(id.GnbID)},
			},
			TAI: &ie.TAI{PLMNIdentity: id.Tai.PLMNIdentity, TAC: id.Tai.TAC},
		},
	}
}

// nrCellIdentity is the 36-bit NCI: the gNB ID in the high bits, cell 1.
func nrCellIdentity(gnbID []byte) aper.BitString {
	const nciBits = 36
	var v uint64
	for _, b := range gnbID {
		v = v<<8 | uint64(b)
	}
	idBits := len(gnbID) * 8
	if idBits > nciBits {
		v >>= uint(idBits - nciBits)
		idBits = nciBits
	}
	nci := v
	if cellBits := nciBits - idBits; cellBits > 0 {
		nci = v<<uint(cellBits) | 1
	}
	packed := nci << 4
	return aper.BitString{
		Bytes:     []byte{byte(packed >> 32), byte(packed >> 24), byte(packed >> 16), byte(packed >> 8), byte(packed)},
		BitLength: nciBits,
	}
}

func (id Identity) initialUEMessage(ranUeID int64, nas []byte) ([]byte, error) {
	return (&message.InitialUEMessage{
		RANUENGAPID:             &ie.RANUENGAPID{Value: ranUeID},
		NASPDU:                  &ie.NASPDU{Value: aper.OctetString(nas)},
		UserLocationInformation: id.userLocation(),
		RRCEstablishmentCause:   &ie.RRCEstablishmentCause{Value: ie.RRCEstablishmentCausePresentMoSignalling},
		UEContextRequest:        &ie.UEContextRequest{Value: ie.UEContextRequestPresentRequested},
	}).MarshalBinary()
}

func (id Identity) uplinkNASTransport(amfUeID, ranUeID int64, nas []byte) ([]byte, error) {
	return (&message.UplinkNASTransport{
		AMFUENGAPID:             &ie.AMFUENGAPID{Value: amfUeID},
		RANUENGAPID:             &ie.RANUENGAPID{Value: ranUeID},
		NASPDU:                  &ie.NASPDU{Value: aper.OctetString(nas)},
		UserLocationInformation: id.userLocation(),
	}).MarshalBinary()
}

func initialContextSetupResponse(amfUeID, ranUeID int64) ([]byte, error) {
	return (&message.InitialContextSetupResponse{
		AMFUENGAPID: &ie.AMFUENGAPID{Value: amfUeID},
		RANUENGAPID: &ie.RANUENGAPID{Value: ranUeID},
	}).MarshalBinary()
}

func (id Identity) ueContextReleaseComplete(amfUeID, ranUeID int64) ([]byte, error) {
	return (&message.UEContextReleaseComplete{
		AMFUENGAPID:             &ie.AMFUENGAPID{Value: amfUeID},
		RANUENGAPID:             &ie.RANUENGAPID{Value: ranUeID},
		UserLocationInformation: id.userLocation(),
	}).MarshalBinary()
}

func pduSessionResourceSetupResponse(amfUeID, ranUeID, pduSessionID int64, dlTeid uint32, gnbN3IP netip.Addr, qfi int64) ([]byte, error) {
	teid := make([]byte, 4)
	binary.BigEndian.PutUint32(teid, dlTeid)
	ip := gnbN3IP.As4()
	transfer := ie.PDUSessionResourceSetupResponseTransfer{
		DLQosFlowPerTNLInformation: &ie.QosFlowPerTNLInformation{
			UPTransportLayerInformation: &ie.UPTransportLayerInformation{
				Choice: &ie.GTPTunnel{
					GTPTEID:               &ie.GTPTEID{Value: aper.OctetString(teid)},
					TransportLayerAddress: &ie.TransportLayerAddress{Value: aper.BitString{Bytes: ip[:], BitLength: 32}},
				},
			},
			AssociatedQosFlowList: &ie.AssociatedQosFlowList{
				List: []ie.AssociatedQosFlowItem{{QosFlowIdentifier: &ie.QosFlowIdentifier{Value: qfi}}},
			},
		},
	}
	encoded, err := ie.MarshalBinary(&transfer)
	if err != nil {
		return nil, fmt.Errorf("encode pdu session resource setup response transfer: %w", err)
	}
	octets := aper.OctetString(encoded)
	return (&message.PDUSessionResourceSetupResponse{
		AMFUENGAPID: &ie.AMFUENGAPID{Value: amfUeID},
		RANUENGAPID: &ie.RANUENGAPID{Value: ranUeID},
		PDUSessionResourceSetupListSURes: &ie.PDUSessionResourceSetupListSURes{
			List: []ie.PDUSessionResourceSetupItemSURes{{
				PDUSessionID:                            &ie.PDUSessionID{Value: pduSessionID},
				PDUSessionResourceSetupResponseTransfer: &octets,
			}},
		},
	}).MarshalBinary()
}
```

`tester/gnb/association.go`:

```go
package gnb

import (
	"errors"
	"fmt"
	"net/netip"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/free5gc/ngap/ie"
	"github.com/free5gc/ngap/message"
)

// DownlinkKind says what a Downlink carries.
type DownlinkKind int

const (
	DownlinkNas      DownlinkKind = iota // Nas is set; Pdu too if it came in a PDU Session Resource Setup
	DownlinkReleased                     // AMF released the UE context (e.g. after a reject)
	DownlinkLost                         // the association went down
)

// PduSetup is what the gNB learned and allocated in a PDU Session
// Resource Setup; the data plane (phase 3) needs all of it.
type PduSetup struct {
	UlTeid []byte     // UPF side
	UpfIP  netip.Addr // UPF N3 address from the transfer
	DlTeid uint32     // allocated by the tester
	Qfi    int64
}

// Downlink is one event for a UE, delivered in arrival order.
type Downlink struct {
	Kind DownlinkKind
	Nas  []byte
	Pdu  *PduSetup
	Err  error // for DownlinkLost
}

// UeLink is one UE's slot on an association. Downlinks are buffered; if a
// UE stops reading (its procedure timed out) further events are dropped.
type UeLink struct {
	RanUeID   int64
	amfUeID   atomic.Int64 // -1 until the AMF's first downlink
	Downlinks chan Downlink
}

// TeidAllocator hands out DL TEIDs unique across every gNB of a run.
type TeidAllocator struct{ next atomic.Uint32 }

func (t *TeidAllocator) Allocate() uint32 { return t.next.Add(1) }

// Association is one gNB's N2 after NG Setup: it multiplexes every UE of
// the gNB over the single SCTP association, standing in for the Uu link
// free-ran-ue needs between a UE process and a gNB process.
type Association struct {
	conn  Conn
	id    Identity
	n3IP  netip.Addr
	teids *TeidAllocator

	writeMu sync.Mutex
	nextID  atomic.Int64

	mu   sync.Mutex
	ues  map[int64]*UeLink
	lost error
}

func NewAssociation(conn Conn, id Identity, n3IP netip.Addr, teids *TeidAllocator) *Association {
	return &Association{conn: conn, id: id, n3IP: n3IP, teids: teids, ues: map[int64]*UeLink{}}
}

// Attach gives a UE a fresh RAN UE NGAP ID (a retry attaches again).
func (a *Association) Attach() (*UeLink, error) {
	l := &UeLink{RanUeID: a.nextID.Add(1), Downlinks: make(chan Downlink, 16)}
	l.amfUeID.Store(-1)
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.lost != nil {
		return nil, a.lost
	}
	a.ues[l.RanUeID] = l
	return l, nil
}

func (a *Association) Detach(l *UeLink) {
	a.mu.Lock()
	delete(a.ues, l.RanUeID)
	a.mu.Unlock()
}

func (a *Association) write(b []byte) error {
	a.writeMu.Lock()
	defer a.writeMu.Unlock()
	if _, err := a.conn.Write(b); err != nil {
		return fmt.Errorf("association lost: %w", err)
	}
	return nil
}

// SendInitialUE carries the UE's first NAS message (Registration Request).
func (a *Association) SendInitialUE(l *UeLink, nas []byte) error {
	b, err := a.id.initialUEMessage(l.RanUeID, nas)
	if err != nil {
		return fmt.Errorf("encode initial ue message: %w", err)
	}
	return a.write(b)
}

// SendUplinkNas carries a later NAS message; it needs the AMF UE NGAP ID,
// so it fails if the AMF has not sent anything for this UE yet.
func (a *Association) SendUplinkNas(l *UeLink, nas []byte) error {
	amfID := l.amfUeID.Load()
	if amfID < 0 {
		return errors.New("uplink nas before the amf assigned an amf ue ngap id")
	}
	b, err := a.id.uplinkNASTransport(amfID, l.RanUeID, nas)
	if err != nil {
		return fmt.Errorf("encode uplink nas transport: %w", err)
	}
	return a.write(b)
}

// Run reads NGAP until the association fails and returns that error.
// Every UE still attached then gets DownlinkLost. EAGAIN (SO_RCVTIMEO on
// an idle association) and EINTR are not failures.
func (a *Association) Run() error {
	buf := make([]byte, 65536)
	for {
		n, err := a.conn.Read(buf)
		if err != nil {
			if errors.Is(err, syscall.EAGAIN) || errors.Is(err, syscall.EINTR) {
				continue
			}
			a.fail(err)
			return err
		}
		a.dispatch(buf[:n])
	}
}

func (a *Association) fail(err error) {
	a.mu.Lock()
	a.lost = fmt.Errorf("association lost: %w", err)
	ues := a.ues
	a.ues = map[int64]*UeLink{}
	a.mu.Unlock()
	for _, l := range ues {
		deliver(l, Downlink{Kind: DownlinkLost, Err: err})
	}
}

func (a *Association) link(ranUeID, amfUeID int64) *UeLink {
	a.mu.Lock()
	l := a.ues[ranUeID]
	a.mu.Unlock()
	if l != nil && amfUeID >= 0 {
		l.amfUeID.CompareAndSwap(-1, amfUeID)
	}
	return l
}

func deliver(l *UeLink, d Downlink) {
	select {
	case l.Downlinks <- d:
	default: // the UE's procedure gave up and nobody is reading
	}
}

func (a *Association) dispatch(raw []byte) {
	msg, err := message.Parse(raw)
	if err != nil {
		return // not ours to fix; the AMF will time the UE out
	}
	switch m := msg.(type) {
	case *message.DownlinkNASTransport:
		if m.RANUENGAPID == nil || m.AMFUENGAPID == nil || m.NASPDU == nil {
			return
		}
		if l := a.link(m.RANUENGAPID.Value, m.AMFUENGAPID.Value); l != nil {
			deliver(l, Downlink{Kind: DownlinkNas, Nas: append([]byte(nil), m.NASPDU.Value...)})
		}
	case *message.InitialContextSetupRequest:
		if m.RANUENGAPID == nil || m.AMFUENGAPID == nil {
			return
		}
		l := a.link(m.RANUENGAPID.Value, m.AMFUENGAPID.Value)
		if l == nil {
			return
		}
		if rsp, err := initialContextSetupResponse(m.AMFUENGAPID.Value, m.RANUENGAPID.Value); err == nil {
			_ = a.write(rsp)
		}
		if m.NASPDU != nil {
			deliver(l, Downlink{Kind: DownlinkNas, Nas: append([]byte(nil), m.NASPDU.Value...)})
		}
	case *message.PDUSessionResourceSetupRequest:
		a.onPduSessionResourceSetup(m)
	case *message.UEContextReleaseCommand:
		if m.UENGAPIDs == nil {
			return
		}
		pair, ok := m.UENGAPIDs.Choice.(*ie.UENGAPIDPair)
		if !ok || pair.AMFUENGAPID == nil || pair.RANUENGAPID == nil {
			return
		}
		if rsp, err := a.id.ueContextReleaseComplete(pair.AMFUENGAPID.Value, pair.RANUENGAPID.Value); err == nil {
			_ = a.write(rsp)
		}
		if l := a.link(pair.RANUENGAPID.Value, -1); l != nil {
			deliver(l, Downlink{Kind: DownlinkReleased})
		}
	}
}

func (a *Association) onPduSessionResourceSetup(m *message.PDUSessionResourceSetupRequest) {
	if m.RANUENGAPID == nil || m.AMFUENGAPID == nil || m.PDUSessionResourceSetupListSUReq == nil {
		return
	}
	l := a.link(m.RANUENGAPID.Value, m.AMFUENGAPID.Value)
	if l == nil {
		return
	}
	for _, item := range m.PDUSessionResourceSetupListSUReq.List {
		if item.PDUSessionID == nil {
			continue
		}
		setup := &PduSetup{Qfi: 1, DlTeid: a.teids.Allocate()}
		if item.PDUSessionResourceSetupRequestTransfer != nil {
			var transfer ie.PDUSessionResourceSetupRequestTransfer
			if err := ie.UnmarshalBinary(*item.PDUSessionResourceSetupRequestTransfer, &transfer); err == nil && transfer.ProtocolIEs != nil {
				for _, f := range transfer.ProtocolIEs.List {
					if f.ULNGUUPTNLInformation != nil {
						if t, ok := f.ULNGUUPTNLInformation.Choice.(*ie.GTPTunnel); ok {
							if t.GTPTEID != nil {
								setup.UlTeid = append([]byte(nil), t.GTPTEID.Value...)
							}
							if t.TransportLayerAddress != nil && len(t.TransportLayerAddress.Value.Bytes) >= 4 {
								setup.UpfIP, _ = netip.AddrFromSlice(t.TransportLayerAddress.Value.Bytes[:4])
							}
						}
					}
					if f.QosFlowSetupRequestList != nil && len(f.QosFlowSetupRequestList.List) > 0 &&
						f.QosFlowSetupRequestList.List[0].QosFlowIdentifier != nil {
						setup.Qfi = f.QosFlowSetupRequestList.List[0].QosFlowIdentifier.Value
					}
				}
			}
		}
		if rsp, err := pduSessionResourceSetupResponse(m.AMFUENGAPID.Value, m.RANUENGAPID.Value,
			item.PDUSessionID.Value, setup.DlTeid, a.n3IP, setup.Qfi); err == nil {
			_ = a.write(rsp)
		}
		var nas []byte
		if item.PDUSessionNASPDU != nil {
			nas = append([]byte(nil), item.PDUSessionNASPDU.Value...)
		}
		deliver(l, Downlink{Kind: DownlinkNas, Nas: nas, Pdu: setup})
	}
}
```

- [ ] **Step 5: Implement the procedures** `tester/procedure/procedure.go`:

```go
// Package procedure drives one UE through registration and PDU session
// establishment over its gNB's association, and reports how it ended in
// the terms the stage cards count (metrics.Outcome + cause).
package procedure

import (
	"errors"
	"fmt"
	"net/netip"
	"time"

	"tester/gnb"
	"tester/metrics"
	"tester/ue"
)

// Outcome is how one attempt ended.
type Outcome struct {
	Result metrics.Outcome
	Cause  string        // empty when Accepted
	UeIP   netip.Addr    // PDU only
	Pdu    *gnb.PduSetup // PDU only; nil if the accept came without a resource setup
}

var errTimeout = errors.New("timeout")

// Register runs one initial registration attempt. On success the returned
// link stays attached for the PDU session; on failure it is detached.
// Timing: from sending Initial UE Message to sending Registration Complete.
func Register(assoc *gnb.Association, u *ue.UE, timeout time.Duration) (*gnb.UeLink, Outcome) {
	link, err := assoc.Attach()
	if err != nil {
		return nil, Outcome{Result: metrics.Failed, Cause: err.Error()}
	}
	out := register(assoc, link, u, time.Now().Add(timeout))
	if out.Result != metrics.Accepted {
		assoc.Detach(link)
		return nil, out
	}
	return link, out
}

func register(assoc *gnb.Association, link *gnb.UeLink, u *ue.UE, deadline time.Time) Outcome {
	req, err := u.RegistrationRequest()
	if err != nil {
		return failed(fmt.Errorf("encode registration request: %w", err))
	}
	if err := assoc.SendInitialUE(link, req); err != nil {
		return failed(err)
	}
	for {
		d, err := next(link, deadline)
		if err != nil {
			return failed(err)
		}
		switch d.Kind {
		case gnb.DownlinkLost:
			return failed(fmt.Errorf("association lost: %w", d.Err))
		case gnb.DownlinkReleased:
			return Outcome{Result: metrics.Rejected, Cause: "ue context released"}
		}
		r, err := u.Handle(d.Nas)
		if err != nil {
			return failed(err)
		}
		if r.Reply != nil {
			if err := assoc.SendUplinkNas(link, r.Reply); err != nil {
				return failed(err)
			}
		}
		switch r.Event {
		case ue.EventRegistered:
			return Outcome{Result: metrics.Accepted}
		case ue.EventRegistrationRejected:
			return Outcome{Result: metrics.Rejected, Cause: r.Cause}
		}
	}
}

// EstablishPdu runs one PDU session attempt for a registered UE.
// Timing: from sending the request to receiving the accept, by which point
// the gNB has already answered the PDU Session Resource Setup.
func EstablishPdu(assoc *gnb.Association, link *gnb.UeLink, u *ue.UE, timeout time.Duration) Outcome {
	deadline := time.Now().Add(timeout)
	req, err := u.PduSessionRequest()
	if err != nil {
		return failed(err)
	}
	if err := assoc.SendUplinkNas(link, req); err != nil {
		return failed(err)
	}
	var setup *gnb.PduSetup
	for {
		d, err := next(link, deadline)
		if err != nil {
			return failed(err)
		}
		switch d.Kind {
		case gnb.DownlinkLost:
			return failed(fmt.Errorf("association lost: %w", d.Err))
		case gnb.DownlinkReleased:
			return Outcome{Result: metrics.Rejected, Cause: "ue context released"}
		}
		if d.Pdu != nil {
			setup = d.Pdu
		}
		if d.Nas == nil {
			continue
		}
		r, err := u.Handle(d.Nas)
		if err != nil {
			return failed(err)
		}
		switch r.Event {
		case ue.EventPduEstablished:
			return Outcome{Result: metrics.Accepted, UeIP: r.UeIP, Pdu: setup}
		case ue.EventPduRejected:
			return Outcome{Result: metrics.Rejected, Cause: r.Cause}
		}
	}
}

func next(link *gnb.UeLink, deadline time.Time) (gnb.Downlink, error) {
	t := time.NewTimer(time.Until(deadline))
	defer t.Stop()
	select {
	case d := <-link.Downlinks:
		return d, nil
	case <-t.C:
		return gnb.Downlink{}, errTimeout
	}
}

func failed(err error) Outcome {
	if errors.Is(err, errTimeout) {
		return Outcome{Result: metrics.TimedOut, Cause: "timeout"}
	}
	return Outcome{Result: metrics.Failed, Cause: err.Error()}
}
```

- [ ] **Step 6: Run the tests repeatedly with the race detector.** Run: `cd tester && go test -race -count=5 ./procedure/ ./gnb/`. Expected: `ok` for both.
- [ ] **Step 7: Commit** `feat: per-gnb ngap association and ue procedures`.

---

### Task 5: Paced pipeline stage

**Files:** Create `tester/run/pipeline.go`, `tester/run/pipeline_test.go`

**Interfaces:**
- Produces: `newProcStage(profile.ProcedureRate, ueCount int, attempt func(ue int) (retry bool)) *procStage`, with methods `enqueue(int)` (never blocks), `run(ctx)` (returns after in-flight attempts finish) and `drain() []int`.

- [ ] **Step 1: Write the failing test** `tester/run/pipeline_test.go`:

```go
package run

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"tester/profile"
)

func runStage(t *testing.T, p *procStage, n int) (cancel func()) {
	t.Helper()
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { p.run(ctx); close(done) }()
	for i := range n {
		p.enqueue(i)
	}
	return func() { stop(); <-done }
}

func TestProcStageHonoursRate(t *testing.T) {
	var mu sync.Mutex
	var starts []time.Time
	var wg sync.WaitGroup
	wg.Add(10)
	p := newProcStage(profile.ProcedureRate{RatePerSec: 50, MaxInFlight: 100}, 10, func(int) bool {
		mu.Lock()
		starts = append(starts, time.Now())
		mu.Unlock()
		wg.Done()
		return false
	})
	stop := runStage(t, p, 10)
	defer stop()
	wg.Wait()
	elapsed := starts[9].Sub(starts[0])
	require.GreaterOrEqual(t, elapsed, 170*time.Millisecond, "10 starts at 50/s span ~180ms")
	require.Less(t, elapsed, 400*time.Millisecond)
}

func TestProcStageCapsInFlight(t *testing.T) {
	var cur, peak atomic.Int32
	var wg sync.WaitGroup
	wg.Add(8)
	p := newProcStage(profile.ProcedureRate{RatePerSec: 100000, MaxInFlight: 2}, 8, func(int) bool {
		n := cur.Add(1)
		for {
			old := peak.Load()
			if n <= old || peak.CompareAndSwap(old, n) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		cur.Add(-1)
		wg.Done()
		return false
	})
	stop := runStage(t, p, 8)
	defer stop()
	wg.Wait()
	require.Equal(t, int32(2), peak.Load())
}

func TestProcStageRequeuesRetriesAtTheBack(t *testing.T) {
	var mu sync.Mutex
	var order []int
	tries := map[int]int{}
	var wg sync.WaitGroup
	wg.Add(6)
	p := newProcStage(profile.ProcedureRate{RatePerSec: 100000, MaxInFlight: 1}, 3, func(ue int) bool {
		mu.Lock()
		defer mu.Unlock()
		order = append(order, ue)
		tries[ue]++
		wg.Done()
		return tries[ue] == 1 // every UE fails once
	})
	stop := runStage(t, p, 3)
	defer stop()
	wg.Wait()
	require.Equal(t, []int{0, 1, 2, 0, 1, 2}, order)
}

func TestProcStageStopLeavesQueuedUesForDrain(t *testing.T) {
	started := make(chan int, 10)
	release := make(chan struct{})
	p := newProcStage(profile.ProcedureRate{RatePerSec: 100000, MaxInFlight: 1}, 5, func(ue int) bool {
		started <- ue
		<-release
		return false
	})
	stop := runStage(t, p, 5)
	<-started // UE 0 is in flight, 1-4 wait for the single slot
	go func() { time.Sleep(20 * time.Millisecond); close(release) }()
	stop() // returns only after UE 0's attempt finished
	require.ElementsMatch(t, []int{1, 2, 3, 4}, p.drain())
}
```

- [ ] **Step 2: Run it to make sure it fails.** Run: `cd tester && go test ./run/ -run ProcStage`. Expected: FAIL (`undefined: newProcStage`).
- [ ] **Step 3: Implement** `tester/run/pipeline.go`:

```go
package run

import (
	"context"
	"sync"
	"time"

	"tester/profile"
)

// procStage paces one per-UE stage (registration or PDU): it starts at
// most RatePerSec attempts per second and keeps at most MaxInFlight
// running at once. Separate knobs, because a semaphore alone lets a fast
// core run far above the configured rate, and a rate alone lets a slow
// core pile up unbounded in-flight requests (design section 5).
type procStage struct {
	interval    time.Duration
	maxInFlight int
	queue       chan int // UE indexes; each UE is queued at most once at a time
	attempt     func(ue int) (retry bool)
	inFlight    sync.WaitGroup
}

func newProcStage(r profile.ProcedureRate, ueCount int, attempt func(ue int) (retry bool)) *procStage {
	return &procStage{
		interval:    time.Second / time.Duration(r.RatePerSec),
		maxInFlight: r.MaxInFlight,
		queue:       make(chan int, ueCount),
		attempt:     attempt,
	}
}

// enqueue never blocks: the queue holds every UE at once.
func (p *procStage) enqueue(ue int) { p.queue <- ue }

// run dispatches until ctx is done, then waits for in-flight attempts.
// A failed attempt the callback wants retried goes to the back of the
// queue (design N3). Whatever is still queued afterwards is returned by
// drain.
func (p *procStage) run(ctx context.Context) {
	sem := make(chan struct{}, p.maxInFlight)
	next := time.Now()
	for {
		var ue int
		select {
		case <-ctx.Done():
			p.inFlight.Wait()
			return
		case ue = <-p.queue:
		}
		// token bucket of depth 1: one start every interval
		if wait := time.Until(next); wait > 0 {
			t := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				t.Stop()
				p.queue <- ue
				p.inFlight.Wait()
				return
			case <-t.C:
			}
		}
		next = maxTime(next, time.Now()).Add(p.interval)
		select {
		case <-ctx.Done():
			p.queue <- ue
			p.inFlight.Wait()
			return
		case sem <- struct{}{}:
		}
		p.inFlight.Add(1)
		go func() {
			defer func() { <-sem; p.inFlight.Done() }()
			// requeue a wanted retry; if we are stopping, drain finds it
			if p.attempt(ue) {
				p.queue <- ue
			}
		}()
	}
}

// drain returns the UEs still queued; call after run has returned.
func (p *procStage) drain() []int {
	var out []int
	for {
		select {
		case ue := <-p.queue:
			out = append(out, ue)
		default:
			return out
		}
	}
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
```

- [ ] **Step 4: Run the tests.** Run: `cd tester && go test -race -count=3 -run ProcStage ./run/`. Expected: `ok`.
- [ ] **Step 5: Commit** `feat: paced procedure stage with in-flight cap and requeue`.

---

### Task 6: Run controller — N2 → registration → PDU

**Files:**
- Modify: `tester/run/snapshot.go`, `tester/run/controller.go`, `tester/run/controller_test.go`
- Create: `tester/run/pipeline_e2e_test.go`

**Interfaces:**
- Consumes: Tasks 1–5.
- Produces:
  - New snapshot fields: `Snapshot.Registration`, `Snapshot.Pdu`, `Snapshot.Ues UeSummary` and `Snapshot.FailedUes []UeFailure` (at most 200).
  - `GnbStatus.Registered` and `GnbStatus.Established`.
  - `UeState` constants `pending|registering|registered|establishing|established|failed|skipped|cancelled`.
  - `UeFailure{Supi, Gnb, Stage, Cause string; Attempts int}`.
  - `StateRunning` now covers both the UE stages and the holding phase.
  - Phase 1's `watchConn` is replaced by `watchAssociation`, which runs `Association.Run`.

- [ ] **Step 1: Write the failing tests.** Replace `tester/run/controller_test.go` with:

```go
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
		Ue: profile.UeTemplate{MsinStart: "0000000001", Key: "8baf473f2f8fd09487cccbd7097c6862",
			Opc: "8e27b6af0e692e750f32667a3b14605d", Amf: "8000", Sqn: "000000000023",
			Integrity: "nia2", Ciphering: "nea0", Dnn: "internet", Sst: 1, Sd: "010203"},
		Network: profile.Network{
			N2: profile.N2Network{Interface: "eth-n2", Cidr: "10.0.1.0/24", StartIP: "10.0.1.10", AmfIP: "10.0.1.1", AmfPort: 38412},
			N3: profile.N3Network{Interface: "eth-n3", Cidr: "10.0.2.0/24", StartIP: "10.0.2.10", UpfIP: "10.0.2.1", UpfPort: 2152},
		},
		// N2-only tests use fakeConn, which never answers NAS: keep the UE
		// stages short so Stop does not wait long for their timeouts.
		Rates: profile.Rates{
			N2:           profile.StageRate{TimeoutMs: 1000, Retries: 1},
			Registration: profile.ProcedureRate{RatePerSec: 1000, MaxInFlight: 100, TimeoutMs: 100, Retries: 0},
			Pdu:          profile.ProcedureRate{RatePerSec: 1000, MaxInFlight: 100, TimeoutMs: 100, Retries: 0},
		},
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
```

Create `tester/run/pipeline_e2e_test.go`. It pins Review Focus #1 and #2.

```go
package run

import (
	"errors"
	"net/netip"
	"testing"
	"time"

	loggergo "github.com/Alonza0314/logger-go/v2"
	"github.com/stretchr/testify/require"

	"tester/gnb"
	"tester/internal/fakecore"
	"tester/metrics"
	"tester/profile"
)

// amfDialer connects every gNB to the same fake AMF over a fresh pipe;
// IPs listed in refuse fail to connect.
type amfDialer struct {
	amf    *fakecore.AMF
	refuse map[string]bool
}

func (d amfDialer) Dial(localIP, _ string, _ int, _ time.Duration) (gnb.Conn, error) {
	if d.refuse[localIP] {
		return nil, errors.New("connection refused")
	}
	g, a := fakecore.Pipe()
	go func() { _ = d.amf.Serve(a) }()
	return g, nil
}

func newFakeAMF(behavior func(supi string) fakecore.Behavior) *fakecore.AMF {
	return &fakecore.AMF{
		Subscriber: fakecore.Subscriber{Mcc: "208", Mnc: "93",
			Key: "8baf473f2f8fd09487cccbd7097c6862", Opc: "8e27b6af0e692e750f32667a3b14605d",
			Amf: "8000", Sqn: "000000000023"},
		Behavior: behavior,
		UpfIP:    netip.MustParseAddr("10.0.1.5"),
	}
}

func e2eProfile() profile.Profile {
	p := testProfile()
	p.Rates.Registration = profile.ProcedureRate{RatePerSec: 1000, MaxInFlight: 100, TimeoutMs: 2000, Retries: 1}
	p.Rates.Pdu = profile.ProcedureRate{RatePerSec: 1000, MaxInFlight: 100, TimeoutMs: 2000, Retries: 1}
	return p
}

func newE2EController(d amfDialer) (*Controller, *fakeAddrs) {
	lg := loggergo.NewLogger("", true)
	lg.SetLevel("error")
	addrs := &fakeAddrs{}
	return NewController(Deps{Addrs: addrs, Dialer: d, Log: lg.WithTags("TEST"), NewID: func() string { return "e2e" }}), addrs
}

func stopAndWait(t *testing.T, c *Controller) Snapshot {
	t.Helper()
	_, err := c.Stop()
	require.NoError(t, err)
	return waitState(t, c, StateStopped)
}

func TestRunRegistersAndEstablishesEveryUe(t *testing.T) {
	amf := newFakeAMF(nil)
	c, addrs := newE2EController(amfDialer{amf: amf})
	_, err := c.Start(e2eProfile())
	require.NoError(t, err)

	snap := waitFor(t, c, "10 UEs established", func(s Snapshot) bool { return s.Ues.Established == 10 })
	require.Equal(t, StateRunning, snap.State)
	for _, st := range []metrics.StageSnapshot{snap.Registration, snap.Pdu} {
		require.Equal(t, int64(10), st.Accepted, st.Name)
		require.True(t, st.Done, st.Name)
		require.Greater(t, st.AvgMs, 0.0, st.Name)
	}
	require.Equal(t, []int{4, 4, 2}, []int{snap.Gnbs[0].Established, snap.Gnbs[1].Established, snap.Gnbs[2].Established})
	require.Equal(t, []int{4, 4, 2}, []int{snap.Gnbs[0].Registered, snap.Gnbs[1].Registered, snap.Gnbs[2].Registered})
	require.Empty(t, snap.FailedUes)
	require.Eventually(t, func() bool { return len(amf.SetupResponses()) == 10 }, time.Second, 5*time.Millisecond)

	snap = stopAndWait(t, c)
	require.Equal(t, UeSummary{Established: 10}, snap.Ues)
	present, _ := addrs.snapshot()
	require.Empty(t, present)
}

func TestRegistrationRejectIsRetriedThenListed(t *testing.T) {
	amf := newFakeAMF(func(supi string) fakecore.Behavior {
		if supi == "imsi-208930000000003" {
			return fakecore.Behavior{RejectRegistration: 7}
		}
		return fakecore.Behavior{}
	})
	c, _ := newE2EController(amfDialer{amf: amf})
	_, err := c.Start(e2eProfile())
	require.NoError(t, err)

	snap := waitFor(t, c, "pdu stage done", func(s Snapshot) bool { return s.Pdu.Done })
	require.Equal(t, int64(9), snap.Registration.Accepted)
	require.Equal(t, int64(1), snap.Registration.Rejected)
	require.Equal(t, int64(1), snap.Registration.Retries)
	require.Equal(t, []metrics.CauseCount{{Cause: "5gmm(7)", Count: 1}}, snap.Registration.Causes)
	require.Equal(t, int64(1), snap.Pdu.Skipped, "a UE that never registered skips the PDU stage")
	require.Equal(t, []UeFailure{{Supi: "imsi-208930000000003", Gnb: "gNB-1", Stage: "registration", Cause: "5gmm(7)", Attempts: 2}}, snap.FailedUes)
	require.Equal(t, UeSummary{Established: 9, Failed: 1}, snap.Ues)
	stopAndWait(t, c)
}

func TestPduRejectIsListed(t *testing.T) {
	amf := newFakeAMF(func(supi string) fakecore.Behavior {
		if supi == "imsi-208930000000010" {
			return fakecore.Behavior{RejectPdu: 27}
		}
		return fakecore.Behavior{}
	})
	c, _ := newE2EController(amfDialer{amf: amf})
	p := e2eProfile()
	p.Rates.Pdu.Retries = 0
	_, err := c.Start(p)
	require.NoError(t, err)
	snap := waitFor(t, c, "pdu stage done", func(s Snapshot) bool { return s.Pdu.Done })
	require.Equal(t, []UeFailure{{Supi: "imsi-208930000000010", Gnb: "gNB-3", Stage: "pdu", Cause: "5gsm(27)", Attempts: 1}}, snap.FailedUes)
	require.Equal(t, 1, snap.Gnbs[2].Established)
	require.Equal(t, 2, snap.Gnbs[2].Registered)
	stopAndWait(t, c)
}

func TestUesOfFailedGnbAreSkipped(t *testing.T) {
	c, _ := newE2EController(amfDialer{amf: newFakeAMF(nil), refuse: map[string]bool{"10.0.1.12": true}})
	_, err := c.Start(e2eProfile())
	require.NoError(t, err)
	snap := waitFor(t, c, "pdu stage done", func(s Snapshot) bool { return s.Pdu.Done })
	require.Equal(t, GnbFailed, snap.Gnbs[2].State)
	require.Equal(t, int64(2), snap.Registration.Skipped)
	require.Equal(t, int64(8), snap.Registration.Accepted)
	require.Equal(t, UeSummary{Established: 8, Skipped: 2}, snap.Ues)
	stopAndWait(t, c)
}

func TestStopDuringRegistrationCancelsQueuedUes(t *testing.T) {
	c, _ := newE2EController(amfDialer{amf: newFakeAMF(nil)})
	p := e2eProfile()
	p.Rates.Registration.RatePerSec = 5 // 10 UEs would take ~2 s
	_, err := c.Start(p)
	require.NoError(t, err)
	waitFor(t, c, "one UE established", func(s Snapshot) bool { return s.Ues.Established >= 1 })

	snap := stopAndWait(t, c)
	require.True(t, snap.Registration.Done, "stopped stage must be complete")
	require.True(t, snap.Pdu.Done)
	require.Positive(t, snap.Ues.Cancelled)
	require.Equal(t, int64(snap.Ues.Cancelled), snap.Registration.Skipped)
	require.Equal(t, 10, snap.Ues.Established+snap.Ues.Cancelled)
	frozen := snap.Registration.TotalTimeMs
	time.Sleep(50 * time.Millisecond)
	require.Equal(t, frozen, c.Snapshot().Registration.TotalTimeMs, "total time stops at the last finish")
}
```

- [ ] **Step 2: Run them to make sure they fail.** Run: `cd tester && go test ./run/`. Expected: FAIL to build (`snap.Ues undefined`, `UeSummary`, `UeFailure`, …).

- [ ] **Step 3: Implement.** Replace `tester/run/snapshot.go`:

```go
// Package run owns the lifecycle of the single active test run: configure
// gNB IPs, bring up N2 for every gNB, hold the associations until Stop,
// then tear everything down. One run at a time (design Q17).
package run

import (
	"time"

	"tester/metrics"
	"tester/profile"
)

type State string

const (
	StateIdle        State = "idle"        // no run since the process started
	StateConfiguring State = "configuring" // adding gNB IPs to host interfaces
	StateN2          State = "n2"          // SCTP + NG Setup in progress
	StateRunning     State = "running"     // N2 finished; UEs register / establish PDU sessions, then hold
	StateStopping    State = "stopping"    // closing associations, removing IPs
	StateStopped     State = "stopped"
	StateFailed      State = "failed" // could not configure the host; nothing was attempted
)

// Finished reports whether a new run may start.
func (s State) Finished() bool {
	return s == StateIdle || s == StateStopped || s == StateFailed
}

type GnbState string

const (
	GnbPending    GnbState = "pending"
	GnbConnecting GnbState = "connecting"
	GnbUp         GnbState = "up"
	GnbFailed     GnbState = "failed"
	GnbLost       GnbState = "lost" // was up, then the AMF side dropped it
	GnbClosed     GnbState = "closed"
)

// GnbStatus is one row of the per-gNB table.
type GnbStatus struct {
	profile.GnbSpec
	State       GnbState `json:"state"`
	Attempts    int      `json:"attempts"`
	LatencyMs   float64  `json:"latencyMs"`
	Cause       string   `json:"cause"`
	Registered  int      `json:"registered"`  // UEs of this gNB that completed registration
	Established int      `json:"established"` // UEs of this gNB with a PDU session
}

// UeState is where one UE is in the pipeline.
type UeState string

const (
	UePending      UeState = "pending" // waiting for its gNB or a registration slot
	UeRegistering  UeState = "registering"
	UeRegistered   UeState = "registered" // waiting for a PDU slot
	UeEstablishing UeState = "establishing"
	UeEstablished  UeState = "established"
	UeFailed       UeState = "failed"    // registration or PDU failed for good
	UeSkipped      UeState = "skipped"   // its gNB never came up
	UeCancelled    UeState = "cancelled" // the run was stopped before it finished
)

// UeSummary counts UEs per state; the fields add up to the UE count.
type UeSummary struct {
	Pending      int `json:"pending"`
	Registering  int `json:"registering"`
	Registered   int `json:"registered"`
	Establishing int `json:"establishing"`
	Established  int `json:"established"`
	Failed       int `json:"failed"`
	Skipped      int `json:"skipped"`
	Cancelled    int `json:"cancelled"`
}

func (s *UeSummary) add(st UeState, d int) {
	switch st {
	case UePending:
		s.Pending += d
	case UeRegistering:
		s.Registering += d
	case UeRegistered:
		s.Registered += d
	case UeEstablishing:
		s.Establishing += d
	case UeEstablished:
		s.Established += d
	case UeFailed:
		s.Failed += d
	case UeSkipped:
		s.Skipped += d
	case UeCancelled:
		s.Cancelled += d
	}
}

// UeFailure is one row of the failed-UE list.
type UeFailure struct {
	Supi     string `json:"supi"`
	Gnb      string `json:"gnb"`
	Stage    string `json:"stage"` // "registration" or "pdu"
	Cause    string `json:"cause"`
	Attempts int    `json:"attempts"`
}

// maxFailuresListed bounds the snapshot; UeSummary.Failed has the total.
const maxFailuresListed = 200

// Snapshot is everything the run page shows; it is what GET /api/run and
// every WebSocket frame carry.
type Snapshot struct {
	RunID        string                `json:"runId"`
	ProfileName  string                `json:"profileName"`
	State        State                 `json:"state"`
	Error        string                `json:"error"`
	StartedAt    *time.Time            `json:"startedAt"`
	StoppedAt    *time.Time            `json:"stoppedAt"`
	N2           metrics.StageSnapshot `json:"n2"`
	Registration metrics.StageSnapshot `json:"registration"`
	Pdu          metrics.StageSnapshot `json:"pdu"`
	Gnbs         []GnbStatus           `json:"gnbs"`
	Ues          UeSummary             `json:"ues"`
	FailedUes    []UeFailure           `json:"failedUes"` // first maxFailuresListed failures
}
```

Replace `tester/run/controller.go`:

```go
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
```

- [ ] **Step 4: Run the whole module.** Run: `cd tester && go vet ./... && go test -race -count=3 ./... && golangci-lint run ./...`. Expected: all `ok`, `0 issues.`
- [ ] **Step 5: Commit** `feat: register ues and establish pdu sessions after n2`.

---

### Task 7: Throttle the snapshot stream

**Files:** Modify `tester/api/api.go`, `tester/api/api_test.go`

- [ ] **Step 1: Write the failing test.** Append to `tester/api/api_test.go`:

```go
func TestStreamCoalescesBurstsOfChanges(t *testing.T) {
	ctrl := &fakeCtrl{snap: run.Snapshot{State: run.StateRunning}, changed: make(chan struct{})}
	close(ctrl.changed) // every frame sees "changed" at once, like a busy run
	srv := httptest.NewServer(NewRouter(ctrl, "t"))
	defer srv.Close()
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/run/stream"
	conn, _, err := websocket.DefaultDialer.Dial(url, http.Header{"Authorization": {"Bearer t"}})
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	var got run.Snapshot
	start := time.Now()
	for range 4 {
		require.NoError(t, conn.ReadJSON(&got))
	}
	require.GreaterOrEqual(t, time.Since(start), 3*minFrameGap-20*time.Millisecond,
		"4 frames need at least 3 gaps")
}
```

- [ ] **Step 2: Run it to make sure it fails.** Run: `cd tester && go test ./api/ -run Coalesces`. Expected: FAIL to build (`undefined: minFrameGap`).
- [ ] **Step 3: Implement.** Replace `tester/api/api.go`:

```go
// Package api is fru-tester's HTTP surface. fru-lab's backend is the only
// intended client: it authenticates users and forwards requests here with
// the shared API token.
package api

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"tester/profile"
	"tester/run"
)

// Controller is the slice of *run.Controller the handlers use.
type Controller interface {
	Validate(p profile.Profile) (*profile.Plan, error)
	Start(p profile.Profile) (run.Snapshot, error)
	Stop() (run.Snapshot, error)
	Snapshot() run.Snapshot
	Changed() <-chan struct{}
}

type MessageResponse struct {
	Message string `json:"message"`
}

// ValidateResponse is returned for both valid and invalid profiles, so the
// setup page can render the plan or the field errors from one call.
type ValidateResponse struct {
	Valid  bool                 `json:"valid"`
	Errors []profile.FieldError `json:"errors"`
	Plan   *profile.Plan        `json:"plan"`
}

// StartErrorResponse carries field errors when Start rejects the profile.
type StartErrorResponse struct {
	Message string               `json:"message"`
	Errors  []profile.FieldError `json:"errors"`
}

// streamInterval is how often the stream re-sends a snapshot even when no
// state changed, so totals and in-flight timers keep moving.
// minFrameGap bounds the frame rate: with thousands of UEs every step is a
// state change, and a full snapshot per change would flood the browser.
const (
	streamInterval = time.Second
	minFrameGap    = 200 * time.Millisecond
)

func NewRouter(ctrl Controller, apiToken string) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	g := r.Group("/api", requireToken(apiToken))
	g.POST("/profile/validate", handleValidate(ctrl))
	g.GET("/run", func(c *gin.Context) { c.JSON(http.StatusOK, ctrl.Snapshot()) })
	g.POST("/run", handleStart(ctrl))
	g.POST("/run/stop", handleStop(ctrl))
	g.GET("/run/stream", handleStream(ctrl))
	return r
}

func requireToken(token string) gin.HandlerFunc {
	want := []byte("Bearer " + token)
	return func(c *gin.Context) {
		got := []byte(c.GetHeader("Authorization"))
		if subtle.ConstantTimeCompare(got, want) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, MessageResponse{Message: "missing or wrong API token"})
			return
		}
		c.Next()
	}
}

func bindProfile(c *gin.Context) (profile.Profile, bool) {
	var p profile.Profile
	dec := json.NewDecoder(c.Request.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		c.JSON(http.StatusBadRequest, MessageResponse{Message: "invalid profile JSON: " + err.Error()})
		return p, false
	}
	return p, true
}

func handleValidate(ctrl Controller) gin.HandlerFunc {
	return func(c *gin.Context) {
		p, ok := bindProfile(c)
		if !ok {
			return
		}
		plan, err := ctrl.Validate(p)
		var verr *profile.ValidationError
		switch {
		case err == nil:
			c.JSON(http.StatusOK, ValidateResponse{Valid: true, Errors: []profile.FieldError{}, Plan: plan})
		case errors.As(err, &verr):
			c.JSON(http.StatusOK, ValidateResponse{Valid: false, Errors: verr.Errors})
		default:
			c.JSON(http.StatusInternalServerError, MessageResponse{Message: err.Error()})
		}
	}
}

func handleStart(ctrl Controller) gin.HandlerFunc {
	return func(c *gin.Context) {
		p, ok := bindProfile(c)
		if !ok {
			return
		}
		snap, err := ctrl.Start(p)
		var verr *profile.ValidationError
		switch {
		case err == nil:
			c.JSON(http.StatusAccepted, snap)
		case errors.As(err, &verr):
			c.JSON(http.StatusBadRequest, StartErrorResponse{Message: "profile is invalid", Errors: verr.Errors})
		case errors.Is(err, run.ErrRunActive):
			c.JSON(http.StatusConflict, MessageResponse{Message: err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, MessageResponse{Message: err.Error()})
		}
	}
}

func handleStop(ctrl Controller) gin.HandlerFunc {
	return func(c *gin.Context) {
		snap, err := ctrl.Stop()
		if errors.Is(err, run.ErrNotRunning) {
			c.JSON(http.StatusConflict, MessageResponse{Message: err.Error()})
			return
		}
		c.JSON(http.StatusAccepted, snap)
	}
}

var upgrader = websocket.Upgrader{
	// Only fru-lab's backend reaches this port (token-guarded above).
	CheckOrigin: func(*http.Request) bool { return true },
}

// handleStream pushes a run.Snapshot JSON text frame immediately, on every
// state change, and at least once per streamInterval, until the client
// goes away.
func handleStream(ctrl Controller) gin.HandlerFunc {
	return func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()

		gone := make(chan struct{})
		go func() { // the client never sends; reading detects its close
			defer close(gone)
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					return
				}
			}
		}()

		ticker := time.NewTicker(streamInterval)
		defer ticker.Stop()
		for {
			changed := ctrl.Changed()
			sent := time.Now()
			if err := conn.WriteJSON(ctrl.Snapshot()); err != nil {
				return
			}
			select {
			case <-gone:
				return
			case <-changed:
			case <-ticker.C:
			}
			// coalesce bursts of changes into one frame per minFrameGap
			if wait := minFrameGap - time.Since(sent); wait > 0 {
				t := time.NewTimer(wait)
				select {
				case <-gone:
					t.Stop()
					return
				case <-t.C:
				}
			}
		}
	}
}
```

- [ ] **Step 4: Run the tests.** Run: `cd tester && go test -race ./api/`. Expected: `ok`.
- [ ] **Step 5: Commit** `feat: throttle tester snapshot stream to 5 frames per second`.

---

### Task 8: OpenAPI schema and client

**Files:** Modify `web/openapi.yaml`; regenerate `web/frontend/src/api/`

- [ ] **Step 1: Apply the schema change.** The script below holds the exact old → new edits. Save it as `/tmp/openapi_phase2.py`, then run `python3 /tmp/openapi_phase2.py web/openapi.yaml`.

```python
import sys
p=sys.argv[1]; s=open(p).read()
def sub(old,new):
    global s; assert s.count(old)==1,(old[:80],s.count(old)); s=s.replace(old,new)
sub('''    TesterProfile:
      type: object
      required: [name, scale, gnb, network, rates]''','''    TesterProfile:
      type: object
      required: [name, scale, gnb, ue, network, rates]''')
sub('''        network:
          type: object
          required: [n2, n3]''','''        ue:
          $ref: '#/components/schemas/TesterUeTemplate'
        network:
          type: object
          required: [n2, n3]''')
sub('''        rates:
          type: object
          required: [n2]
          properties:
            n2:
              $ref: '#/components/schemas/TesterStageRate'
''','''        rates:
          type: object
          required: [n2, registration, pdu]
          properties:
            n2:
              $ref: '#/components/schemas/TesterStageRate'
            registration:
              $ref: '#/components/schemas/TesterProcedureRate'
            pdu:
              $ref: '#/components/schemas/TesterProcedureRate'
    TesterUeTemplate:
      type: object
      description: Expanded once per UE. MCC/MNC come from the gNB template; MCC+MNC+MSIN must be 15 digits. Key/OPc/AMF/SQN must match the subscribers created in the core.
      required: [msinStart, key, opc, amf, sqn, integrity, ciphering, dnn, sst, sd]
      properties:
        msinStart:
          type: string
          description: Decimal; incremented per UE, keeping its width.
          example: '0000000001'
        key:
          type: string
          example: 8baf473f2f8fd09487cccbd7097c6862
        opc:
          type: string
          example: 8e27b6af0e692e750f32667a3b14605d
        amf:
          type: string
          example: '8000'
        sqn:
          type: string
          example: '000000000023'
        integrity:
          type: string
          enum: [nia0, nia1, nia2, nia3]
        ciphering:
          type: string
          enum: [nea0, nea1, nea2, nea3]
        dnn:
          type: string
          example: internet
        sst:
          type: integer
          example: 1
        sd:
          type: string
          example: '010203'
    TesterProcedureRate:
      type: object
      description: At most ratePerSec new attempts per second and at most maxInFlight at once; each attempt bounded by timeoutMs; a failed UE with retries left is requeued at the back.
      required: [ratePerSec, maxInFlight, timeoutMs, retries]
      properties:
        ratePerSec:
          type: integer
          example: 50
        maxInFlight:
          type: integer
          example: 200
        timeoutMs:
          type: integer
          example: 10000
        retries:
          type: integer
          example: 1
''')
sub('''      required: [index, name, gnbId, n2Ip, n3Ip, ueCount, ueFirst, ueLast]''','''      required: [index, name, gnbId, n2Ip, n3Ip, ueCount, ueFirst, ueLast, firstSupi, lastSupi]''')
sub('''        ueLast:
          type: integer
''','''        ueLast:
          type: integer
        firstSupi:
          type: string
          description: SUPI of this gNB's first UE; empty when it has none.
          example: imsi-208930000000001
        lastSupi:
          type: string
''')
sub('''      required: [name, expected, attempted, retries, inFlight, accepted, rejected, timedOut, failed, done, totalTimeMs, avgMs, p50Ms, p95Ms, p99Ms, maxMs, causes]''','''      required: [name, expected, attempted, retries, inFlight, accepted, rejected, timedOut, failed, skipped, done, totalTimeMs, avgMs, p50Ms, p95Ms, p99Ms, maxMs, causes]''')
sub('''        failed:
          type: integer
          format: int64
        done:
          type: boolean''','''        failed:
          type: integer
          format: int64
        skipped:
          type: integer
          format: int64
          description: Never attempted (gNB down, earlier stage failed, or the run was stopped first).
        done:
          type: boolean''')
sub('''          required: [state, attempts, latencyMs, cause]''','''          required: [state, attempts, latencyMs, cause, registered, established]''')
sub('''            latencyMs:
              type: number
            cause:
              type: string
''','''            latencyMs:
              type: number
            cause:
              type: string
            registered:
              type: integer
            established:
              type: integer
''')
sub('''      required: [runId, profileName, state, error, n2, gnbs]''','''      required: [runId, profileName, state, error, n2, registration, pdu, gnbs, ues, failedUes]''')
sub('''        n2:
          $ref: '#/components/schemas/TesterStageSnapshot'
        gnbs:
          type: array
          items:
            $ref: '#/components/schemas/TesterGnbStatus\'''','''        n2:
          $ref: '#/components/schemas/TesterStageSnapshot'
        registration:
          $ref: '#/components/schemas/TesterStageSnapshot'
        pdu:
          $ref: '#/components/schemas/TesterStageSnapshot'
        gnbs:
          type: array
          items:
            $ref: '#/components/schemas/TesterGnbStatus'
        ues:
          $ref: '#/components/schemas/TesterUeSummary'
        failedUes:
          type: array
          description: The first 200 UEs that failed; ues.failed has the total.
          items:
            $ref: '#/components/schemas/TesterUeFailure'
    TesterUeSummary:
      type: object
      description: UEs per pipeline state; the fields add up to the UE count.
      required: [pending, registering, registered, establishing, established, failed, skipped, cancelled]
      properties:
        pending:
          type: integer
        registering:
          type: integer
        registered:
          type: integer
        establishing:
          type: integer
        established:
          type: integer
        failed:
          type: integer
        skipped:
          type: integer
        cancelled:
          type: integer
    TesterUeFailure:
      type: object
      required: [supi, gnb, stage, cause, attempts]
      properties:
        supi:
          type: string
        gnb:
          type: string
        stage:
          type: string
          enum: [registration, pdu]
        cause:
          type: string
          example: 5gmm(62)
        attempts:
          type: integer''')
open(p,'w').write(s)
```

- [ ] **Step 2: Generate.** Run: `make openapi && sudo chown -R "$USER" web/frontend/src/api && grep -c "TesterUeTemplate\|TesterUeSummary\|TesterProcedureRate" web/frontend/src/api/api.ts`. Expected: a non-zero count. `yarn build` now fails in `testerDefaults.ts` (missing `ue` and rates) until Task 9.
- [ ] **Step 3: Commit** `feat: openapi schema for ue stages`, together with Task 9 if you prefer the tree always building. Either is fine; the ledger records the choice.

---

### Task 9: Setup page — UE template and pacing

**Files:** Modify `web/frontend/src/page/tester/testerDefaults.ts`, `web/frontend/src/page/tester/TesterSetupPage.tsx`

The default `ue` template is free-ran-ue's sample subscriber (`imsi-208930000000001`, the K/OPc that the user's core already has for 4 subscribers). `normalizeProfile` fills `ue` and the new rates into profiles saved by Phase 1.

- [ ] **Step 1: Apply the edits.** Save the script as `/tmp/frontend_setup_phase2.py` and run `python3 /tmp/frontend_setup_phase2.py web/frontend/src/page/tester`:

```python
import sys
root=sys.argv[1]  # web/frontend/src/page/tester
def edit(name, pairs):
    p=root+'/'+name; s=open(p).read()
    for old,new in pairs:
        assert s.count(old)==1,(name,old[:80],s.count(old)); s=s.replace(old,new)
    open(p,'w').write(s)
edit('testerDefaults.ts',[
('''// frulab-cn-ran bridge, host side docker-cn-ran) so it runs as-is there.''','''// frulab-cn-ran bridge, host side docker-cn-ran) so it runs as-is there.
// The UE template matches free-ran-ue's sample ue.yaml subscriber; create
// subscribers imsi-208930000000001.. in the core with the same keys.'''),
('''  network: {
    n2:''','''  ue: {
    msinStart: '0000000001',
    key: '8baf473f2f8fd09487cccbd7097c6862',
    opc: '8e27b6af0e692e750f32667a3b14605d',
    amf: '8000',
    sqn: '000000000023',
    integrity: 'nia2',
    ciphering: 'nea0',
    dnn: 'internet',
    sst: 1,
    sd: '010203',
  },
  network: {
    n2:'''),
('''  rates: { n2: { timeoutMs: 5000, retries: 1 } },''','''  rates: {
    n2: { timeoutMs: 5000, retries: 1 },
    registration: { ratePerSec: 50, maxInFlight: 200, timeoutMs: 10000, retries: 1 },
    pdu: { ratePerSec: 50, maxInFlight: 200, timeoutMs: 10000, retries: 1 },
  },'''),
])
edit('TesterSetupPage.tsx',[
('''  numeric?: boolean
  onChange: (path: string, value: string | number) => void
}

function Field({ label, path, profile, errors, numeric = false, onChange }: FieldProps) {
  const message = errors.find((e) => e.field === path)?.message
  const value = getPath(profile, path)
  return (
    <div className={styles.field}>
      <label htmlFor={path}>{label}</label>
      <input
        id={path}
        className={`${styles.input} ${message ? styles.inputError : ''}`}
        type={numeric ? 'number' : 'text'}
        value={numeric && Number.isNaN(value) ? '' : value}
        onChange={(e) => onChange(path, numeric ? e.target.valueAsNumber : e.target.value)}
      />
      {message && <p className={styles.fieldError}>{message}</p>}
    </div>
  )
}''','''  numeric?: boolean
  options?: string[] // render a select instead of an input
  onChange: (path: string, value: string | number) => void
}

function Field({ label, path, profile, errors, numeric = false, options, onChange }: FieldProps) {
  const message = errors.find((e) => e.field === path)?.message
  const value = getPath(profile, path)
  const className = `${styles.input} ${message ? styles.inputError : ''}`
  return (
    <div className={styles.field}>
      <label htmlFor={path}>{label}</label>
      {options ? (
        <select id={path} className={className} value={value} onChange={(e) => onChange(path, e.target.value)}>
          {options.map((o) => <option key={o} value={o}>{o}</option>)}
        </select>
      ) : (
        <input
          id={path}
          className={className}
          type={numeric ? 'number' : 'text'}
          value={numeric && Number.isNaN(value) ? '' : value}
          onChange={(e) => onChange(path, numeric ? e.target.valueAsNumber : e.target.value)}
        />
      )}
      {message && <p className={styles.fieldError}>{message}</p>}
    </div>
  )
}

// RateFields renders one per-UE stage's pacing knobs.
function RateFields({ stage, fieldProps }: { stage: 'registration' | 'pdu', fieldProps: Omit<FieldProps, 'label' | 'path'> }) {
  return (
    <>
      <Field label="Starts per second" path={`rates.${stage}.ratePerSec`} numeric {...fieldProps} />
      <Field label="Max in flight" path={`rates.${stage}.maxInFlight`} numeric {...fieldProps} />
      <Field label="Timeout per attempt (ms)" path={`rates.${stage}.timeoutMs`} numeric {...fieldProps} />
      <Field label="Retries" path={`rates.${stage}.retries`} numeric {...fieldProps} />
    </>
  )
}'''),
('''            <p className={styles.subtitle}>Describe the gNBs to simulate. Phase 1 brings up N2 (SCTP + NG Setup) for every gNB and holds it until you stop the run.</p>''','''            <p className={styles.subtitle}>Describe the gNBs and UEs to simulate. A run brings up N2 for every gNB, registers its UEs and establishes one PDU session each, then holds everything until you stop it.</p>'''),
('''            <section className={styles.card}>
              <h3 className={styles.cardTitle}>N2 · gNB ↔ AMF</h3>''','''            <section className={styles.card}>
              <h3 className={styles.cardTitle}>UE template</h3>
              <div className={styles.fieldGrid}>
                <Field label="First MSIN" path="ue.msinStart" {...fieldProps} />
                <Field label="Key (K)" path="ue.key" {...fieldProps} />
                <Field label="OPc" path="ue.opc" {...fieldProps} />
                <Field label="AMF" path="ue.amf" {...fieldProps} />
                <Field label="SQN" path="ue.sqn" {...fieldProps} />
                <Field label="Integrity" path="ue.integrity" options={['nia0', 'nia1', 'nia2', 'nia3']} {...fieldProps} />
                <Field label="Ciphering" path="ue.ciphering" options={['nea0', 'nea1', 'nea2', 'nea3']} {...fieldProps} />
                <Field label="DNN" path="ue.dnn" {...fieldProps} />
                <Field label="SST" path="ue.sst" numeric {...fieldProps} />
                <Field label="SD (hex, optional)" path="ue.sd" {...fieldProps} />
              </div>
              <p className={styles.hint}>
                UEs use the gNB template's PLMN; the MSIN is incremented per UE. The tester does not create subscribers: add
                {plan && plan.gnbs.length > 0
                  ? <> <span className={styles.mono}>{plan.gnbs[0].firstSupi}</span> … <span className={styles.mono}>{lastSupi(plan)}</span></>
                  : ' every UE'}
                {' '}to the core with these keys and a slice the gNB advertises.
              </p>
            </section>

            <section className={styles.card}>
              <h3 className={styles.cardTitle}>N2 · gNB ↔ AMF</h3>'''),
('''              <p className={styles.hint}>A failed attempt with retries left goes to the back of the queue.</p>
            </section>''','''              <p className={styles.hint}>A failed attempt with retries left goes to the back of the queue.</p>
            </section>

            <section className={styles.card}>
              <h3 className={styles.cardTitle}>Registration pacing</h3>
              <div className={styles.fieldGrid}>
                <RateFields stage="registration" fieldProps={fieldProps} />
              </div>
              <p className={styles.hint}>Starts per second caps how fast new registrations begin; max in flight caps how many wait for the core at once.</p>
            </section>

            <section className={styles.card}>
              <h3 className={styles.cardTitle}>PDU session pacing</h3>
              <div className={styles.fieldGrid}>
                <RateFields stage="pdu" fieldProps={fieldProps} />
              </div>
              <p className={styles.hint}>A UE moves on to its PDU session as soon as it is registered.</p>
            </section>'''),
('''                      <tr><th>#</th><th>Name</th><th>gNB ID</th><th>N2 IP</th><th>N3 IP</th><th>UEs</th></tr>''','''                      <tr><th>#</th><th>Name</th><th>gNB ID</th><th>N2 IP</th><th>N3 IP</th><th>UEs</th><th>SUPIs</th></tr>'''),
('''                          <td>{g.ueCount ? `${g.ueCount} (#${g.ueFirst}–${g.ueLast})` : '0'}</td>''','''                          <td>{g.ueCount ? `${g.ueCount} (#${g.ueFirst}–${g.ueLast})` : '0'}</td>
                          <td className={styles.mono}>{g.ueCount ? `${g.firstSupi} … ${g.lastSupi.slice(-4)}` : '—'}</td>'''),
('''const PREVIEW_ROWS = 10''','''const PREVIEW_ROWS = 10

// lastSupi is the SUPI of the plan's last UE (the last gNB may own none).
function lastSupi(plan: TesterPlan): string {
  for (let i = plan.gnbs.length - 1; i >= 0; i--) {
    if (plan.gnbs[i].lastSupi) return plan.gnbs[i].lastSupi
  }
  return ''
}'''),
('''import type { TesterFieldError, TesterProfile, TesterValidateResponse } from '../../api\'''','''import type { TesterFieldError, TesterPlan, TesterProfile, TesterValidateResponse } from '../../api\''''),
])
```

- [ ] **Step 2: Verify.** Run: `cd web/frontend && yarn build`. Expected: the build succeeds.
- [ ] **Step 3:** Run the `normalizeProfile` scratch assertions from Phase 1's I3 fix again (bundle with esbuild, run with node). Also check that a Phase 1-shaped profile, one without `ue`, normalizes to the default `ue`.
- [ ] **Step 4: Commit** `feat: ue template and pacing on the tester setup page`.

---

### Task 10: Run page — stage cards, UE summary, failed UEs

**Files:** Modify `web/frontend/src/page/tester/TesterRunPage.tsx`, `web/frontend/src/page/tester/tester.module.css`

- [ ] **Step 1: Apply the edits.** Save as `/tmp/frontend_run_phase2.py` and run `python3 /tmp/frontend_run_phase2.py web/frontend/src/page/tester`:

```python
import sys
root=sys.argv[1]  # web/frontend/src/page/tester
def edit(name, pairs):
    p=root+'/'+name; s=open(p).read()
    for old,new in pairs:
        assert s.count(old)==1,(name,old[:80],s.count(old)); s=s.replace(old,new)
    open(p,'w').write(s)
edit('TesterRunPage.tsx',[
('''  running: 'Holding N2',''','''  running: 'Running','''),
('''        <span className={`${styles.pill} ${stage.done ? styles.pillOk : styles.pillActive}`}>
          {stage.done ? 'Done' : `${stage.inFlight} in flight`}
        </span>''','''        <span className={`${styles.pill} ${stage.done ? styles.pillOk : styles.pillActive}`}>
          {stage.done ? 'Done' : stage.attempted === 0 ? 'Waiting' : `${stage.inFlight} in flight`}
        </span>'''),
('''        <dt>Retries</dt><dd>{stage.retries}</dd>''','''        <dt>Retries</dt><dd>{stage.retries}</dd>
        <dt>Not started</dt><dd>{stage.skipped}</dd>'''),
('''            <StageCard title="N2 setup" stage={snapshot.n2} />
''','''            <div className={styles.stageRow}>
              <StageCard title="N2 setup" stage={snapshot.n2} />
              <StageCard title="Registration" stage={snapshot.registration} />
              <StageCard title="PDU session" stage={snapshot.pdu} />
            </div>

            <section className={styles.card}>
              <h3 className={styles.cardTitle}>UEs</h3>
              <div className={styles.ueChips}>
                {UE_STATES.map(([key, label, tone]) => (
                  <span key={key} className={`${styles.pill} ${styles[tone]}`}>{label} {snapshot.ues[key]}</span>
                ))}
              </div>
              {snapshot.failedUes.length > 0 && (
                <div className={styles.tableWrap}>
                  <h4 className={styles.subTitle}>
                    Failed UEs{snapshot.ues.failed > snapshot.failedUes.length && ` (first ${snapshot.failedUes.length} of ${snapshot.ues.failed})`}
                  </h4>
                  <table className={styles.table}>
                    <thead>
                      <tr><th>SUPI</th><th>gNB</th><th>Stage</th><th>Cause</th><th>Attempts</th></tr>
                    </thead>
                    <tbody>
                      {snapshot.failedUes.map((f) => (
                        <tr key={f.supi}>
                          <td className={styles.mono}>{f.supi}</td>
                          <td>{f.gnb}</td>
                          <td>{f.stage === 'pdu' ? 'PDU session' : 'Registration'}</td>
                          <td className={styles.mono}>{f.cause}</td>
                          <td>{f.attempts}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </section>
'''),
('''                    <tr><th>#</th><th>Name</th><th>N2 IP</th><th>State</th><th>Attempts</th><th>Setup time</th><th>Last error</th></tr>''','''                    <tr><th>#</th><th>Name</th><th>N2 IP</th><th>State</th><th>Attempts</th><th>Setup time</th><th>Registered</th><th>PDU sessions</th><th>Last error</th></tr>'''),
('''                        <td>{g.state === 'up' || g.state === 'closed' ? formatMs(g.latencyMs) : '—'}</td>''','''                        <td>{g.state === 'up' || g.state === 'closed' ? formatMs(g.latencyMs) : '—'}</td>
                        <td>{g.registered} / {g.ueCount}</td>
                        <td>{g.established} / {g.ueCount}</td>'''),
('''const ACTIVE_STATES''','''// UE_STATES orders the UE summary chips: key, label, pill style.
const UE_STATES: [keyof TesterUeSummary, string, 'pillOk' | 'pillBad' | 'pillActive' | 'pillMuted'][] = [
  ['established', 'Established', 'pillOk'],
  ['establishing', 'Establishing', 'pillActive'],
  ['registered', 'Registered', 'pillActive'],
  ['registering', 'Registering', 'pillActive'],
  ['pending', 'Pending', 'pillMuted'],
  ['failed', 'Failed', 'pillBad'],
  ['skipped', 'gNB down', 'pillMuted'],
  ['cancelled', 'Cancelled', 'pillMuted'],
]

const ACTIVE_STATES'''),
('''import type { TesterRunSnapshot, TesterStageSnapshot } from '../../api\'''','''import type { TesterRunSnapshot, TesterStageSnapshot, TesterUeSummary } from '../../api\''''),
])
p=root+'/tester.module.css'; s=open(p).read()
old='''.runError {'''
new='''.stageRow { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 1.5rem; }
.ueChips { display: flex; flex-wrap: wrap; gap: 0.5rem; }
.subTitle { margin: 1.25rem 0 0.5rem; font-size: 0.85rem; color: #475569; }
.runError {'''
assert s.count(old)==1; open(p,'w').write(s.replace(old,new))
```

- [ ] **Step 2: Verify.** Run: `cd web/frontend && yarn build`. Expected: the build succeeds.
- [ ] **Step 3: Commit** `feat: registration and pdu stats on the tester run page`.

---

### Task 11: Guide

**Files:** Modify `docs/tester-guide.md`

- [ ] **Step 1: Update the guide.**
  - Change the intro: Phase 2 registers UEs and establishes one PDU session each.
  - Add a "Subscribers" section: the tester never creates them. Provision `firstSupi … lastSupi` from the preview with the UE template's K/OPc/AMF/SQN, on a slice the gNB advertises. Otherwise the AMF rejects with `5gmm(62)`.
  - Add a "Pacing" section covering starts per second vs max in flight.
  - Extend "What the numbers mean":
    - registration timing: Initial UE Message → Registration Complete;
    - PDU timing: request → accept;
    - "Not started" (skipped);
    - the UE chips;
    - the failed-UE list (first 200).
  - Extend "Known limitations" with the three from this plan's Review Focus footer.
- [ ] **Step 2: Commit** `docs: tester guide for registration and pdu sessions`.

---

### Task 12: Acceptance against the real core, and the artifact record

This task is manual, against the user's running free5GC, and needs permission to hit it (granted: "我現在核網會用 docker 一直開著 你可以直接打").

- [ ] **Step 1:** `make tester frontend`, then restart `fru-tester` from the new build. Restarting the user's process needs their OK; otherwise run a second instance on another port as the prototype did, e.g. `listen: 127.0.0.1:9101` with a separate IP range.
- [ ] **Step 2:** Use the profile `basic` with 2 gNBs and as many UEs as there are matching subscribers on slice 1/010203. Expect every such UE to reach `established`, and both stage cards to show `Done` with non-zero averages. In the AMF log, expect "Select SMF … internet".
- [ ] **Step 3:** Include a subscriber on another slice, or one that is not provisioned. Expect it to be `rejected` with `5gmm(62)`, or the not-provisioned cause, and listed in the failed UEs.
- [ ] **Step 4:** Press Stop. Expect every stage `Done`, the IPs removed, and the AMF to remove the RAN contexts.
- [ ] **Step 5:** Record Phase 2 in artifact section 13 (memory rule `tester-phase-log-in-artifact`): delivered, verified (with real numbers), deviations and rulings, review fixes, and what is left. Mark the "第 2 期" card 已完成.
