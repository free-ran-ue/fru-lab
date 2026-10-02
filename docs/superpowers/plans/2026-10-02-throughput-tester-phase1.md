# Throughput Tester Phase 1 (Skeleton + N2) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the first usable slice of the 5G Throughput Tester: a separate `fru-tester` engine that brings up N2 (SCTP + NG Setup) for N simulated gNBs against any AMF, and a new fru-lab sidebar section to configure it, start/stop it, and watch per-stage N2 statistics live.

**Architecture:** `fru-tester` is a new Go module at `tester/` with one package per concern: `profile` (validate and expand the user's profile into per-gNB settings and IPs), `metrics` (stage counters and latency histogram), `gnb` (NGAP identity, NG Setup exchange, SCTP dialer), `netcfg` (add/remove gNB IPs via netlink), `run` (the single active run's lifecycle) and `api` (REST + WebSocket). fru-lab's backend stores the profile in its bbolt DB and reverse-proxies everything else to fru-tester with a shared API token. The React frontend gets a Setup page (form + live validation preview) and a Run page (N2 stage card + gNB table over a WebSocket).

**Tech Stack:** Go 1.26.2, gin, gorilla/websocket, free5gc/ngap + free5gc/sctp, vishvananda/netlink, bbolt, React 19 + Vite + generated OpenAPI axios client.

**Spec:** Design doc artifact https://claude.ai/artifact/JmvGVKAnieV1CvNAbnkf9f (read it with the Artifact tool, `action: "read"`). Its sections 2–5 and 9 describe this phase; section 12 "第 1 期" is the scope. The user's answers to every open question (Q1–Q17, N1–N7) are in `/home/alonza/fru-lab/5g-throughput-tester.md` under `## Q` and `## N`. That file is **untracked**, so a git worktree will not contain it; read it from the main checkout path above.

## Global Constraints

- Go `1.26.2`, Node `v20`, yarn `1.22.22` (README "Develop Environment").
- `tester/` is its own Go module named `tester`. It must not import the untracked `free-ran-ue/` directory; code from free-ran-ue is copied in and adapted (spec: "可以完全 copy free-ran-ue 的 code，但是不能直接用").
- Pin dependency versions to the ones free-ran-ue and fru-lab already use: `github.com/free5gc/ngap v1.2.0`, `github.com/free5gc/sctp v1.2.0`, `github.com/free5gc/openapi v1.3.0`, `github.com/free-ran-ue/util v0.2.0`, `github.com/gin-gonic/gin v1.12.0`, `github.com/gorilla/websocket v1.5.3`, `github.com/Alonza0314/logger-go/v2 v2.1.0`, `github.com/spf13/cobra v1.10.2`, `github.com/stretchr/testify v1.12.1`, `github.com/vishvananda/netlink v1.3.1`, `gopkg.in/yaml.v3 v3.0.1`.
- Only one run at a time (Q17). Any core network; the user types every interface and IP by hand (Q2).
- UEs fill gNBs in order: each gNB takes ⌈UE ÷ gNB⌉ and the last one may be partly filled. There is no choice of mode (Q5).
- Each gNB gets its own N2 IP and its own N3 IP. N2 and N3 each have their own CIDR and start IP and are incremented separately (Q6, N1). Allocation skips the network and broadcast addresses, both core IPs (AMF and UPF), and every IP already on the host (N2).
- A stage's "total time" is wall clock from the first attempt starting to the last item finishing (Q8). Averages and percentiles cover accepted items only.
- Every stage except the data plane has a retry count, and a failed item with retries left goes to the back of the queue (Q13, N3).
- Stop is the only way to end a run in Phase 1. It closes every N2 association and removes every IP fru-tester added. UE deregistration and SCTP-cut statistics arrive in Phase 4 (Q12).
- fru-tester needs `CAP_NET_ADMIN` (it adds IPs with netlink) and the `sctp` kernel module (`sudo modprobe sctp`).
- UI copy is English, matching the existing fru-lab pages (Dashboard, Images, …).
- Frontend verification is `yarn build` (tsc + vite). `npx eslint` already fails to load its config across the whole repo; that is pre-existing and out of scope.
- Commit messages follow the repo style (`feat: …`, `docs: …`, lowercase) and end with the line `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## Review Focus

1. **Stop pressed while gNB IPs are still being added** → no SCTP attempt is made, every IP already added is removed, and the run ends `stopped`. Pinned by `TestStopWhileConfiguringSkipsN2` (Task 9).
2. **An interface name that does not exist on this host** → the setup page shows a field error before Start, instead of the run failing after it starts. Pinned by `TestValidateFlagsUnknownInterface` (Task 9).
3. **N2 and N3 sharing one interface and CIDR** → no IP is handed out twice, and neither the AMF nor the UPF IP is ever given to a gNB. Pinned by `TestExpandSharedN2N3CidrNeverReusesAnIP` (Task 4).
4. **The AMF drops an association that was up (AMF restart)** → that gNB shows `lost` with the error, and the run keeps holding the others. Pinned by `TestLostAssociationIsReported` (Task 9).
5. **fru-lab running without `backend.tester.url`, or fru-tester down** → the tester pages show a clear 503 or 502 message instead of hanging. Pinned by `TestTesterRoutesAnswer503WhenNotConfigured` and `TestTesterProxyReportsUnreachableAs502` (Task 13).

Known Phase 1 limitations, documented in Task 17 and not fixed here: if fru-tester is killed with SIGKILL, the gNB IPs it added stay on the interface, and later runs simply skip them as host IPs. AMF-initiated NGAP messages are read and discarded. There is no run history yet (Phase 4).

## File Structure

New module `tester/`:

| Path | Responsibility |
|---|---|
| `tester/go.mod`, `tester/.golangci.yaml` | Module and lint config (copied from `web/backend/.golangci.yaml`) |
| `tester/main.go`, `tester/cmd/root.go` | cobra entry point: load config, wire deps, serve, shut down cleanly |
| `tester/config/config.go` | fru-tester's own YAML config (listen address, API token, log level) |
| `tester/profile/profile.go` | `Profile` JSON contract shared with the frontend |
| `tester/profile/errors.go` | `FieldError`, `ValidationError` |
| `tester/profile/increment.go` | Hex ID increment, `{i}` name pattern |
| `tester/profile/ipalloc.go` | CIDR allocation with exclusions |
| `tester/profile/expand.go` | Field validation + expansion to `Plan` / `GnbSpec` |
| `tester/metrics/histogram.go`, `stage.go` | Fixed-memory latency histogram; per-stage counters and `StageSnapshot` |
| `tester/gnb/identity.go` | Profile → NGAP IEs; NG Setup Request encoding |
| `tester/gnb/ngsetup.go` | NG Setup exchange; `RejectedError`; cause text |
| `tester/gnb/dial.go` | SCTP dialer with connect/read timeouts and NGAP PPID |
| `tester/netcfg/netcfg.go` | `AddrManager` interface + netlink implementation |
| `tester/run/snapshot.go`, `classify.go`, `controller.go` | Run states, outcome classification, run lifecycle |
| `tester/api/api.go` | REST + WebSocket handlers |

fru-lab changes:

| Path | Change |
|---|---|
| `Makefile` | `tester`, `run-tester`, `test` targets; tidy/lint/all include tester |
| `config.yaml`, `tester.yaml` (new) | Dev config for fru-lab → fru-tester |
| `web/backend/config/config.go` | `backend.tester.url` / `apiToken` |
| `web/backend/logger/{logger.go,tag.go}` | `TesterLog` |
| `web/backend/internal/context/{db.go,dbBbolt.go,dbContext.go}` | Generic `Get`/`Put`; tester profile accessors |
| `web/backend/internal/processor/tester.go`, `web/backend/model/tester.go` | Profile get/put |
| `web/backend/internal/testerProxy.go`, `api_tester.go`, `backend.go` | Reverse proxy, routes, wiring |
| `web/openapi.yaml` + generated `web/frontend/src/api/` | Tester endpoints and schemas |
| `web/frontend/src/components/sidebar/*`, `App.tsx` | New sidebar group and routes |
| `web/frontend/src/page/tester/*` | Setup and Run pages |
| `docker/*`, `README.md` | Image, compose service, docs |

---

### Task 1: fru-tester module skeleton and config

**Files:**
- Create: `tester/go.mod`, `tester/.golangci.yaml`, `tester/config/config.go`, `tester/config/config_test.go`
- Modify: `Makefile`

**Interfaces:**
- Produces: `config.Load(path string) (*config.Config, error)`; `config.Config{Listen, ApiToken string; Logger config.Logger{Level string}}`. Task 11 uses it.

- [ ] **Step 1: Create the module**

```bash
mkdir -p tester/config
cd tester
go mod init tester
go mod edit -go=1.26.2
cp ../web/backend/.golangci.yaml .
```

- [ ] **Step 2: Write the failing test** at `tester/config/config_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func write(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tester.yaml")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}

func TestLoadAppliesDefaults(t *testing.T) {
	cfg, err := Load(write(t, "apiToken: abc\n"))
	require.NoError(t, err)
	require.Equal(t, &Config{Listen: "0.0.0.0:9100", ApiToken: "abc", Logger: Logger{Level: "info"}}, cfg)
}

func TestLoadOverrides(t *testing.T) {
	cfg, err := Load(write(t, "listen: 127.0.0.1:9200\napiToken: abc\nlogger:\n  level: debug\n"))
	require.NoError(t, err)
	require.Equal(t, "127.0.0.1:9200", cfg.Listen)
	require.Equal(t, "debug", cfg.Logger.Level)
}

func TestLoadRequiresToken(t *testing.T) {
	_, err := Load(write(t, "listen: :9100\n"))
	require.ErrorContains(t, err, "apiToken must be set")
}
```

- [ ] **Step 3: Run it to make sure it fails**

Run: `cd tester && go get github.com/stretchr/testify@v1.12.1 && go test ./config/`
Expected: FAIL (`undefined: Load`, `undefined: Config`).

- [ ] **Step 4: Implement** `tester/config/config.go`:

```go
// Package config loads fru-tester's own YAML config (not a test profile).
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	// Listen is host:port for the API. fru-lab reaches it from inside its
	// container, so the default binds every interface; ApiToken guards it.
	Listen   string `yaml:"listen"`
	ApiToken string `yaml:"apiToken"`
	Logger   Logger `yaml:"logger"`
}

type Logger struct {
	Level string `yaml:"level"`
}

func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := &Config{Listen: "0.0.0.0:9100", Logger: Logger{Level: "info"}}
	if err := yaml.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if cfg.ApiToken == "" {
		return nil, fmt.Errorf("%s: apiToken must be set", path)
	}
	return cfg, nil
}
```

- [ ] **Step 5: Run the tests**

Run: `cd tester && go get gopkg.in/yaml.v3@v3.0.1 && go mod tidy && go test ./config/`
Expected: `ok  tester/config`.

- [ ] **Step 6: Add Makefile targets.** In `Makefile`, change the `.PHONY` line and `all`, and add the tester lines to `tidy` and `lint`:

```make
.PHONY: backend frontend tester openapi openapi-webconsole run run-tester test tidy lint clean docker
```

```make
all: backend frontend tester
```

Add after the `run:` target:

```make
test:
	cd tester && go test -race ./...
	cd web/backend && go test ./...
```

```make
tidy:
	cd web/backend && go mod tidy
	cd tester && go mod tidy

lint:
	cd web/backend && golangci-lint run
	cd tester && golangci-lint run
```

(`tester` and `run-tester` targets are added in Task 11, once there is a `main` package.)

- [ ] **Step 7: Verify and commit**

Run: `make test && cd tester && golangci-lint run`
Expected: tests pass, `0 issues.`

```bash
git add tester/go.mod tester/go.sum tester/.golangci.yaml tester/config Makefile
git commit -m "$(cat <<'EOF'
feat: fru-tester module skeleton and config

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 2: Profile contract, field errors, increments

**Files:**
- Create: `tester/profile/profile.go`, `tester/profile/errors.go`, `tester/profile/increment.go`, `tester/profile/increment_test.go`

**Interfaces:**
- Produces: the `profile.Profile` struct tree (JSON field names are the contract with the frontend and with `web/openapi.yaml` in Task 14); `profile.FieldError{Field, Message string}`; `*profile.ValidationError{Errors []FieldError}` with an unexported `add(field, message string)`; `profile.IncrementHex(start string, step int) (string, error)`; `profile.RenderName(pattern string, index int) string`.

- [ ] **Step 1: Write the types.** `tester/profile/profile.go`:

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
	N2 StageRate `json:"n2"`
}

// StageRate bounds one attempt with TimeoutMs; Retries is how many more
// attempts a failed item gets (it is requeued at the back each time).
type StageRate struct {
	TimeoutMs int `json:"timeoutMs"`
	Retries   int `json:"retries"`
}
```

`tester/profile/errors.go`:

```go
package profile

import "strings"

// FieldError points at one invalid field using its JSON path, so the setup
// page can show the message next to the input.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationError collects every problem found, not just the first.
type ValidationError struct {
	Errors []FieldError
}

func (e *ValidationError) Error() string {
	parts := make([]string, 0, len(e.Errors))
	for _, fe := range e.Errors {
		parts = append(parts, fe.Field+": "+fe.Message)
	}
	return "invalid profile: " + strings.Join(parts, "; ")
}

func (e *ValidationError) add(field, message string) {
	e.Errors = append(e.Errors, FieldError{Field: field, Message: message})
}
```

- [ ] **Step 2: Write the failing test** `tester/profile/increment_test.go`:

```go
package profile

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIncrementHex(t *testing.T) {
	cases := []struct {
		start string
		step  int
		want  string
	}{
		{"000314", 0, "000314"},
		{"000314", 1, "000315"},
		{"0003ff", 1, "000400"},
		{"00000001", 9, "0000000a"},
	}
	for _, c := range cases {
		got, err := IncrementHex(c.start, c.step)
		require.NoError(t, err)
		require.Equal(t, c.want, got)
	}
}

func TestIncrementHexRejectsOverflowAndBadInput(t *testing.T) {
	_, err := IncrementHex("ffffff", 1)
	require.ErrorContains(t, err, "overflows")
	_, err = IncrementHex("00031", 0)
	require.ErrorContains(t, err, "hex")
	_, err = IncrementHex("zz", 0)
	require.ErrorContains(t, err, "hex")
	_, err = IncrementHex("", 0)
	require.ErrorContains(t, err, "hex")
}

func TestRenderName(t *testing.T) {
	require.Equal(t, "gNB-3", RenderName("gNB-{i}", 3))
	require.Equal(t, "g3-3", RenderName("g{i}-{i}", 3))
}
```

- [ ] **Step 3: Run it to make sure it fails**

Run: `cd tester && go test ./profile/`
Expected: FAIL (`undefined: IncrementHex`, `undefined: RenderName`).

- [ ] **Step 4: Implement** `tester/profile/increment.go`:

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

// RenderName replaces every "{i}" in pattern with the 1-based index.
func RenderName(pattern string, index int) string {
	return strings.ReplaceAll(pattern, "{i}", strconv.Itoa(index))
}
```

- [ ] **Step 5: Run the tests**

Run: `cd tester && go test ./profile/`
Expected: `ok  tester/profile`.

- [ ] **Step 6: Commit**

```bash
git add tester/profile
git commit -m "$(cat <<'EOF'
feat: tester profile contract and id increments

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 3: CIDR allocation

**Files:**
- Create: `tester/profile/ipalloc.go`, `tester/profile/ipalloc_test.go`

**Interfaces:**
- Produces: `profile.AllocateIPs(cidr, start string, count int, exclude []netip.Addr) ([]netip.Addr, error)`. Its error text is shown to users verbatim as a field error, e.g. `10.0.1.0/29 from 10.0.1.2 has only 5 usable IPs, need 10 (short by 5)`.

- [ ] **Step 1: Write the failing test** `tester/profile/ipalloc_test.go`:

```go
package profile

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
)

func addrs(ss ...string) []netip.Addr {
	out := make([]netip.Addr, 0, len(ss))
	for _, s := range ss {
		out = append(out, netip.MustParseAddr(s))
	}
	return out
}

func TestAllocateIPsSequential(t *testing.T) {
	got, err := AllocateIPs("10.0.1.0/24", "10.0.1.100", 3, nil)
	require.NoError(t, err)
	require.Equal(t, addrs("10.0.1.100", "10.0.1.101", "10.0.1.102"), got)
}

func TestAllocateIPsSkipsNetworkBroadcastAndExcluded(t *testing.T) {
	got, err := AllocateIPs("10.0.1.0/29", "10.0.1.0", 4, addrs("10.0.1.1", "10.0.1.3"))
	require.NoError(t, err)
	// .0 network, .1 and .3 excluded, .7 broadcast never reached
	require.Equal(t, addrs("10.0.1.2", "10.0.1.4", "10.0.1.5", "10.0.1.6"), got)
}

func TestAllocateIPsReportsShortfall(t *testing.T) {
	_, err := AllocateIPs("10.0.1.0/29", "10.0.1.2", 10, addrs("10.0.1.1"))
	require.EqualError(t, err, "10.0.1.0/29 from 10.0.1.2 has only 5 usable IPs, need 10 (short by 5)")
}

func TestAllocateIPsSlash31And32UseEveryAddress(t *testing.T) {
	got, err := AllocateIPs("10.0.1.0/31", "10.0.1.0", 2, nil)
	require.NoError(t, err)
	require.Equal(t, addrs("10.0.1.0", "10.0.1.1"), got)

	got, err = AllocateIPs("10.0.1.9/32", "10.0.1.9", 1, nil)
	require.NoError(t, err)
	require.Equal(t, addrs("10.0.1.9"), got)
}

func TestAllocateIPsTopOfAddressSpaceTerminates(t *testing.T) {
	got, err := AllocateIPs("255.255.255.254/31", "255.255.255.254", 2, nil)
	require.NoError(t, err)
	require.Equal(t, addrs("255.255.255.254", "255.255.255.255"), got)
}

func TestAllocateIPsRejectsBadInput(t *testing.T) {
	_, err := AllocateIPs("10.0.1.0", "10.0.1.2", 1, nil)
	require.ErrorContains(t, err, "not an IPv4 CIDR")
	_, err = AllocateIPs("10.0.1.0/24", "10.0.2.2", 1, nil)
	require.EqualError(t, err, "start IP 10.0.2.2 is outside 10.0.1.0/24")
	_, err = AllocateIPs("fd00::/64", "fd00::1", 1, nil)
	require.ErrorContains(t, err, "not an IPv4 CIDR")
}
```

- [ ] **Step 2: Run it to make sure it fails**

Run: `cd tester && go test ./profile/ -run AllocateIPs`
Expected: FAIL (`undefined: AllocateIPs`).

- [ ] **Step 3: Implement** `tester/profile/ipalloc.go`:

```go
package profile

import (
	"fmt"
	"net/netip"
)

// AllocateIPs hands out count addresses from cidr, beginning at start and
// walking upward. It skips the network and broadcast addresses (for
// prefixes shorter than /31) and anything in exclude, which callers fill
// with the core-side IP and the IPs already configured on the host.
func AllocateIPs(cidr, start string, count int, exclude []netip.Addr) ([]netip.Addr, error) {
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil || !prefix.Addr().Is4() {
		return nil, fmt.Errorf("%q is not an IPv4 CIDR", cidr)
	}
	prefix = prefix.Masked()
	first, err := netip.ParseAddr(start)
	if err != nil || !first.Is4() {
		return nil, fmt.Errorf("%q is not an IPv4 address", start)
	}
	if !prefix.Contains(first) {
		return nil, fmt.Errorf("start IP %s is outside %s", first, prefix)
	}

	network, broadcast := prefix.Addr(), lastAddr(prefix)
	skipEdges := prefix.Bits() < 31
	excluded := make(map[netip.Addr]bool, len(exclude))
	for _, a := range exclude {
		excluded[a] = true
	}

	out := make([]netip.Addr, 0, count)
	for a := first; prefix.Contains(a) && len(out) < count; a = a.Next() {
		if skipEdges && (a == network || a == broadcast) {
			continue
		}
		if excluded[a] {
			continue
		}
		out = append(out, a)
		if a == broadcast {
			break
		}
	}
	if len(out) < count {
		return nil, fmt.Errorf("%s from %s has only %d usable IPs, need %d (short by %d)",
			prefix, first, len(out), count, count-len(out))
	}
	return out, nil
}

func lastAddr(p netip.Prefix) netip.Addr {
	b := p.Addr().As4()
	hostBits := 32 - p.Bits()
	v := uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
	v |= (1 << hostBits) - 1
	return netip.AddrFrom4([4]byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)})
}
```

- [ ] **Step 4: Run the tests**

Run: `cd tester && go test ./profile/`
Expected: `ok  tester/profile`. `TestAllocateIPsTopOfAddressSpaceTerminates` must return promptly, because `netip.Addr.Next()` past 255.255.255.255 is invalid and `Contains` then returns false.

- [ ] **Step 5: Commit**

```bash
git add tester/profile/ipalloc.go tester/profile/ipalloc_test.go
git commit -m "$(cat <<'EOF'
feat: allocate gnb ips from a cidr

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 4: Validate and expand a profile into a per-gNB plan

**Files:**
- Create: `tester/profile/expand.go`, `tester/profile/expand_test.go`

**Interfaces:**
- Consumes: `IncrementHex`, `RenderName` (Task 2), `AllocateIPs` (Task 3).
- Produces: `profile.Expand(p Profile, hostIPs []netip.Addr) (*Plan, error)`; `profile.Plan{Gnbs []GnbSpec; UesPerGnb, N2Prefix, N3Prefix int}`; `profile.GnbSpec{Index int; Name, GnbID, N2IP, N3IP string; UeCount, UeFirst, UeLast int}`. On bad input the error is a `*ValidationError` carrying every field problem at once, keyed by JSON path (`"network.n2.cidr"`).

- [ ] **Step 1: Write the failing test** `tester/profile/expand_test.go`. The last test pins Review Focus #3.

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
		Network: Network{
			N2: N2Network{Interface: "ens19", Cidr: "10.0.1.0/24", StartIP: "10.0.1.1", AmfIP: "10.0.1.1", AmfPort: 38412},
			N3: N3Network{Interface: "ens20", Cidr: "10.0.2.0/24", StartIP: "10.0.2.2", UpfIP: "10.0.2.1", UpfPort: 2152},
		},
		Rates: Rates{N2: StageRate{TimeoutMs: 5000, Retries: 1}},
	}
}

func TestExpandFillsGnbsInOrder(t *testing.T) {
	plan, err := Expand(sampleProfile(), []netip.Addr{netip.MustParseAddr("10.0.1.3")})
	require.NoError(t, err)
	require.Equal(t, 4, plan.UesPerGnb)
	require.Equal(t, 24, plan.N2Prefix)
	require.Equal(t, []GnbSpec{
		// .1 is the AMF, .3 is already on the host
		{Index: 1, Name: "gNB-1", GnbID: "000314", N2IP: "10.0.1.2", N3IP: "10.0.2.2", UeCount: 4, UeFirst: 1, UeLast: 4},
		{Index: 2, Name: "gNB-2", GnbID: "000315", N2IP: "10.0.1.4", N3IP: "10.0.2.3", UeCount: 4, UeFirst: 5, UeLast: 8},
		{Index: 3, Name: "gNB-3", GnbID: "000316", N2IP: "10.0.1.5", N3IP: "10.0.2.4", UeCount: 2, UeFirst: 9, UeLast: 10},
	}, plan.Gnbs)
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
```

- [ ] **Step 2: Run it to make sure it fails**

Run: `cd tester && go test ./profile/ -run Expand`
Expected: FAIL (`undefined: Expand`, `undefined: GnbSpec`).

- [ ] **Step 3: Implement** `tester/profile/expand.go`:

```go
package profile

import (
	"fmt"
	"net/netip"
	"regexp"
	"strings"
)

// GnbSpec is one concrete gNB a run will bring up. UeFirst/UeLast are the
// 1-based UE indexes it owns (both 0 when it owns none).
type GnbSpec struct {
	Index   int    `json:"index"`
	Name    string `json:"name"`
	GnbID   string `json:"gnbId"`
	N2IP    string `json:"n2Ip"`
	N3IP    string `json:"n3Ip"`
	UeCount int    `json:"ueCount"`
	UeFirst int    `json:"ueFirst"`
	UeLast  int    `json:"ueLast"`
}

// Plan is a profile expanded against a specific host.
type Plan struct {
	Gnbs      []GnbSpec `json:"gnbs"`
	UesPerGnb int       `json:"uesPerGnb"`
	N2Prefix  int       `json:"n2Prefix"`
	N3Prefix  int       `json:"n3Prefix"`
}

var (
	reMcc  = regexp.MustCompile(`^[0-9]{3}$`)
	reMnc  = regexp.MustCompile(`^[0-9]{2,3}$`)
	reHex6 = regexp.MustCompile(`^[0-9a-fA-F]{6}$`)
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
		if ues > 0 {
			spec.UeFirst, spec.UeLast = nextUe, nextUe+ues-1
			nextUe += ues
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
	validateEndpoint(verr, "network.n2", p.Network.N2.Interface, p.Network.N2.Cidr, p.Network.N2.StartIP, "amfIp", p.Network.N2.AmfIP, "amfPort", p.Network.N2.AmfPort)
	validateEndpoint(verr, "network.n3", p.Network.N3.Interface, p.Network.N3.Cidr, p.Network.N3.StartIP, "upfIp", p.Network.N3.UpfIP, "upfPort", p.Network.N3.UpfPort)
	if p.Rates.N2.TimeoutMs < 1 {
		verr.add("rates.n2.timeoutMs", "must be at least 1")
	}
	if p.Rates.N2.Retries < 0 {
		verr.add("rates.n2.retries", "must not be negative")
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

- [ ] **Step 4: Run the tests**

Run: `cd tester && go test ./profile/ && golangci-lint run ./profile/`
Expected: `ok  tester/profile`, `0 issues.`

- [ ] **Step 5: Commit**

```bash
git add tester/profile/expand.go tester/profile/expand_test.go
git commit -m "$(cat <<'EOF'
feat: validate and expand tester profile into gnb plan

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 5: Stage metrics

**Files:**
- Create: `tester/metrics/histogram.go`, `tester/metrics/histogram_test.go`, `tester/metrics/stage.go`, `tester/metrics/stage_test.go`

**Interfaces:**
- Produces: `metrics.Outcome` (`Accepted`, `Rejected`, `TimedOut`, `Failed`); `metrics.NewStage(name string, expected int) *Stage`; `(*Stage).Begin(retry bool)`, `Retrying()`, `Finish(o Outcome, latency time.Duration, cause string)`, `Snapshot() StageSnapshot`; `metrics.StageSnapshot` (JSON fields listed in the code; Task 14 mirrors them as `TesterStageSnapshot`); `metrics.CauseCount{Cause string; Count int64}`.

Semantics the run controller relies on:
- Call `Begin(false)` on an item's first attempt and `Begin(true)` on each retry.
- A failed attempt that will be retried calls `Retrying()`. It only decrements in-flight and is not an outcome.
- Every item calls `Finish` exactly once, with latency measured from its *first* attempt.
- `Done` is true once the number of finished items equals `expected`, and from then on total time stops growing.

- [ ] **Step 1: Write the failing tests.** `tester/metrics/histogram_test.go`:

```go
package metrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHistogramQuantilesWithinResolution(t *testing.T) {
	var h histogram
	for i := 1; i <= 1000; i++ {
		h.record(time.Duration(i) * time.Millisecond)
	}
	require.InEpsilon(t, 500.0, ms(h.quantile(0.50)), 0.05)
	require.InEpsilon(t, 950.0, ms(h.quantile(0.95)), 0.05)
	require.InEpsilon(t, 990.0, ms(h.quantile(0.99)), 0.05)
	require.Equal(t, time.Second, h.quantile(1.0))
	require.Equal(t, 500500*time.Microsecond, h.mean())
}

func TestHistogramEdges(t *testing.T) {
	var h histogram
	require.Equal(t, time.Duration(0), h.quantile(0.5))
	require.Equal(t, time.Duration(0), h.mean())
	h.record(0)
	h.record(time.Hour * 24 * 365)
	require.Equal(t, time.Hour*24*365, h.max)
	// sub-microsecond samples share bucket 0, reported as its 1µs bound
	require.Equal(t, time.Microsecond, h.quantile(0.5))
}
```

`tester/metrics/stage_test.go`:

```go
package metrics

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time          { return c.t }
func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

func TestStageCountsOutcomesAndTotalTime(t *testing.T) {
	clk := &fakeClock{t: time.Unix(1000, 0)}
	s := NewStage("n2", 4)
	s.now = clk.now

	s.Begin(false)
	s.Begin(false)
	s.Begin(false)
	s.Begin(false)
	clk.advance(10 * time.Millisecond)
	s.Finish(Accepted, 10*time.Millisecond, "")
	s.Finish(Rejected, 0, "misc(4)")
	s.Retrying()
	s.Begin(true)

	snap := s.Snapshot()
	require.Equal(t, int64(4), snap.Attempted)
	require.Equal(t, int64(1), snap.Retries)
	require.Equal(t, int64(2), snap.InFlight)
	require.False(t, snap.Done)
	require.InDelta(t, 10.0, snap.TotalTimeMs, 0.001)

	clk.advance(20 * time.Millisecond)
	s.Finish(TimedOut, 0, "")
	s.Finish(Failed, 0, "sctp: connection refused")
	clk.advance(time.Hour) // after Done, total time must stop growing

	snap = s.Snapshot()
	require.True(t, snap.Done)
	require.Equal(t, int64(0), snap.InFlight)
	require.Equal(t, []int64{1, 1, 1, 1}, []int64{snap.Accepted, snap.Rejected, snap.TimedOut, snap.Failed})
	require.InDelta(t, 30.0, snap.TotalTimeMs, 0.001)
	require.InDelta(t, 10.0, snap.MaxMs, 0.001)
	require.Equal(t, []CauseCount{{"misc(4)", 1}, {"sctp: connection refused", 1}, {"unknown", 1}}, snap.Causes)
}

func TestStageEmptySnapshot(t *testing.T) {
	snap := NewStage("n2", 3).Snapshot()
	require.Equal(t, 0.0, snap.TotalTimeMs)
	require.False(t, snap.Done)
	require.NotNil(t, snap.Causes)
}

func TestStageConcurrentUse(t *testing.T) {
	s := NewStage("n2", 1000)
	var wg sync.WaitGroup
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Begin(false)
			_ = s.Snapshot()
			s.Finish(Accepted, time.Millisecond, "")
		}()
	}
	wg.Wait()
	snap := s.Snapshot()
	require.Equal(t, int64(1000), snap.Accepted)
	require.True(t, snap.Done)
}
```

- [ ] **Step 2: Run them to make sure they fail**

Run: `cd tester && go test ./metrics/`
Expected: FAIL (`undefined: histogram`, `undefined: NewStage`).

- [ ] **Step 3: Implement.** `tester/metrics/histogram.go`:

```go
// Package metrics counts stage outcomes and latencies with fixed memory,
// so the same code serves 10 gNBs today and 100k UEs later.
package metrics

import (
	"math"
	"time"
)

// bucketsPerOctave sets resolution: each power of two is split into 16
// buckets, so a reported percentile is within ~4.4% of the true value.
const (
	bucketsPerOctave = 16
	maxOctaves       = 40 // 2^40 µs ≈ 12.7 days, far beyond any timeout
	numBuckets       = bucketsPerOctave*maxOctaves + 1
)

// histogram is not safe for concurrent use; Stage guards it.
type histogram struct {
	counts [numBuckets]uint64
	total  uint64
	sum    time.Duration
	max    time.Duration
}

func bucketOf(d time.Duration) int {
	us := float64(d) / float64(time.Microsecond)
	if us < 1 {
		return 0
	}
	i := int(math.Log2(us)*bucketsPerOctave) + 1
	return min(i, numBuckets-1)
}

// upperBound is the largest duration that lands in bucket i.
func upperBound(i int) time.Duration {
	if i == 0 {
		return time.Microsecond
	}
	return time.Duration(math.Exp2(float64(i)/bucketsPerOctave) * float64(time.Microsecond))
}

func (h *histogram) record(d time.Duration) {
	h.counts[bucketOf(d)]++
	h.total++
	h.sum += d
	h.max = max(h.max, d)
}

// quantile returns the bucket upper bound below which q of the samples
// fall, capped at the true max so p100 is exact.
func (h *histogram) quantile(q float64) time.Duration {
	if h.total == 0 {
		return 0
	}
	rank := uint64(math.Ceil(q * float64(h.total)))
	rank = max(rank, 1)
	var seen uint64
	for i, c := range h.counts {
		seen += c
		if seen >= rank {
			return min(upperBound(i), h.max)
		}
	}
	return h.max
}

func (h *histogram) mean() time.Duration {
	if h.total == 0 {
		return 0
	}
	return h.sum / time.Duration(h.total)
}
```

`tester/metrics/stage.go`:

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
		Done:      int(finished) == s.expected,
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

- [ ] **Step 4: Run the tests with the race detector**

Run: `cd tester && go test -race ./metrics/`
Expected: `ok  tester/metrics`.

- [ ] **Step 5: Commit**

```bash
git add tester/metrics
git commit -m "$(cat <<'EOF'
feat: stage counters and latency histogram

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 6: gNB identity and NG Setup exchange

**Files:**
- Create: `tester/gnb/identity.go`, `tester/gnb/ngsetup.go`, `tester/gnb/ngsetup_test.go`

**Interfaces:**
- Consumes: `profile.GnbSpec`, `profile.GnbTemplate`.
- Produces: `gnb.NewIdentity(spec profile.GnbSpec, tpl profile.GnbTemplate) (Identity, error)`; `(Identity).NGSetupRequest() ([]byte, error)`; `gnb.ExchangeNGSetup(conn io.ReadWriter, request []byte) error`; `*gnb.RejectedError{Cause string}` (its `Error()` is `"ng setup rejected: " + Cause`); `gnb.DescribeCause(*ie.Cause) string`.

Background: the NG Setup Request matches free-ran-ue's `gnb/ngapBuilder.go` `buildNgapSetupRequest`, copied rather than imported. `free5gc/sctp`'s `SCTPConn.SetReadDeadline` returns `EOPNOTSUPP`, so `ExchangeNGSetup` sets no deadline itself. The timeout comes from `SO_RCVTIMEO`, which the dialer in Task 7 sets, and a timed-out read surfaces as `syscall.EAGAIN`.

- [ ] **Step 1: Write the failing test** `tester/gnb/ngsetup_test.go`. It drives a fake AMF over `net.Pipe`.

```go
package gnb

import (
	"bytes"
	"errors"
	"net"
	"syscall"
	"testing"

	"github.com/free5gc/ngap/aper"
	"github.com/free5gc/ngap/ie"
	"github.com/free5gc/ngap/message"
	"github.com/stretchr/testify/require"

	"tester/profile"
)

func testIdentity(t *testing.T) Identity {
	t.Helper()
	id, err := NewIdentity(
		profile.GnbSpec{Index: 1, Name: "gNB-1", GnbID: "000314"},
		profile.GnbTemplate{Mcc: "208", Mnc: "93", Tac: "000001", Sst: 1, Sd: "010203"},
	)
	require.NoError(t, err)
	return id
}

func ngSetupResponse(t *testing.T, id Identity) []byte {
	t.Helper()
	plmn, snssai := id.PlmnID, id.Snssai
	b, err := (&message.NGSetupResponse{
		AMFName: &ie.AMFName{Value: "AMF"},
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
	require.NoError(t, err)
	return b
}

// fakeAMF answers one NG Setup Request on its end of a net.Pipe.
func fakeAMF(t *testing.T, reply []byte) net.Conn {
	t.Helper()
	gnbEnd, amfEnd := net.Pipe()
	go func() {
		defer func() { _ = amfEnd.Close() }()
		buf := make([]byte, 4096)
		n, err := amfEnd.Read(buf)
		if err != nil {
			return
		}
		if _, err := message.Parse(buf[:n]); err != nil {
			return
		}
		_, _ = amfEnd.Write(reply)
	}()
	t.Cleanup(func() { _ = gnbEnd.Close() })
	return gnbEnd
}

func TestNGSetupRequestRoundTrips(t *testing.T) {
	id := testIdentity(t)
	raw, err := id.NGSetupRequest()
	require.NoError(t, err)
	msg, err := message.Parse(raw)
	require.NoError(t, err)
	req, ok := msg.(*message.NGSetupRequest)
	require.True(t, ok)
	require.Equal(t, "gNB-1", string(req.RANNodeName.Value))
	gnbID := req.GlobalRANNodeID.Choice.(*ie.GlobalGNBID).GNBID.Choice.(*ie.GNBIDForGNBID).Value
	require.Equal(t, []byte{0x00, 0x03, 0x14}, gnbID.Bytes)
	require.Equal(t, uint64(24), gnbID.BitLength)
}

func TestExchangeNGSetupAccepted(t *testing.T) {
	id := testIdentity(t)
	req, err := id.NGSetupRequest()
	require.NoError(t, err)
	require.NoError(t, ExchangeNGSetup(fakeAMF(t, ngSetupResponse(t, id)), req))
}

func TestExchangeNGSetupRejectedCarriesCause(t *testing.T) {
	id := testIdentity(t)
	req, err := id.NGSetupRequest()
	require.NoError(t, err)
	failure, err := (&message.NGSetupFailure{
		Cause: &ie.Cause{Choice: &ie.CauseMisc{Value: ie.CauseMiscPresentUnknownPLMNOrSNPN}},
	}).MarshalBinary()
	require.NoError(t, err)

	err = ExchangeNGSetup(fakeAMF(t, failure), req)
	var rejected *RejectedError
	require.ErrorAs(t, err, &rejected)
	require.Equal(t, "misc(4)", rejected.Cause)
	require.EqualError(t, err, "ng setup rejected: misc(4)")
}

func TestExchangeNGSetupGarbageReply(t *testing.T) {
	id := testIdentity(t)
	req, err := id.NGSetupRequest()
	require.NoError(t, err)
	err = ExchangeNGSetup(fakeAMF(t, []byte{0xde, 0xad}), req)
	require.ErrorContains(t, err, "decode ng setup response")
	var rejected *RejectedError
	require.False(t, errors.As(err, &rejected))
}

// timeoutConn behaves like an SCTP socket whose SO_RCVTIMEO expired.
type timeoutConn struct{ bytes.Buffer }

func (c *timeoutConn) Read([]byte) (int, error) { return 0, syscall.EAGAIN }

func TestExchangeNGSetupReadTimeoutKeepsErrno(t *testing.T) {
	id := testIdentity(t)
	req, err := id.NGSetupRequest()
	require.NoError(t, err)
	err = ExchangeNGSetup(&timeoutConn{}, req)
	require.ErrorIs(t, err, syscall.EAGAIN)
}

func TestDescribeCause(t *testing.T) {
	require.Equal(t, "unknown", DescribeCause(nil))
	require.Equal(t, "radioNetwork(0)", DescribeCause(&ie.Cause{Choice: &ie.CauseRadioNetwork{Value: 0}}))
}
```

- [ ] **Step 2: Run it to make sure it fails**

Run: `cd tester && go test ./gnb/`
Expected: FAIL (`undefined: NewIdentity`, `undefined: ExchangeNGSetup`).

- [ ] **Step 3: Implement.** `tester/gnb/identity.go`:

```go
package gnb

import (
	"encoding/hex"
	"fmt"

	"github.com/free-ran-ue/util"
	"github.com/free5gc/ngap/aper"
	"github.com/free5gc/ngap/ie"
	"github.com/free5gc/ngap/message"
	"github.com/free5gc/openapi/models"

	"tester/profile"
)

// Identity is the NGAP-encoded form of one gNB's settings.
type Identity struct {
	GnbID  []byte
	Name   string
	PlmnID ie.PLMNIdentity
	Tai    ie.TAI
	Snssai ie.SNSSAI
}

// NewIdentity encodes spec + tpl. The profile package has already
// validated the formats; errors here mean a caller skipped profile.Expand.
func NewIdentity(spec profile.GnbSpec, tpl profile.GnbTemplate) (Identity, error) {
	gnbID, err := hex.DecodeString(spec.GnbID)
	if err != nil {
		return Identity{}, fmt.Errorf("gnb id %q: %w", spec.GnbID, err)
	}
	plmn := models.PlmnId{Mcc: tpl.Mcc, Mnc: tpl.Mnc}
	plmnID, err := util.PlmnIdToNgap(plmn)
	if err != nil {
		return Identity{}, fmt.Errorf("plmn %s-%s: %w", tpl.Mcc, tpl.Mnc, err)
	}
	tai, err := util.TaiToNgap(models.Tai{PlmnId: &plmn, Tac: tpl.Tac})
	if err != nil {
		return Identity{}, fmt.Errorf("tac %q: %w", tpl.Tac, err)
	}
	snssai, err := util.SNssaiToNgap(models.Snssai{Sst: int32(tpl.Sst), Sd: tpl.Sd})
	if err != nil {
		return Identity{}, fmt.Errorf("snssai %d-%s: %w", tpl.Sst, tpl.Sd, err)
	}
	return Identity{GnbID: gnbID, Name: spec.Name, PlmnID: plmnID, Tai: tai, Snssai: snssai}, nil
}

// NGSetupRequest encodes the NG Setup Request for this gNB, matching the
// one free-ran-ue sends (gnb/ngapBuilder.go buildNgapSetupRequest).
func (id Identity) NGSetupRequest() ([]byte, error) {
	plmnID, snssai := id.PlmnID, id.Snssai
	req := &message.NGSetupRequest{
		GlobalRANNodeID: &ie.GlobalRANNodeID{
			Choice: &ie.GlobalGNBID{
				PLMNIdentity: &plmnID,
				GNBID: &ie.GNBID{
					Choice: &ie.GNBIDForGNBID{
						Value: aper.BitString{Bytes: id.GnbID, BitLength: uint64(len(id.GnbID) * 8)},
					},
				},
			},
		},
		RANNodeName: &ie.RANNodeName{Value: aper.PrintableString(id.Name)},
		SupportedTAList: &ie.SupportedTAList{
			List: []ie.SupportedTAItem{{
				TAC: id.Tai.TAC,
				BroadcastPLMNList: &ie.BroadcastPLMNList{
					List: []ie.BroadcastPLMNItem{{
						PLMNIdentity:        id.Tai.PLMNIdentity,
						TAISliceSupportList: &ie.SliceSupportList{List: []ie.SliceSupportItem{{SNSSAI: &snssai}}},
					}},
				},
			}},
		},
		DefaultPagingDRX: &ie.PagingDRX{Value: ie.PagingDRXPresentV128},
	}
	return req.MarshalBinary()
}
```

`tester/gnb/ngsetup.go`:

```go
// Package gnb holds what one simulated gNB needs on N2: its NGAP identity,
// the SCTP association to the AMF, and the NG Setup exchange.
package gnb

import (
	"fmt"
	"io"

	"github.com/free5gc/ngap/ie"
	"github.com/free5gc/ngap/message"
)

// RejectedError is returned when the AMF answers with NGSetupFailure.
type RejectedError struct {
	Cause string // e.g. "misc(4)", see DescribeCause
}

func (e *RejectedError) Error() string { return "ng setup rejected: " + e.Cause }

// ExchangeNGSetup sends request and waits for the AMF's answer. The read
// timeout comes from the connection itself (SO_RCVTIMEO on SCTP, see
// SCTPDialer), because free5gc/sctp does not support read deadlines.
func ExchangeNGSetup(conn io.ReadWriter, request []byte) error {
	if _, err := conn.Write(request); err != nil {
		return fmt.Errorf("send ng setup request: %w", err)
	}
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return fmt.Errorf("read ng setup response: %w", err)
	}
	msg, err := message.Parse(buf[:n])
	if err != nil {
		return fmt.Errorf("decode ng setup response: %w", err)
	}
	switch m := msg.(type) {
	case *message.NGSetupResponse:
		return nil
	case *message.NGSetupFailure:
		return &RejectedError{Cause: DescribeCause(m.Cause)}
	default:
		return fmt.Errorf("unexpected %T in reply to ng setup request", msg)
	}
}

// DescribeCause renders an NGAP cause as "group(value)", e.g. "misc(4)",
// which is what the UI groups failures by.
func DescribeCause(c *ie.Cause) string {
	if c == nil || c.Choice == nil {
		return "unknown"
	}
	switch v := c.Choice.(type) {
	case *ie.CauseRadioNetwork:
		return fmt.Sprintf("radioNetwork(%d)", v.Value)
	case *ie.CauseTransport:
		return fmt.Sprintf("transport(%d)", v.Value)
	case *ie.CauseNas:
		return fmt.Sprintf("nas(%d)", v.Value)
	case *ie.CauseProtocol:
		return fmt.Sprintf("protocol(%d)", v.Value)
	case *ie.CauseMisc:
		return fmt.Sprintf("misc(%d)", v.Value)
	default:
		return fmt.Sprintf("%T", v)
	}
}
```

- [ ] **Step 4: Add dependencies and run the tests**

Run:
```bash
cd tester
go get github.com/free5gc/ngap@v1.2.0 github.com/free5gc/openapi@v1.3.0 github.com/free-ran-ue/util@v0.2.0
go mod tidy
go test ./gnb/
```
Expected: `ok  tester/gnb`.

- [ ] **Step 5: Commit**

```bash
git add tester/gnb tester/go.mod tester/go.sum
git commit -m "$(cat <<'EOF'
feat: gnb ngap identity and ng setup exchange

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 7: SCTP dialer

**Files:**
- Create: `tester/gnb/dial.go`, `tester/gnb/dial_test.go`

**Interfaces:**
- Produces: `gnb.Conn` (= `io.ReadWriteCloser`); `gnb.Dialer` interface `Dial(localIP, amfIP string, amfPort int, timeout time.Duration) (Conn, error)`; `gnb.SCTPDialer{}` implements it.

The local port is 0, so the kernel picks one. That keeps gNBs from colliding even if two of them share an IP. `SO_SNDTIMEO` bounds the blocking connect and `SO_RCVTIMEO` bounds every read, both set through `sctp.SocketConfig.Control` before connect. The NGAP PPID `0x3c000000` is set as the default send parameter, the same way free-ran-ue does in `gnb/gnb.go connectToAmf`.

- [ ] **Step 1: Write the test** `tester/gnb/dial_test.go`. It needs the kernel `sctp` module, so it is opt-in:

```go
package gnb

import (
	"net"
	"os"
	"testing"
	"time"

	"github.com/free5gc/sctp"
	"github.com/stretchr/testify/require"
)

// Needs the sctp kernel module: FRU_TESTER_SCTP=1 go test ./gnb/ -run SCTP
func TestSCTPDialerLoopback(t *testing.T) {
	if os.Getenv("FRU_TESTER_SCTP") != "1" {
		t.Skip("set FRU_TESTER_SCTP=1 (and `sudo modprobe sctp`) to run")
	}
	ln, err := sctp.ListenSCTP("sctp", &sctp.SCTPAddr{IPAddrs: []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, Port: 0})
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()
	port := ln.Addr().(*sctp.SCTPAddr).Port

	go func() {
		c, err := ln.AcceptSCTP(-1)
		if err != nil {
			return
		}
		defer func() { _ = c.Close() }()
		time.Sleep(2 * time.Second) // never answer
	}()

	conn, err := SCTPDialer{}.Dial("127.0.0.1", "127.0.0.1", port, 300*time.Millisecond)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	start := time.Now()
	_, err = conn.Read(make([]byte, 16))
	require.Error(t, err)
	require.Less(t, time.Since(start), time.Second, "SO_RCVTIMEO should end the read")
}
```

- [ ] **Step 2: Run it to make sure it fails**

Run: `cd tester && sudo modprobe sctp && FRU_TESTER_SCTP=1 go test ./gnb/ -run SCTP`
Expected: FAIL (`undefined: SCTPDialer`).

- [ ] **Step 3: Implement** `tester/gnb/dial.go`:

```go
package gnb

import (
	"fmt"
	"io"
	"net"
	"syscall"
	"time"

	"github.com/free5gc/sctp"
)

// ngapPPID is the SCTP payload protocol identifier for NGAP (TS 38.412).
const ngapPPID uint32 = 0x3c000000

// Conn is one gNB's N2 association.
type Conn = io.ReadWriteCloser

// Dialer opens an N2 association from localIP to the AMF. timeout bounds
// both the connect and every later blocking read on the returned Conn.
type Dialer interface {
	Dial(localIP, amfIP string, amfPort int, timeout time.Duration) (Conn, error)
}

// SCTPDialer is the real Dialer. The local port is 0 (kernel-chosen), so
// gNBs never collide on ports even when they share an IP.
type SCTPDialer struct{}

func (SCTPDialer) Dial(localIP, amfIP string, amfPort int, timeout time.Duration) (Conn, error) {
	local, err := sctpAddr(localIP, 0)
	if err != nil {
		return nil, err
	}
	remote, err := sctpAddr(amfIP, amfPort)
	if err != nil {
		return nil, err
	}
	tv := syscall.NsecToTimeval(timeout.Nanoseconds())
	cfg := sctp.SocketConfig{
		InitMsg: sctp.InitMsg{NumOstreams: sctp.SCTP_MAX_STREAM},
		// SO_SNDTIMEO bounds the blocking connect; SO_RCVTIMEO bounds
		// reads, since SCTPConn.SetReadDeadline returns EOPNOTSUPP.
		Control: func(_, _ string, c syscall.RawConn) error {
			var serr error
			if err := c.Control(func(fd uintptr) {
				if serr = syscall.SetsockoptTimeval(int(fd), syscall.SOL_SOCKET, syscall.SO_SNDTIMEO, &tv); serr != nil {
					return
				}
				serr = syscall.SetsockoptTimeval(int(fd), syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &tv)
			}); err != nil {
				return err
			}
			return serr
		},
	}
	conn, err := cfg.Dial("sctp", local, remote)
	if err != nil {
		if conn != nil {
			_ = conn.Close()
		}
		return nil, fmt.Errorf("sctp connect %s -> %s:%d: %w", localIP, amfIP, amfPort, err)
	}
	info, err := conn.GetDefaultSentParam()
	if err == nil {
		info.PPID = ngapPPID
		err = conn.SetDefaultSentParam(info)
	}
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("set ngap ppid: %w", err)
	}
	return conn, nil
}

func sctpAddr(ip string, port int) (*sctp.SCTPAddr, error) {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return nil, fmt.Errorf("%q is not an IP address", ip)
	}
	return &sctp.SCTPAddr{IPAddrs: []net.IPAddr{{IP: parsed}}, Port: port}, nil
}
```

- [ ] **Step 4: Run the tests**

Run: `cd tester && go get github.com/free5gc/sctp@v1.2.0 && go mod tidy && FRU_TESTER_SCTP=1 go test ./gnb/ -v -run SCTP && go test ./gnb/`
Expected: `--- PASS: TestSCTPDialerLoopback` (well under 1 s), then `ok  tester/gnb` (the loopback test skips without the env var).

- [ ] **Step 5: Commit**

```bash
git add tester/gnb/dial.go tester/gnb/dial_test.go tester/go.mod tester/go.sum
git commit -m "$(cat <<'EOF'
feat: sctp dialer with connect and read timeouts

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 8: Host address management (netlink)

**Files:**
- Create: `tester/netcfg/netcfg.go`, `tester/netcfg/netcfg_test.go`

**Interfaces:**
- Produces: `netcfg.AddrManager` interface `{ HostIPv4s() ([]netip.Addr, error); Interfaces() ([]string, error); Add(iface string, addr netip.Prefix) error; Remove(iface string, addr netip.Prefix) error }`; `netcfg.Netlink{}` implements it.

Why `Remove` treats a missing address as success: when an interface has no IP in the CIDR yet, the first gNB IP becomes the *primary* address. The kernel deletes secondaries together with their primary. The run controller removes in reverse order to avoid that, but `Remove` must still be idempotent.

- [ ] **Step 1: Write the test** `tester/netcfg/netcfg_test.go`. It creates and deletes a throwaway dummy link and needs root:

```go
package netcfg

import (
	"net/netip"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
)

// Needs root: sudo FRU_TESTER_NETLINK=1 go test ./netcfg/
func TestNetlinkAddRemoveOnDummyLink(t *testing.T) {
	if os.Getenv("FRU_TESTER_NETLINK") != "1" {
		t.Skip("set FRU_TESTER_NETLINK=1 and run as root to exercise netlink")
	}
	link := &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: "frutest0"}}
	require.NoError(t, netlink.LinkAdd(link))
	t.Cleanup(func() { _ = netlink.LinkDel(link) })

	m := Netlink{}
	names, err := m.Interfaces()
	require.NoError(t, err)
	require.Contains(t, names, "frutest0")

	primary := netip.MustParsePrefix("10.250.0.2/24")
	secondary := netip.MustParsePrefix("10.250.0.3/24")
	require.NoError(t, m.Add("frutest0", primary))
	require.NoError(t, m.Add("frutest0", secondary))

	ips, err := m.HostIPv4s()
	require.NoError(t, err)
	require.Contains(t, ips, primary.Addr())
	require.Contains(t, ips, secondary.Addr())

	// removing the primary may take the secondary with it; the second
	// Remove must still succeed.
	require.NoError(t, m.Remove("frutest0", primary))
	require.NoError(t, m.Remove("frutest0", secondary))
	require.NoError(t, m.Remove("frutest0", secondary))

	require.ErrorContains(t, m.Add("no-such-if0", primary), `interface "no-such-if0"`)
}
```

- [ ] **Step 2: Run it to make sure it fails**

Run: `cd tester && go test ./netcfg/`
Expected: FAIL (`undefined: Netlink`).

- [ ] **Step 3: Implement** `tester/netcfg/netcfg.go`:

```go
// Package netcfg adds and removes the per-gNB IPs on host interfaces.
package netcfg

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"syscall"

	"github.com/vishvananda/netlink"
)

// AddrManager is what a run needs from the host network stack. The run
// package depends on this interface so tests can use a fake.
type AddrManager interface {
	// HostIPv4s lists every IPv4 address already on any interface.
	HostIPv4s() ([]netip.Addr, error)
	// Interfaces lists the names of every link on the host.
	Interfaces() ([]string, error)
	Add(iface string, addr netip.Prefix) error
	// Remove must treat an already-missing address as success, because the
	// kernel drops secondary addresses when their primary is removed.
	Remove(iface string, addr netip.Prefix) error
}

// Netlink is the real AddrManager; it needs CAP_NET_ADMIN.
type Netlink struct{}

func (Netlink) HostIPv4s() ([]netip.Addr, error) {
	list, err := netlink.AddrList(nil, netlink.FAMILY_V4)
	if err != nil {
		return nil, fmt.Errorf("list host addresses: %w", err)
	}
	out := make([]netip.Addr, 0, len(list))
	for _, a := range list {
		if ip, ok := netip.AddrFromSlice(a.IP.To4()); ok {
			out = append(out, ip)
		}
	}
	return out, nil
}

func (Netlink) Interfaces() ([]string, error) {
	links, err := netlink.LinkList()
	if err != nil {
		return nil, fmt.Errorf("list host interfaces: %w", err)
	}
	names := make([]string, 0, len(links))
	for _, l := range links {
		names = append(names, l.Attrs().Name)
	}
	return names, nil
}

func (Netlink) Add(iface string, addr netip.Prefix) error {
	link, err := netlink.LinkByName(iface)
	if err != nil {
		return fmt.Errorf("interface %q: %w", iface, err)
	}
	if err := netlink.AddrAdd(link, toNetlink(addr)); err != nil {
		return fmt.Errorf("add %s to %s: %w", addr, iface, err)
	}
	return nil
}

func (Netlink) Remove(iface string, addr netip.Prefix) error {
	link, err := netlink.LinkByName(iface)
	if err != nil {
		return fmt.Errorf("interface %q: %w", iface, err)
	}
	if err := netlink.AddrDel(link, toNetlink(addr)); err != nil {
		if errors.Is(err, syscall.EADDRNOTAVAIL) {
			return nil
		}
		return fmt.Errorf("remove %s from %s: %w", addr, iface, err)
	}
	return nil
}

func toNetlink(p netip.Prefix) *netlink.Addr {
	ip := p.Addr().As4()
	return &netlink.Addr{IPNet: &net.IPNet{
		IP:   net.IP(ip[:]),
		Mask: net.CIDRMask(p.Bits(), 32),
	}}
}
```

- [ ] **Step 4: Run the tests as root**

Run:
```bash
cd tester
go get github.com/vishvananda/netlink@v1.3.1 && go mod tidy
go test -c -o /tmp/netcfg.test ./netcfg/
sudo FRU_TESTER_NETLINK=1 /tmp/netcfg.test -test.v
ip link show frutest0   # must say: Device "frutest0" does not exist.
```
Expected: `--- PASS: TestNetlinkAddRemoveOnDummyLink`, and no `frutest0` left behind.

- [ ] **Step 5: Commit**

```bash
git add tester/netcfg tester/go.mod tester/go.sum
git commit -m "$(cat <<'EOF'
feat: add and remove gnb ips with netlink

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 9: Run controller

**Files:**
- Create: `tester/run/snapshot.go`, `tester/run/classify.go`, `tester/run/controller.go`, `tester/run/controller_test.go`, `tester/run/ngap_fixtures_test.go`

**Interfaces:**
- Consumes: `profile.Expand`/`Plan`/`GnbSpec`/`ValidationError` (Tasks 2–4), `metrics.Stage` (Task 5), `gnb.NewIdentity`/`ExchangeNGSetup`/`RejectedError`/`Dialer`/`Conn` (Tasks 6–7), `netcfg.AddrManager` (Task 8).
- Produces:
  - `run.NewController(run.Deps{Addrs, Dialer, Log, Now, NewID}) *Controller`. `Now` and `NewID` are optional.
  - `(*Controller).Validate(profile.Profile) (*profile.Plan, error)`, `Start(profile.Profile) (Snapshot, error)`, `Stop() (Snapshot, error)`, `Snapshot() Snapshot`, `Changed() <-chan struct{}`, `Shutdown()`.
  - Errors `run.ErrRunActive`, `run.ErrNotRunning`.
  - `run.Snapshot` and `run.GnbStatus` (JSON contract mirrored in Task 14), plus the `State`/`GnbState` constants.
  - `run.Classify(error) metrics.Outcome` and `run.CauseOf(error) string`.

Lifecycle: `configuring` (add one N2 IP per gNB) → `n2` (all gNBs attempt NG Setup concurrently; a failure with retries left is requeued at the back) → `running` (holding associations; `watchConn` flags `lost` ones) → on Stop: `stopping` (close conns, remove IPs in reverse) → `stopped`. If an IP cannot be added, the run rolls back and ends `failed`, and nothing is attempted.

- [ ] **Step 1: Write the failing tests.** `tester/run/ngap_fixtures_test.go`:

```go
package run

import (
	"testing"

	"github.com/free5gc/ngap/aper"
	"github.com/free5gc/ngap/ie"
	"github.com/free5gc/ngap/message"
	"github.com/stretchr/testify/require"
)

func ngSetupResponseBytes(t *testing.T) []byte {
	t.Helper()
	plmn := ie.PLMNIdentity{Value: aper.OctetString{0x02, 0xf8, 0x39}}
	snssai := ie.SNSSAI{SST: &ie.SST{Value: aper.OctetString{1}}}
	b, err := (&message.NGSetupResponse{
		AMFName: &ie.AMFName{Value: "AMF"},
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
	require.NoError(t, err)
	return b
}

func ngSetupFailureBytes(t *testing.T) []byte {
	t.Helper()
	b, err := (&message.NGSetupFailure{
		Cause: &ie.Cause{Choice: &ie.CauseMisc{Value: ie.CauseMiscPresentUnknownPLMNOrSNPN}},
	}).MarshalBinary()
	require.NoError(t, err)
	return b
}
```

`tester/run/controller_test.go`. `TestValidateFlagsUnknownInterface`, `TestStopWhileConfiguringSkipsN2` and `TestLostAssociationIsReported` pin Review Focus #2, #1 and #4.

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
	select {
	case <-c.closed:
		return 0, net.ErrClosed
	case <-c.drop:
		return 0, io.EOF
	}
}
func (c *fakeConn) Close() error { c.once.Do(func() { close(c.closed) }); return nil }

// fakeDialer: script[localIP] returns per-attempt behaviour.
type fakeDialer struct {
	mu     sync.Mutex
	script func(localIP string, attempt int) (reply func() ([]byte, error), dialErr error)
	counts map[string]int
	opened []*fakeConn
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
	c := &fakeConn{reply: reply, closed: make(chan struct{}), drop: make(chan struct{})}
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
```

- [ ] **Step 2: Run them to make sure they fail**

Run: `cd tester && go test ./run/`
Expected: FAIL (`undefined: Controller`, `undefined: NewController`, …).

- [ ] **Step 3: Implement.** `tester/run/snapshot.go`:

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
	StateRunning     State = "running"     // N2 finished; holding associations
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
	State     GnbState `json:"state"`
	Attempts  int      `json:"attempts"`
	LatencyMs float64  `json:"latencyMs"`
	Cause     string   `json:"cause"`
}

// Snapshot is everything the run page shows; it is what GET /api/run and
// every WebSocket frame carry.
type Snapshot struct {
	RunID       string                `json:"runId"`
	ProfileName string                `json:"profileName"`
	State       State                 `json:"state"`
	Error       string                `json:"error"`
	StartedAt   *time.Time            `json:"startedAt"`
	StoppedAt   *time.Time            `json:"stoppedAt"`
	N2          metrics.StageSnapshot `json:"n2"`
	Gnbs        []GnbStatus           `json:"gnbs"`
}
```

`tester/run/classify.go`:

```go
package run

import (
	"errors"
	"os"
	"syscall"

	"tester/gnb"
	"tester/metrics"
)

// Classify maps an N2 attempt error to the outcome the stage card counts.
func Classify(err error) metrics.Outcome {
	var rejected *gnb.RejectedError
	switch {
	case err == nil:
		return metrics.Accepted
	case errors.As(err, &rejected):
		return metrics.Rejected
	case errors.Is(err, syscall.EAGAIN), errors.Is(err, syscall.EINPROGRESS),
		errors.Is(err, syscall.ETIMEDOUT), errors.Is(err, os.ErrDeadlineExceeded):
		return metrics.TimedOut
	default:
		return metrics.Failed
	}
}

// CauseOf is the key failures are grouped by: the NGAP cause for a
// rejection, otherwise the innermost error (e.g. "connection refused").
func CauseOf(err error) string {
	var rejected *gnb.RejectedError
	if errors.As(err, &rejected) {
		return rejected.Cause
	}
	for {
		inner := errors.Unwrap(err)
		if inner == nil {
			return err.Error()
		}
		err = inner
	}
}
```

`tester/run/controller.go`:

```go
package run

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"sync"
	"syscall"
	"time"

	loggergoModel "github.com/Alonza0314/logger-go/v2/model"

	"tester/gnb"
	"tester/metrics"
	"tester/netcfg"
	"tester/profile"
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
		return Snapshot{State: StateIdle, Gnbs: []GnbStatus{}, N2: metrics.NewStage("n2", 0).Snapshot()}
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

	n2   *metrics.Stage
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
	added     []addedAddr // in the order they were added
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
		ctx:       ctx,
		stop:      cancel,
		done:      make(chan struct{}),
		st:        StateConfiguring,
		startedAt: deps.Now(),
		gnbs:      make([]GnbStatus, len(plan.Gnbs)),
		conns:     make([]gnb.Conn, len(plan.Gnbs)),
	}
	for i, spec := range plan.Gnbs {
		r.gnbs[i] = GnbStatus{GnbSpec: spec, State: GnbPending}
	}
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

func (r *run) requestStop() { r.stop() }

func (r *run) snapshot() Snapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	snap := Snapshot{
		RunID: r.id, ProfileName: r.profile.Name, State: r.st, Error: r.errMsg,
		N2:   r.n2.Snapshot(),
		Gnbs: append([]GnbStatus(nil), r.gnbs...),
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

	if r.ctx.Err() == nil { // Stop may arrive while IPs are being added
		r.setState(StateN2)
		r.runN2()
	}
	if r.ctx.Err() == nil {
		r.setState(StateRunning)
		<-r.ctx.Done()
	}

	r.setState(StateStopping)
	r.closeConns()
	r.removeIPs()
	r.setState(StateStopped)
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

func (r *run) closeConns() {
	for i := range r.conns {
		r.mu.Lock()
		conn := r.conns[i]
		r.conns[i] = nil
		r.mu.Unlock()
		if conn == nil {
			continue
		}
		_ = conn.Close()
		r.updateGnb(i, func(g *GnbStatus) { g.State = GnbClosed })
	}
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

	conn, err := r.connectGnb(spec, n.timeout)
	latency := r.deps.Now().Sub(n.firstAttempt[i])
	if err == nil {
		r.mu.Lock()
		r.conns[i] = conn
		r.mu.Unlock()
		r.n2.Finish(metrics.Accepted, latency, "")
		r.updateGnb(i, func(g *GnbStatus) {
			g.State, g.LatencyMs, g.Cause = GnbUp, float64(latency)/float64(time.Millisecond), ""
		})
		go r.watchConn(i, conn)
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
	n.finished()
}

// watchConn reads the association until it fails, discarding whatever
// the AMF sends (phase 1 handles no AMF-initiated procedures). A read
// error while the run is not stopping means the AMF side went away.
// EAGAIN is just SO_RCVTIMEO expiring on an idle association.
func (r *run) watchConn(i int, conn gnb.Conn) {
	buf := make([]byte, 4096)
	for {
		_, err := conn.Read(buf)
		if err == nil || errors.Is(err, syscall.EAGAIN) {
			continue
		}
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
		return
	}
}

func (r *run) connectGnb(spec profile.GnbSpec, timeout time.Duration) (gnb.Conn, error) {
	id, err := gnb.NewIdentity(spec, r.profile.Gnb)
	if err != nil {
		return nil, err
	}
	req, err := id.NGSetupRequest()
	if err != nil {
		return nil, fmt.Errorf("encode ng setup request: %w", err)
	}
	n2 := r.profile.Network.N2
	conn, err := r.deps.Dialer.Dial(spec.N2IP, n2.AmfIP, n2.AmfPort, timeout)
	if err != nil {
		return nil, err
	}
	if err := gnb.ExchangeNGSetup(conn, req); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}
```

- [ ] **Step 4: Run the tests repeatedly under the race detector**

Run: `cd tester && go get github.com/Alonza0314/logger-go/v2@v2.1.0 && go mod tidy && go test -race -count=20 ./run/ && golangci-lint run ./...`
Expected: `ok  tester/run`, `0 issues.` Any flake here is a real concurrency bug, so fix it rather than retrying.

- [ ] **Step 5: Commit**

```bash
git add tester/run tester/go.mod tester/go.sum
git commit -m "$(cat <<'EOF'
feat: tester run controller with n2 stage

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 10: fru-tester HTTP and WebSocket API

**Files:**
- Create: `tester/api/api.go`, `tester/api/api_test.go`

**Interfaces:**
- Consumes: `run.Controller` methods (Task 9) through a local `api.Controller` interface.
- Produces: `api.NewRouter(ctrl api.Controller, apiToken string) *gin.Engine` with these routes, all requiring `Authorization: Bearer <apiToken>`:

| Method | Path | Success | Errors |
|---|---|---|---|
| POST | `/api/profile/validate` | 200 `ValidateResponse{valid, errors, plan}` | 400 bad JSON or unknown field |
| GET | `/api/run` | 200 `run.Snapshot` (`state:"idle"` before the first run) | — |
| POST | `/api/run` | 202 `run.Snapshot` | 400 `StartErrorResponse{message, errors}`, 409 run active |
| POST | `/api/run/stop` | 202 `run.Snapshot` | 409 nothing to stop |
| GET | `/api/run/stream` | WebSocket: `run.Snapshot` JSON text frames on connect, on every change, and at least every 1 s | 401 |

- [ ] **Step 1: Write the failing test** `tester/api/api_test.go`:

```go
package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"

	"tester/profile"
	"tester/run"
)

type fakeCtrl struct {
	validateErr error
	startErr    error
	stopErr     error
	snap        run.Snapshot
	changed     chan struct{}
}

func (f *fakeCtrl) Validate(profile.Profile) (*profile.Plan, error) {
	if f.validateErr != nil {
		return nil, f.validateErr
	}
	return &profile.Plan{UesPerGnb: 4}, nil
}
func (f *fakeCtrl) Start(profile.Profile) (run.Snapshot, error) { return f.snap, f.startErr }
func (f *fakeCtrl) Stop() (run.Snapshot, error)                 { return f.snap, f.stopErr }
func (f *fakeCtrl) Snapshot() run.Snapshot                      { return f.snap }
func (f *fakeCtrl) Changed() <-chan struct{}                    { return f.changed }

func do(t *testing.T, h http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestTokenRequired(t *testing.T) {
	h := NewRouter(&fakeCtrl{}, "secret")
	require.Equal(t, http.StatusUnauthorized, do(t, h, http.MethodGet, "/api/run", "", "").Code)
	require.Equal(t, http.StatusUnauthorized, do(t, h, http.MethodGet, "/api/run", "", "wrong").Code)
	require.Equal(t, http.StatusOK, do(t, h, http.MethodGet, "/api/run", "", "secret").Code)
}

func TestValidateReturnsPlanOrFieldErrors(t *testing.T) {
	ctrl := &fakeCtrl{}
	h := NewRouter(ctrl, "t")

	rec := do(t, h, http.MethodPost, "/api/profile/validate", `{"name":"x"}`, "t")
	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"valid":true,"errors":[],"plan":{"gnbs":null,"uesPerGnb":4,"n2Prefix":0,"n3Prefix":0}}`, rec.Body.String())

	ctrl.validateErr = &profile.ValidationError{Errors: []profile.FieldError{{Field: "name", Message: "must not be empty"}}}
	rec = do(t, h, http.MethodPost, "/api/profile/validate", `{}`, "t")
	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"valid":false,"errors":[{"field":"name","message":"must not be empty"}],"plan":null}`, rec.Body.String())

	rec = do(t, h, http.MethodPost, "/api/profile/validate", `{"bogus":1}`, "t")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), `unknown field \"bogus\"`)
}

func TestStartAndStopStatusCodes(t *testing.T) {
	ctrl := &fakeCtrl{snap: run.Snapshot{RunID: "r1", State: run.StateConfiguring}}
	h := NewRouter(ctrl, "t")

	require.Equal(t, http.StatusAccepted, do(t, h, http.MethodPost, "/api/run", `{}`, "t").Code)

	ctrl.startErr = run.ErrRunActive
	require.Equal(t, http.StatusConflict, do(t, h, http.MethodPost, "/api/run", `{}`, "t").Code)

	ctrl.startErr = &profile.ValidationError{Errors: []profile.FieldError{{Field: "scale.gnbCount", Message: "must be at least 1"}}}
	rec := do(t, h, http.MethodPost, "/api/run", `{}`, "t")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.JSONEq(t, `{"message":"profile is invalid","errors":[{"field":"scale.gnbCount","message":"must be at least 1"}]}`, rec.Body.String())

	require.Equal(t, http.StatusAccepted, do(t, h, http.MethodPost, "/api/run/stop", "", "t").Code)
	ctrl.stopErr = run.ErrNotRunning
	require.Equal(t, http.StatusConflict, do(t, h, http.MethodPost, "/api/run/stop", "", "t").Code)
}

func TestStreamSendsSnapshotOnConnectAndOnChange(t *testing.T) {
	ctrl := &fakeCtrl{snap: run.Snapshot{RunID: "r1", State: run.StateN2}, changed: make(chan struct{})}
	srv := httptest.NewServer(NewRouter(ctrl, "t"))
	defer srv.Close()

	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/run/stream"
	_, resp, err := websocket.DefaultDialer.Dial(url, nil)
	require.Error(t, err)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	conn, _, err := websocket.DefaultDialer.Dial(url, http.Header{"Authorization": {"Bearer t"}})
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	var got run.Snapshot
	require.NoError(t, conn.ReadJSON(&got))
	require.Equal(t, run.StateN2, got.State)

	ctrl.snap = run.Snapshot{RunID: "r1", State: run.StateRunning}
	close(ctrl.changed) // the fake never replaces it, so later frames come at once
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
	require.NoError(t, conn.ReadJSON(&got))
	require.Equal(t, run.StateRunning, got.State)
}

func TestSnapshotJSONShape(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, json.NewEncoder(&buf).Encode(run.Snapshot{State: run.StateIdle}))
	for _, key := range []string{`"runId"`, `"state":"idle"`, `"n2"`, `"gnbs"`, `"startedAt"`} {
		require.Contains(t, buf.String(), key)
	}
}
```

- [ ] **Step 2: Run it to make sure it fails**

Run: `cd tester && go test ./api/`
Expected: FAIL (`undefined: NewRouter`).

- [ ] **Step 3: Implement** `tester/api/api.go`:

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
const streamInterval = time.Second

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
			if err := conn.WriteJSON(ctrl.Snapshot()); err != nil {
				return
			}
			select {
			case <-gone:
				return
			case <-changed:
			case <-ticker.C:
			}
		}
	}
}
```

- [ ] **Step 4: Run the tests**

Run: `cd tester && go get github.com/gin-gonic/gin@v1.12.0 github.com/gorilla/websocket@v1.5.3 && go mod tidy && go test -race ./api/`
Expected: `ok  tester/api`.

- [ ] **Step 5: Commit**

```bash
git add tester/api tester/go.mod tester/go.sum
git commit -m "$(cat <<'EOF'
feat: fru-tester rest and websocket api

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 11: fru-tester binary

**Files:**
- Create: `tester/main.go`, `tester/cmd/root.go`, `tester.yaml` (repo root, dev config)
- Modify: `Makefile`

**Interfaces:**
- Consumes: everything above.
- Produces: `build/fru-tester -c <config>`. On SIGINT or SIGTERM it calls `Controller.Shutdown()`, which stops the active run and removes its IPs, before exiting.

- [ ] **Step 1: Write** `tester/main.go`:

```go
package main

import "tester/cmd"

func main() {
	cmd.Execute()
}
```

`tester/cmd/root.go`:

```go
package cmd

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	loggergo "github.com/Alonza0314/logger-go/v2"
	loggergoUtil "github.com/Alonza0314/logger-go/v2/util"
	"github.com/spf13/cobra"

	"tester/api"
	"tester/config"
	"tester/gnb"
	"tester/netcfg"
	"tester/run"
)

var rootCmd = &cobra.Command{
	Use:   "fru-tester",
	Short: "5G throughput tester engine (driven by fru-lab)",
	RunE:  serve,
}

func init() {
	rootCmd.Flags().StringP("config", "c", "tester.yaml", "path to fru-tester config")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func serve(cmd *cobra.Command, _ []string) error {
	path, _ := cmd.Flags().GetString("config")
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}

	lg := loggergo.NewLogger("", true)
	lg.SetLevel(loggergoUtil.LogLevelString(cfg.Logger.Level))
	mainLog := lg.WithTags("TESTER")

	ctrl := run.NewController(run.Deps{
		Addrs:  netcfg.Netlink{},
		Dialer: gnb.SCTPDialer{},
		Log:    lg.WithTags("RUN"),
	})
	srv := &http.Server{Addr: cfg.Listen, Handler: api.NewRouter(ctrl, cfg.ApiToken)}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	mainLog.Infof("fru-tester listening on %s", cfg.Listen)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-sig:
	}

	mainLog.Infoln("shutting down: stopping active run and removing gNB IPs")
	ctrl.Shutdown()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}
```

Dev config `tester.yaml` at the repo root:

```yaml
# fru-tester dev config. fru-lab's config.yaml backend.tester must point at
# this listen address with the same apiToken.
listen: "127.0.0.1:9100"
apiToken: "frulab-tester"

logger:
  level: "info" # error, warn, info, debug, trace
```

- [ ] **Step 2: Add Makefile targets.** Add the source list next to `BACKEND_SRC`:

```make
TESTER_SRC := $(shell find tester -name "*.go") tester/go.mod tester/go.sum
```

Add after the `build/frontend` rule:

```make
build/fru-tester: $(TESTER_SRC)
	@echo "[+] Building fru-tester..."
	mkdir -p build
	cd tester && go build -o ../build/fru-tester .
	@echo "[✔] fru-tester build finished"

tester: build/fru-tester
```

Add after `run:`:

```make
# needs root for netlink (gNB IPs) and the sctp kernel module
run-tester:
	sudo modprobe sctp
	sudo ./build/fru-tester -c tester.yaml
```

- [ ] **Step 3: Build and smoke-test**

Run:
```bash
cd tester && go get github.com/spf13/cobra@v1.10.2 && go mod tidy && cd ..
make tester
sudo ./build/fru-tester -c tester.yaml &
sleep 1
curl -s -H 'Authorization: Bearer frulab-tester' 127.0.0.1:9100/api/run
curl -s -o /dev/null -w '%{http_code}\n' 127.0.0.1:9100/api/run
sudo kill %1
```
Expected: the first curl prints JSON starting `{"runId":"","profileName":"","state":"idle"`, the second prints `401`, and the process logs `shutting down` and exits.

- [ ] **Step 4: Commit**

```bash
git add tester/main.go tester/cmd tester/go.mod tester/go.sum tester.yaml Makefile
git commit -m "$(cat <<'EOF'
feat: fru-tester binary and make targets

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 12: fru-lab stores the tester profile

**Files:**
- Modify: `web/backend/config/config.go`, `web/backend/logger/tag.go`, `web/backend/logger/logger.go`, `web/backend/internal/context/db.go`, `web/backend/internal/context/dbBbolt.go`, `web/backend/internal/context/dbContext.go`
- Create: `web/backend/internal/context/dbBbolt_test.go`, `web/backend/internal/processor/tester.go`, `web/backend/model/tester.go`

**Interfaces:**
- Produces:
  - `config.BackendIE.Tester config.TesterIE{URL, ApiToken string}`, read from YAML `backend.tester.url` and `backend.tester.apiToken`.
  - `BackendLogger.TesterLog`.
  - `DbIf.Get(bucket, key string) ([]byte, error)` (returns nil, nil when missing) and `DbIf.Put(bucket, key string, value []byte) error`.
  - `(*dbContext).GetTesterProfile() ([]byte, error)` and `PutTesterProfile([]byte) error`, reachable through the embedded `FlContext`.
  - `(*Processor).TesterProfileGet() ([]byte, *model.ErrorDetail)` and `TesterProfilePut(raw []byte) (*model.ResponseTesterAction, *model.ErrorDetail)`.
  - `model.ResponseTesterAction{Message string}`.

fru-lab stores the profile as opaque JSON. fru-tester owns its schema and validation, so fru-lab only checks that the body is a JSON object of at most 64 KiB.

- [ ] **Step 1: Write the failing test** `web/backend/internal/context/dbBbolt_test.go`:

```go
package context

import (
	"path/filepath"
	"testing"
)

func TestBboltGetPut(t *testing.T) {
	db, err := newBboltDb(filepath.Join(t.TempDir(), "sub", "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Release() }()

	got, err := db.Get("tester", "profile")
	if err != nil || got != nil {
		t.Fatalf("missing bucket: got %q, %v; want nil, nil", got, err)
	}
	if err := db.Put("tester", "profile", []byte(`{"name":"a"}`)); err != nil {
		t.Fatal(err)
	}
	if err := db.Put("tester", "profile", []byte(`{"name":"b"}`)); err != nil {
		t.Fatal(err)
	}
	got, err = db.Get("tester", "profile")
	if err != nil || string(got) != `{"name":"b"}` {
		t.Fatalf("got %q, %v; want the second write", got, err)
	}
	got, err = db.Get("tester", "other")
	if err != nil || got != nil {
		t.Fatalf("missing key: got %q, %v; want nil, nil", got, err)
	}
}
```

- [ ] **Step 2: Run it to make sure it fails**

Run: `cd web/backend && go test ./internal/context/ -run Bbolt`
Expected: FAIL (`db.Get undefined`).

- [ ] **Step 3: Implement storage.** In `web/backend/internal/context/db.go` replace the interface:

```go
type DbIf interface {
	// Get returns nil, nil when bucket or key does not exist yet.
	Get(bucket, key string) ([]byte, error)
	Put(bucket, key string, value []byte) error
	Release() error
}
```

In `web/backend/internal/context/dbBbolt.go` add above `Release`:

```go
func (b *bboltDb) Get(bucket, key string) ([]byte, error) {
	var out []byte
	err := b.db.View(func(tx *bbolt.Tx) error {
		bk := tx.Bucket([]byte(bucket))
		if bk == nil {
			return nil
		}
		if v := bk.Get([]byte(key)); v != nil {
			out = append([]byte(nil), v...) // v is only valid inside the tx
		}
		return nil
	})
	return out, err
}

func (b *bboltDb) Put(bucket, key string, value []byte) error {
	return b.db.Update(func(tx *bbolt.Tx) error {
		bk, err := tx.CreateBucketIfNotExists([]byte(bucket))
		if err != nil {
			return err
		}
		return bk.Put([]byte(key), value)
	})
}
```

In `web/backend/internal/context/dbContext.go` add above `release`:

```go
const (
	testerBucket     = "tester"
	testerProfileKey = "profile"
)

// GetTesterProfile returns the saved Throughput Tester profile JSON, or
// nil if none was saved yet. fru-lab stores it opaquely; fru-tester owns
// its schema and validation.
func (d *dbContext) GetTesterProfile() ([]byte, error) {
	return d.db.Get(testerBucket, testerProfileKey)
}

func (d *dbContext) PutTesterProfile(profile []byte) error {
	return d.db.Put(testerBucket, testerProfileKey, profile)
}
```

- [ ] **Step 4: Run the test**

Run: `cd web/backend && go test ./internal/context/ -run Bbolt`
Expected: `ok  backend/internal/context`.

- [ ] **Step 5: Config, logger, model, processor.** In `web/backend/config/config.go`, inside `BackendIE`, after `FrontendFilePath`:

```go
	Tester TesterIE `yaml:"tester"`
}

// TesterIE points at the fru-tester engine. Leaving URL empty disables the
// Throughput Tester pages' backend routes (they answer 503).
type TesterIE struct {
	URL      string `yaml:"url"`
	ApiToken string `yaml:"apiToken"`
}
```

(The `}` replaces the original closing brace of `BackendIE`.)

`web/backend/logger/tag.go`: add `TESTER_LOG = "TESTER"` after `DEPLOY_LOG`. In `web/backend/logger/logger.go`, add a `TesterLog loggergoModel.LoggerInterface` field after `DeployLog`, and `TesterLog: logger.WithTags(TESTER_LOG),` after `DeployLog: logger.WithTags(DEPLOY_LOG),`.

Create `web/backend/model/tester.go`:

```go
package model

type ResponseTesterAction struct {
	Message string `json:"message"`
}
```

Create `web/backend/internal/processor/tester.go`:

```go
package processor

import (
	"backend/model"
	"bytes"
	"encoding/json"
	"net/http"
)

// maxTesterProfileBytes caps what a client can make us store.
const maxTesterProfileBytes = 64 << 10

// TesterProfileGet returns the saved profile JSON, or nil if none exists.
func (p *Processor) TesterProfileGet() ([]byte, *model.ErrorDetail) {
	raw, err := p.FlContext.GetTesterProfile()
	if err != nil {
		p.ProcLog.Errorf("Failed to read tester profile: %v", err)
		return nil, &model.ErrorDetail{HttpStatus: http.StatusInternalServerError, Detail: "Failed to read the saved profile"}
	}
	return raw, nil
}

// TesterProfilePut stores raw as-is after checking it is one JSON object.
// Field-level validation belongs to fru-tester (/api/tester/profile/validate).
func (p *Processor) TesterProfilePut(raw []byte) (*model.ResponseTesterAction, *model.ErrorDetail) {
	if len(raw) > maxTesterProfileBytes {
		return nil, &model.ErrorDetail{HttpStatus: http.StatusRequestEntityTooLarge, Detail: "Profile is larger than 64 KiB"}
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, &model.ErrorDetail{HttpStatus: http.StatusBadRequest, Detail: "Profile must be a JSON object: " + err.Error()}
	}
	var compact bytes.Buffer
	_ = json.Compact(&compact, raw)
	if err := p.FlContext.PutTesterProfile(compact.Bytes()); err != nil {
		p.ProcLog.Errorf("Failed to save tester profile: %v", err)
		return nil, &model.ErrorDetail{HttpStatus: http.StatusInternalServerError, Detail: "Failed to save the profile"}
	}
	return &model.ResponseTesterAction{Message: "Profile saved"}, nil
}
```

- [ ] **Step 6: Verify and commit**

Run: `cd web/backend && go build ./... && go test ./... && golangci-lint run`
Expected: builds, tests pass, `0 issues.`

```bash
git add web/backend
git commit -m "$(cat <<'EOF'
feat: store throughput tester profile in bbolt

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 13: fru-lab routes and reverse proxy to fru-tester

**Files:**
- Create: `web/backend/internal/testerProxy.go`, `web/backend/internal/testerProxy_test.go`, `web/backend/internal/api_tester.go`
- Modify: `web/backend/internal/backend.go`, `config.yaml`

**Interfaces:**
- Consumes: Task 12's processor methods, `config.TesterIE`, and fru-tester's API (Task 10).
- Produces these fru-lab routes. Every one except the stream is JWT-header protected; the stream checks `?token=<JWT>`:

| Method | fru-lab path | Handled by |
|---|---|---|
| GET / PUT | `/api/tester/profile` | fru-lab (bbolt); GET answers 204 when nothing is saved |
| POST | `/api/tester/profile/validate` | proxy → `/api/profile/validate` |
| GET / POST | `/api/tester/run` | proxy → `/api/run` |
| POST | `/api/tester/run/stop` | proxy → `/api/run/stop` |
| GET (WS) | `/api/tester/run/stream?token=` | JWT check, then proxy → `/api/run/stream` |

The proxy rewrites `/api/tester/<rest>` to `/api/<rest>`, swaps the user's JWT for `Bearer <tester.apiToken>`, and drops `?token=`. It answers 502 when fru-tester is down. When `backend.tester.url` is empty, every tester route answers 503.

- [ ] **Step 1: Write the failing test** `web/backend/internal/testerProxy_test.go`. It pins Review Focus #5.

```go
package internal

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func TestTesterProxyRewritesPathAndAuth(t *testing.T) {
	var gotPath, gotQuery, gotAuth, gotBody string
	tester := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery, gotAuth = r.URL.Path, r.URL.RawQuery, r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"state":"configuring"}`))
	}))
	defer tester.Close()

	proxy, err := newTesterProxy(tester.URL, "engine-token")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/tester/run?token=user-jwt&x=1", strings.NewReader(`{"name":"p"}`))
	req.Header.Set("Authorization", "Bearer user-jwt")
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted || rec.Body.String() != `{"state":"configuring"}` {
		t.Fatalf("response = %d %q", rec.Code, rec.Body.String())
	}
	if gotPath != "/api/run" || gotQuery != "x=1" || gotAuth != "Bearer engine-token" || gotBody != `{"name":"p"}` {
		t.Fatalf("tester saw path=%q query=%q auth=%q body=%q", gotPath, gotQuery, gotAuth, gotBody)
	}
}

func TestTesterProxyReportsUnreachableAs502(t *testing.T) {
	tester := httptest.NewServer(http.NotFoundHandler())
	url := tester.URL
	tester.Close() // nothing listens there now

	proxy, err := newTesterProxy(url, "t")
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/tester/run", nil))
	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if rec.Code != http.StatusBadGateway || !strings.HasPrefix(body["message"], "fru-tester is unreachable") {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
}

func TestTesterProxyPassesWebSocket(t *testing.T) {
	up := websocket.Upgrader{}
	tester := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/run/stream" || r.Header.Get("Authorization") != "Bearer t" {
			http.Error(w, "bad", http.StatusUnauthorized)
			return
		}
		c, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = c.Close() }()
		_ = c.WriteMessage(websocket.TextMessage, []byte(`{"state":"n2"}`))
	}))
	defer tester.Close()

	proxy, err := newTesterProxy(tester.URL, "t")
	if err != nil {
		t.Fatal(err)
	}
	front := httptest.NewServer(proxy)
	defer front.Close()

	ws, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(front.URL, "http")+"/api/tester/run/stream?token=jwt", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ws.Close() }()
	_, msg, err := ws.ReadMessage()
	if err != nil || string(msg) != `{"state":"n2"}` {
		t.Fatalf("got %q, %v", msg, err)
	}
}

func TestNewTesterProxyRejectsRelativeURL(t *testing.T) {
	if _, err := newTesterProxy("localhost:9100", "t"); err == nil {
		t.Fatal("want error for URL without scheme")
	}
}

func TestTesterRoutesAnswer503WhenNotConfigured(t *testing.T) {
	b := &backend{} // tester.url empty -> no proxy
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/tester/run", nil)
	b.handleTesterProxy(c)
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "backend.tester.url") {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
}
```

- [ ] **Step 2: Run it to make sure it fails**

Run: `cd web/backend && go test ./internal/ -run Tester`
Expected: FAIL (`undefined: newTesterProxy`, `b.handleTesterProxy undefined`).

- [ ] **Step 3: Implement the proxy** `web/backend/internal/testerProxy.go`:

```go
package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

// newTesterProxy forwards /api/tester/<rest> to <testerURL>/api/<rest>.
// The caller has already checked the user's JWT; the proxy replaces it
// with fru-tester's own API token and drops the ?token= query param the
// WebSocket route uses. WebSocket upgrades pass through unchanged.
func newTesterProxy(testerURL, apiToken string) (*httputil.ReverseProxy, error) {
	target, err := url.Parse(testerURL)
	if err != nil || target.Scheme == "" || target.Host == "" {
		return nil, fmt.Errorf("tester.url %q must be an absolute http(s) URL", testerURL)
	}
	return &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.Out.URL.Scheme = target.Scheme
			pr.Out.URL.Host = target.Host
			pr.Out.URL.Path = "/api" + strings.TrimPrefix(pr.In.URL.Path, "/api/tester")
			pr.Out.URL.RawPath = ""
			q := pr.Out.URL.Query()
			q.Del("token")
			pr.Out.URL.RawQuery = q.Encode()
			pr.Out.Host = target.Host
			pr.Out.Header.Set("Authorization", "Bearer "+apiToken)
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"message": "fru-tester is unreachable: " + err.Error(),
			})
		},
	}, nil
}
```

- [ ] **Step 4: Implement the routes** `web/backend/internal/api_tester.go`:

```go
package internal

import (
	"backend/model"
	"io"
	"net/http"

	"github.com/free-ran-ue/util"
	"github.com/gin-gonic/gin"
)

// getTesterRoutes are the Throughput Tester routes behind the JWT header
// middleware. Everything except the saved profile is forwarded to
// fru-tester unchanged (see newTesterProxy).
func (b *backend) getTesterRoutes() util.Routes {
	return util.Routes{
		{Name: "TesterProfileGet", Method: http.MethodGet, Pattern: "/tester/profile",
			HandlerFunc: withLogging("TesterProfileGet", b.TesterLog, b.handleTesterProfileGet)},
		{Name: "TesterProfilePut", Method: http.MethodPut, Pattern: "/tester/profile",
			HandlerFunc: withLogging("TesterProfilePut", b.TesterLog, b.handleTesterProfilePut)},
		{Name: "TesterProfileValidate", Method: http.MethodPost, Pattern: "/tester/profile/validate",
			HandlerFunc: withLogging("TesterProfileValidate", b.TesterLog, b.handleTesterProxy)},
		{Name: "TesterRunGet", Method: http.MethodGet, Pattern: "/tester/run",
			HandlerFunc: b.handleTesterProxy}, // polled; not logged per request
		{Name: "TesterRunStart", Method: http.MethodPost, Pattern: "/tester/run",
			HandlerFunc: withLogging("TesterRunStart", b.TesterLog, b.handleTesterProxy)},
		{Name: "TesterRunStop", Method: http.MethodPost, Pattern: "/tester/run/stop",
			HandlerFunc: withLogging("TesterRunStop", b.TesterLog, b.handleTesterProxy)},
	}
}

// getTesterStreamRoutes: a browser WebSocket cannot send an Authorization
// header, so the stream checks ?token= itself, like the ue terminal.
func (b *backend) getTesterStreamRoutes() util.Routes {
	return util.Routes{
		{Name: "TesterRunStream", Method: http.MethodGet, Pattern: "/tester/run/stream",
			HandlerFunc: b.handleTesterStream},
	}
}

func (b *backend) handleTesterProfileGet(c *gin.Context) {
	raw, errDetail := b.Processor.TesterProfileGet()
	if errDetail != nil {
		c.JSON(errDetail.HttpStatus, model.ResponseTesterAction{Message: errDetail.Detail})
		return
	}
	if raw == nil {
		c.Status(http.StatusNoContent)
		return
	}
	c.Data(http.StatusOK, "application/json", raw)
}

func (b *backend) handleTesterProfilePut(c *gin.Context) {
	raw, err := io.ReadAll(io.LimitReader(c.Request.Body, maxTesterBodyBytes))
	if err != nil {
		c.JSON(http.StatusBadRequest, model.ResponseTesterAction{Message: "Failed to read request body"})
		return
	}
	response, errDetail := b.Processor.TesterProfilePut(raw)
	if errDetail != nil {
		c.JSON(errDetail.HttpStatus, model.ResponseTesterAction{Message: errDetail.Detail})
		return
	}
	c.JSON(http.StatusOK, response)
}

// maxTesterBodyBytes is one byte over the processor's 64 KiB cap, so an
// oversized body is reported as too large rather than cut and misparsed.
const maxTesterBodyBytes = 64<<10 + 1

func (b *backend) handleTesterProxy(c *gin.Context) {
	if b.testerProxy == nil {
		c.JSON(http.StatusServiceUnavailable, model.ResponseTesterAction{
			Message: "Throughput Tester is not configured: set backend.tester.url in config.yaml",
		})
		return
	}
	b.testerProxy.ServeHTTP(c.Writer, c.Request)
}

func (b *backend) handleTesterStream(c *gin.Context) {
	if _, err := util.ValidateJWT(c.Query("token"), b.jwt.secret); err != nil {
		c.JSON(http.StatusUnauthorized, model.ResponseTesterAction{Message: "Invalid token: " + err.Error()})
		return
	}
	b.handleTesterProxy(c)
}
```

- [ ] **Step 5: Wire it into the backend.** In `web/backend/internal/backend.go`:

1. Add `"net/http/httputil"` to the imports.
2. Add to `type backend struct`, after `frontendFilePath string`:

```go
	// testerProxy is nil when backend.tester.url is not configured.
	testerProxy *httputil.ReverseProxy
```

3. In `NewBackend`, immediately before `gin.DefaultWriter, gin.DefaultErrorWriter = ...`:

```go
	if config.Backend.Tester.URL != "" {
		proxy, err := newTesterProxy(config.Backend.Tester.URL, config.Backend.Tester.ApiToken)
		if err != nil {
			logger.BckLog.Errorf("Invalid tester config: %v", err)
			return nil
		}
		b.testerProxy = proxy
	} else {
		logger.BckLog.Warnln("backend.tester.url is empty; Throughput Tester routes will answer 503")
	}
```

4. In `addServices`, after `addRoutes(apiGroup, b.getTerminalRoutes())`:

```go
	addRoutes(authGroup, b.getTesterRoutes())
	addRoutes(apiGroup, b.getTesterStreamRoutes())
```

5. In the repo-root `config.yaml`, add under `backend:` after `frontendFilePath`:

```yaml

  tester:
    # fru-tester API (see tester.yaml). Leave url empty to disable the pages.
    url: "http://127.0.0.1:9100"
    apiToken: "frulab-tester"
```

- [ ] **Step 6: Verify**

Run: `cd web/backend && go test ./... && golangci-lint run && cd ../.. && make backend`
Expected: all pass, `0 issues.`, backend builds.

Manual check with both processes running (`make run` in one shell, `make run-tester` in another):

```bash
TOKEN=$(curl -s -X POST localhost:8888/api/login -H 'Content-Type: application/json' -d '{"username":"admin","password":"frulab"}' | sed 's/.*"token":"\([^"]*\)".*/\1/')
curl -s -o /dev/null -w '%{http_code}\n' -H "Authorization: Bearer $TOKEN" localhost:8888/api/tester/profile          # 204
curl -s -X PUT -H "Authorization: Bearer $TOKEN" -d '{"name":"x"}' localhost:8888/api/tester/profile                  # {"message":"Profile saved"}
curl -s -H "Authorization: Bearer $TOKEN" localhost:8888/api/tester/profile                                           # {"name":"x"}
curl -s -H "Authorization: Bearer $TOKEN" localhost:8888/api/tester/run | head -c 60                                  # {"runId":"","profileName":"","state":"idle"...
```

(`/api/login` answers `{"message":…,"token":…}`; see `web/backend/model/account.go`.)

- [ ] **Step 7: Commit**

```bash
git add web/backend config.yaml
git commit -m "$(cat <<'EOF'
feat: proxy throughput tester api through fru-lab

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 14: OpenAPI spec and generated client

**Files:**
- Modify: `web/openapi.yaml`
- Regenerate: `web/frontend/src/api/` (via `make openapi`)

**Interfaces:**
- Produces these TS client methods on `api` from `apiClient.ts`: `testerProfileGet()`, `testerProfilePut(p)`, `testerProfileValidate(p)`, `testerRunGet()`, `testerRunStart(p)`, `testerRunStop()`.
- Produces these TS types: `TesterProfile`, `TesterValidateResponse`, `TesterPlan`, `TesterGnbSpec`, `TesterFieldError`, `TesterRunSnapshot` (with `state` enum `idle|configuring|n2|running|stopping|stopped|failed`), `TesterStageSnapshot`, `TesterGnbStatus` (with `state` enum `pending|connecting|up|failed|lost|closed`), `TesterCauseCount`.
- Field names must match the Go JSON tags in Tasks 2, 4, 5, 9 and 10 exactly.

- [ ] **Step 1: Add the paths.** In `web/openapi.yaml`, insert this block immediately before the top-level `components:` line, after the last `/api/images/{key}/pull` path:

```yaml
  /api/tester/profile:
    get:
      summary: Get the saved Throughput Tester profile
      description: Returns the last profile saved from the setup page. 204 when none has been saved yet.
      operationId: testerProfileGet
      security:
        - bearerAuth: []
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/TesterProfile'
        '204':
          description: No profile saved yet
        '401':
          description: Unauthorized
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/MessageResponse'
    put:
      summary: Save the Throughput Tester profile
      description: Stores the profile as-is. Field validation is done by POST /api/tester/profile/validate, not here.
      operationId: testerProfilePut
      security:
        - bearerAuth: []
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/TesterProfile'
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/MessageResponse'
        '400':
          description: Body is not a JSON object
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/MessageResponse'
        '401':
          description: Unauthorized
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/MessageResponse'

  /api/tester/profile/validate:
    post:
      summary: Validate a profile and preview the per-gNB plan
      description: Forwarded to fru-tester. Always 200 for a well-formed body; `valid` says whether the profile can start a run.
      operationId: testerProfileValidate
      security:
        - bearerAuth: []
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/TesterProfile'
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/TesterValidateResponse'
        '502':
          description: fru-tester is unreachable
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/MessageResponse'
        '503':
          description: backend.tester.url is not configured
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/MessageResponse'

  /api/tester/run:
    get:
      summary: Current run snapshot
      description: Forwarded to fru-tester. State is `idle` before the first run.
      operationId: testerRunGet
      security:
        - bearerAuth: []
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/TesterRunSnapshot'
        '502':
          description: fru-tester is unreachable
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/MessageResponse'
        '503':
          description: backend.tester.url is not configured
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/MessageResponse'
    post:
      summary: Start a run
      description: Forwarded to fru-tester. Only one run may be active at a time.
      operationId: testerRunStart
      security:
        - bearerAuth: []
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/TesterProfile'
      responses:
        '202':
          description: Run started
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/TesterRunSnapshot'
        '400':
          description: Profile is invalid
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/TesterStartErrorResponse'
        '409':
          description: A run is already active
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/MessageResponse'
        '502':
          description: fru-tester is unreachable
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/MessageResponse'
        '503':
          description: backend.tester.url is not configured
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/MessageResponse'

  /api/tester/run/stop:
    post:
      summary: Stop the active run
      description: Closes every N2 association and removes the gNB IPs fru-tester added. Returns at once; watch the stream for `stopped`.
      operationId: testerRunStop
      security:
        - bearerAuth: []
      responses:
        '202':
          description: Stop requested
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/TesterRunSnapshot'
        '409':
          description: No active run
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/MessageResponse'
```

- [ ] **Step 2: Add the schemas** at the end of `components.schemas` (the end of the file, after `ImageListResponse`):

```yaml
    TesterProfile:
      type: object
      required: [name, scale, gnb, network, rates]
      properties:
        name:
          type: string
          example: 1000 UE baseline
        scale:
          type: object
          required: [gnbCount, ueCount]
          properties:
            gnbCount:
              type: integer
              example: 10
            ueCount:
              type: integer
              example: 1000
        gnb:
          type: object
          required: [gnbIdStart, namePattern, mcc, mnc, tac, sst, sd]
          properties:
            gnbIdStart:
              type: string
              description: 6 or 8 hex digits; incremented per gNB, keeping its width.
              example: '000314'
            namePattern:
              type: string
              description: Must contain {i}, replaced by the 1-based gNB index.
              example: gNB-{i}
            mcc:
              type: string
              example: '208'
            mnc:
              type: string
              example: '93'
            tac:
              type: string
              example: '000001'
            sst:
              type: integer
              example: 1
            sd:
              type: string
              description: Empty or 6 hex digits.
              example: '010203'
        network:
          type: object
          required: [n2, n3]
          properties:
            n2:
              type: object
              required: [interface, cidr, startIp, amfIp, amfPort]
              properties:
                interface:
                  type: string
                  example: ens19
                cidr:
                  type: string
                  example: 10.0.1.0/24
                startIp:
                  type: string
                  example: 10.0.1.100
                amfIp:
                  type: string
                  example: 10.0.1.1
                amfPort:
                  type: integer
                  example: 38412
            n3:
              type: object
              required: [interface, cidr, startIp, upfIp, upfPort]
              properties:
                interface:
                  type: string
                  example: ens20
                cidr:
                  type: string
                  example: 10.0.2.0/24
                startIp:
                  type: string
                  example: 10.0.2.100
                upfIp:
                  type: string
                  example: 10.0.2.1
                upfPort:
                  type: integer
                  example: 2152
        rates:
          type: object
          required: [n2]
          properties:
            n2:
              $ref: '#/components/schemas/TesterStageRate'
    TesterStageRate:
      type: object
      required: [timeoutMs, retries]
      properties:
        timeoutMs:
          type: integer
          example: 5000
        retries:
          type: integer
          example: 1
    TesterFieldError:
      type: object
      required: [field, message]
      properties:
        field:
          type: string
          example: network.n2.cidr
        message:
          type: string
    TesterGnbSpec:
      type: object
      required: [index, name, gnbId, n2Ip, n3Ip, ueCount, ueFirst, ueLast]
      properties:
        index:
          type: integer
        name:
          type: string
        gnbId:
          type: string
        n2Ip:
          type: string
        n3Ip:
          type: string
        ueCount:
          type: integer
        ueFirst:
          type: integer
          description: 1-based index of the first UE on this gNB; 0 when it has none.
        ueLast:
          type: integer
    TesterPlan:
      type: object
      required: [gnbs, uesPerGnb, n2Prefix, n3Prefix]
      properties:
        gnbs:
          type: array
          items:
            $ref: '#/components/schemas/TesterGnbSpec'
        uesPerGnb:
          type: integer
        n2Prefix:
          type: integer
        n3Prefix:
          type: integer
    TesterValidateResponse:
      type: object
      required: [valid, errors]
      properties:
        valid:
          type: boolean
        errors:
          type: array
          items:
            $ref: '#/components/schemas/TesterFieldError'
        plan:
          allOf:
            - $ref: '#/components/schemas/TesterPlan'
          nullable: true
    TesterStartErrorResponse:
      type: object
      required: [message, errors]
      properties:
        message:
          type: string
        errors:
          type: array
          items:
            $ref: '#/components/schemas/TesterFieldError'
    TesterCauseCount:
      type: object
      required: [cause, count]
      properties:
        cause:
          type: string
          example: misc(4)
        count:
          type: integer
          format: int64
    TesterStageSnapshot:
      type: object
      description: One stage card. Latencies are milliseconds over accepted items only; totalTimeMs is first start to last finish.
      required: [name, expected, attempted, retries, inFlight, accepted, rejected, timedOut, failed, done, totalTimeMs, avgMs, p50Ms, p95Ms, p99Ms, maxMs, causes]
      properties:
        name:
          type: string
        expected:
          type: integer
        attempted:
          type: integer
          format: int64
        retries:
          type: integer
          format: int64
        inFlight:
          type: integer
          format: int64
        accepted:
          type: integer
          format: int64
        rejected:
          type: integer
          format: int64
        timedOut:
          type: integer
          format: int64
        failed:
          type: integer
          format: int64
        done:
          type: boolean
        totalTimeMs:
          type: number
        avgMs:
          type: number
        p50Ms:
          type: number
        p95Ms:
          type: number
        p99Ms:
          type: number
        maxMs:
          type: number
        causes:
          type: array
          items:
            $ref: '#/components/schemas/TesterCauseCount'
    TesterGnbStatus:
      allOf:
        - $ref: '#/components/schemas/TesterGnbSpec'
        - type: object
          required: [state, attempts, latencyMs, cause]
          properties:
            state:
              type: string
              enum: [pending, connecting, up, failed, lost, closed]
              description: lost = was up, then the AMF side dropped the association.
            attempts:
              type: integer
            latencyMs:
              type: number
            cause:
              type: string
    TesterRunSnapshot:
      type: object
      required: [runId, profileName, state, error, n2, gnbs]
      properties:
        runId:
          type: string
        profileName:
          type: string
        state:
          type: string
          enum: [idle, configuring, n2, running, stopping, stopped, failed]
        error:
          type: string
        startedAt:
          type: string
          format: date-time
          nullable: true
        stoppedAt:
          type: string
          format: date-time
          nullable: true
        n2:
          $ref: '#/components/schemas/TesterStageSnapshot'
        gnbs:
          type: array
          items:
            $ref: '#/components/schemas/TesterGnbStatus'
```

- [ ] **Step 3: Generate and verify**

Run:
```bash
make openapi
sudo chown -R "$USER" web/frontend/src/api   # the generator container writes as root
grep -c "testerRunStart\|TesterRunSnapshotStateEnum\|Lost: 'lost'" web/frontend/src/api/api.ts
cd web/frontend && yarn build
```
Expected: the grep count is non-zero and the build succeeds. The generated diff should only add tester types and methods. If it also rewrites unrelated files, compare with `git diff --stat web/frontend/src/api` and keep only what the spec change implies.

- [ ] **Step 4: Commit**

```bash
git add web/openapi.yaml web/frontend/src/api
git commit -m "$(cat <<'EOF'
feat: openapi spec for throughput tester

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 15: Sidebar group and Setup page

**Files:**
- Modify: `web/frontend/src/components/sidebar/Sidebar.tsx`, `web/frontend/src/components/sidebar/sidebar.module.css`, `web/frontend/src/App.tsx`
- Create: `web/frontend/src/page/tester/testerDefaults.ts`, `web/frontend/src/page/tester/tester.module.css`, `web/frontend/src/page/tester/TesterSetupPage.tsx`

**Interfaces:**
- Consumes: Task 14's client.
- Produces: route `/tester` and the shared `tester.module.css`. Task 16 reuses `layout`, `content`, `header`, `title`, `subtitle`, `headerActions`, `card`, `cardTitle`, `tableWrap`, `table`, `mono`, `pill*`, `stageTop`, `bigNumber`, `bar*`, `kv`, `causes`, `causeCell` and `runError` from it.

Setup page behaviour:
- It loads the saved profile, and falls back to `DEFAULT_TESTER_PROFILE` on 204.
- It re-validates 400 ms after the last edit. A sequence number drops stale answers.
- Field errors are shown under the input whose `path` equals the error's `field`.
- The plan preview shows the first 10 gNBs.
- "Start run" is enabled only when the profile is valid. It saves first, then starts the run and navigates to `/tester/run`.

- [ ] **Step 1: Sidebar.** In `Sidebar.tsx`, after the closing `</nav>` of the existing nav, still inside the first `<div>`:

```tsx

        <p className={styles.navGroup}>Throughput Tester</p>
        <nav className={styles.nav}>
          <NavLink to="/tester" end className={navItemClassName}>Setup</NavLink>
          <NavLink to="/tester/run" className={navItemClassName}>Run</NavLink>
        </nav>
```

In `sidebar.module.css`, before `.navItem {`:

```css
.navGroup {
  margin: 1.5rem 0 0.5rem 0.75rem;
  font-size: 0.68rem;
  font-weight: 700;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: #94a3b8;
}

```

- [ ] **Step 2: Page files.** `web/frontend/src/page/tester/testerDefaults.ts`:

```ts
import type { TesterProfile } from '../../api'

// Starting point for a first-time setup page; values mirror free-ran-ue's
// sample gnb.yaml so a lab already running fru-lab's free5GC gets close.
export const DEFAULT_TESTER_PROFILE: TesterProfile = {
  name: 'N2 baseline',
  scale: { gnbCount: 10, ueCount: 1000 },
  gnb: {
    gnbIdStart: '000314',
    namePattern: 'gNB-{i}',
    mcc: '208',
    mnc: '93',
    tac: '000001',
    sst: 1,
    sd: '010203',
  },
  network: {
    n2: { interface: '', cidr: '10.0.1.0/24', startIp: '10.0.1.100', amfIp: '10.0.1.1', amfPort: 38412 },
    n3: { interface: '', cidr: '10.0.2.0/24', startIp: '10.0.2.100', upfIp: '10.0.2.1', upfPort: 2152 },
  },
  rates: { n2: { timeoutMs: 5000, retries: 1 } },
}
```

`web/frontend/src/page/tester/tester.module.css`:

```css
/* Shared by the Throughput Tester setup and run pages. Layout and header
   follow images-page.module.css; cards and fields follow
   subscribers/webconsole-style.module.css. */
.layout { min-height: 100vh; display: flex; background: #f8fafc; }
.content { flex-grow: 1; min-width: 0; padding: 2.5rem; display: flex; flex-direction: column; gap: 1.5rem; box-sizing: border-box; }
.header { display: flex; align-items: flex-start; justify-content: space-between; gap: 1rem; flex-wrap: wrap; }
.title { margin: 0 0 0.35rem; font-size: 1.625rem; font-weight: 700; color: #0f172a; }
.subtitle { margin: 0; color: #64748b; font-size: 0.875rem; max-width: 60ch; }
.headerActions { display: flex; align-items: center; gap: 0.75rem; flex-wrap: wrap; }
.emptyState { color: #64748b; font-size: 0.9rem; }

.card { background: #ffffff; border-radius: 16px; box-shadow: 0 1px 3px 0 rgb(0 0 0 / 0.1); padding: 1.5rem 1.75rem; min-width: 0; }
.cardTitle { margin: 0 0 1rem; font-size: 1.05rem; font-weight: 700; color: #0f172a; }
.fieldGrid { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 1.5rem 1.25rem; }
.field { position: relative; margin-top: 0.6rem; min-width: 0; }
.field label { position: absolute; top: -0.55rem; left: 0.7rem; background: #ffffff; padding: 0 0.35rem; font-size: 0.72rem; color: #64748b; white-space: nowrap; }
.input { width: 100%; box-sizing: border-box; border: 1px solid #cbd5e1; border-radius: 10px; padding: 0.75rem 0.85rem; font-size: 0.92rem; color: #0f172a; background: #ffffff; }
.input:focus { outline: none; border-color: #4f46e5; box-shadow: 0 0 0 3px rgba(79, 70, 229, 0.16); }
.inputError { border-color: #dc2626; }
.fieldError { margin: 0.35rem 0 0; color: #dc2626; font-size: 0.78rem; }
.hint { margin: 0.9rem 0 0; color: #64748b; font-size: 0.8rem; }

.tableWrap { overflow-x: auto; }
.table { width: 100%; border-collapse: collapse; font-size: 0.85rem; }
.table th { text-align: left; font-size: 0.72rem; color: #64748b; font-weight: 600; padding: 0.5rem 0.75rem; border-bottom: 1px solid #e2e8f0; white-space: nowrap; }
.table td { padding: 0.55rem 0.75rem; border-bottom: 1px solid #f1f5f9; color: #0f172a; white-space: nowrap; }
.mono { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 0.82rem; }
.causeCell { white-space: normal; color: #64748b; font-size: 0.8rem; min-width: 16rem; }

.pill { display: inline-block; padding: 0.2rem 0.65rem; border-radius: 999px; font-size: 0.74rem; font-weight: 600; white-space: nowrap; }
.pillOk { background: #dcfce7; color: #15803d; }
.pillBad { background: #fee2e2; color: #b91c1c; }
.pillActive { background: #eef2ff; color: #4338ca; }
.pillMuted { background: #f1f5f9; color: #475569; }

.stageTop { display: flex; justify-content: space-between; align-items: baseline; gap: 1rem; }
.bigNumber { font-size: 1.8rem; font-weight: 700; color: #0f172a; font-variant-numeric: tabular-nums; }
.bigNumber small { font-size: 0.85rem; font-weight: 500; color: #64748b; }
.bar { display: flex; height: 8px; border-radius: 4px; overflow: hidden; background: #f1f5f9; margin: 0.75rem 0 1rem; }
.barOk { background: #16a34a; }
.barBad { background: #dc2626; }
.barActive { background: #818cf8; }
.kv { display: grid; grid-template-columns: max-content 1fr; gap: 0.35rem 1.5rem; margin: 0; font-size: 0.85rem; max-width: 34rem; }
.kv dt { color: #64748b; }
.kv dd { margin: 0; font-variant-numeric: tabular-nums; color: #0f172a; }
.causes h4 { margin: 1.25rem 0 0.4rem; font-size: 0.85rem; color: #475569; }
.causes ul { margin: 0; padding-left: 1.1rem; font-size: 0.85rem; color: #0f172a; }
.runError { margin: 0; padding: 0.75rem 1rem; border-radius: 10px; background: #fef2f2; color: #b91c1c; font-size: 0.85rem; white-space: pre-wrap; }

@media (max-width: 900px) {
  .layout { flex-direction: column; }
  .content { padding: 1.25rem 1rem; }
}
```

`web/frontend/src/page/tester/TesterSetupPage.tsx`:

```tsx
import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import Sidebar from '../../components/sidebar/Sidebar'
import Button from '../../components/button/button'
import NotificationContainer from '../../components/notifications/NotificationContainer'
import { useNotifications } from '../../hooks/useNotifications'
import { api, extractErrorMessage } from '../../apiClient'
import type { TesterFieldError, TesterProfile, TesterValidateResponse } from '../../api'
import { DEFAULT_TESTER_PROFILE } from './testerDefaults'
import styles from './tester.module.css'

const PREVIEW_ROWS = 10

// Sets a value at a dotted path ("network.n2.cidr") on a deep copy, so
// one onChange handler serves every field.
function setPath(profile: TesterProfile, path: string, value: string | number): TesterProfile {
  const next = structuredClone(profile) as unknown as Record<string, unknown>
  const keys = path.split('.')
  let node = next
  for (const key of keys.slice(0, -1)) node = node[key] as Record<string, unknown>
  node[keys[keys.length - 1]] = value
  return next as unknown as TesterProfile
}

function getPath(profile: TesterProfile, path: string): string | number {
  return path.split('.').reduce<unknown>((node, key) => (node as Record<string, unknown>)[key], profile) as string | number
}

interface FieldProps {
  label: string
  path: string
  profile: TesterProfile
  errors: TesterFieldError[]
  numeric?: boolean
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
}

export default function TesterSetupPage() {
  const navigate = useNavigate()
  const { errors, successes, addError, addSuccess, removeNotification } = useNotifications()
  const [profile, setProfile] = useState<TesterProfile>(DEFAULT_TESTER_PROFILE)
  const [isLoading, setIsLoading] = useState(true)
  const [validation, setValidation] = useState<TesterValidateResponse | null>(null)
  const [isBusy, setIsBusy] = useState(false)
  const validateSeq = useRef(0)

  useEffect(() => {
    api.testerProfileGet()
      .then((response) => {
        if (response.status === 200 && response.data) setProfile(response.data)
      })
      .catch((error) => addError(extractErrorMessage(error, 'Failed to load the saved profile')))
      .finally(() => setIsLoading(false))
  }, [addError])

  // Re-validate 400 ms after the last edit; a sequence number drops
  // answers that arrive after a newer request was sent.
  useEffect(() => {
    if (isLoading) return
    const seq = ++validateSeq.current
    const timer = window.setTimeout(() => {
      api.testerProfileValidate(profile)
        .then((response) => {
          if (seq === validateSeq.current) setValidation(response.data)
        })
        .catch((error) => {
          if (seq === validateSeq.current) {
            setValidation(null)
            addError(extractErrorMessage(error, 'Failed to validate the profile'))
          }
        })
    }, 400)
    return () => window.clearTimeout(timer)
  }, [profile, isLoading, addError])

  function handleChange(path: string, value: string | number) {
    setProfile((prev) => setPath(prev, path, value))
  }

  async function handleSave() {
    setIsBusy(true)
    try {
      await api.testerProfilePut(profile)
      addSuccess('Profile saved')
    } catch (error) {
      addError(extractErrorMessage(error, 'Failed to save the profile'))
    } finally {
      setIsBusy(false)
    }
  }

  async function handleStart() {
    setIsBusy(true)
    try {
      await api.testerProfilePut(profile)
      await api.testerRunStart(profile)
      navigate('/tester/run')
    } catch (error) {
      addError(extractErrorMessage(error, 'Failed to start the run'))
      setIsBusy(false)
    }
  }

  const fieldErrors = validation?.errors ?? []
  const plan = validation?.valid ? validation.plan : null
  const fieldProps = { profile, errors: fieldErrors, onChange: handleChange }

  return (
    <div className={styles.layout}>
      <NotificationContainer errors={errors} successes={successes} onClose={removeNotification} />
      <Sidebar />

      <main className={styles.content}>
        <header className={styles.header}>
          <div>
            <h2 className={styles.title}>Throughput Tester · Setup</h2>
            <p className={styles.subtitle}>Describe the gNBs to simulate. Phase 1 brings up N2 (SCTP + NG Setup) for every gNB and holds it until you stop the run.</p>
          </div>
          <div className={styles.headerActions}>
            <Button variant="secondary" onClick={handleSave} disabled={isLoading || isBusy}>Save</Button>
            <Button onClick={handleStart} disabled={isLoading || isBusy || !validation?.valid}>
              {isBusy ? 'Starting…' : 'Start run'}
            </Button>
          </div>
        </header>

        {isLoading ? (
          <p className={styles.emptyState}>Loading profile…</p>
        ) : (
          <>
            <section className={styles.card}>
              <h3 className={styles.cardTitle}>Scale</h3>
              <div className={styles.fieldGrid}>
                <Field label="Profile name" path="name" {...fieldProps} />
                <Field label="gNB count" path="scale.gnbCount" numeric {...fieldProps} />
                <Field label="UE count" path="scale.ueCount" numeric {...fieldProps} />
              </div>
              <p className={styles.hint}>
                UEs fill gNBs in order: each gNB takes up to {plan ? plan.uesPerGnb : '⌈UEs ÷ gNBs⌉'} UEs and the last one may be partly filled.
              </p>
            </section>

            <section className={styles.card}>
              <h3 className={styles.cardTitle}>gNB template</h3>
              <div className={styles.fieldGrid}>
                <Field label="First gNB ID (hex)" path="gnb.gnbIdStart" {...fieldProps} />
                <Field label="Name pattern ({i} = index)" path="gnb.namePattern" {...fieldProps} />
                <Field label="MCC" path="gnb.mcc" {...fieldProps} />
                <Field label="MNC" path="gnb.mnc" {...fieldProps} />
                <Field label="TAC (hex)" path="gnb.tac" {...fieldProps} />
                <Field label="SST" path="gnb.sst" numeric {...fieldProps} />
                <Field label="SD (hex, optional)" path="gnb.sd" {...fieldProps} />
              </div>
            </section>

            <section className={styles.card}>
              <h3 className={styles.cardTitle}>N2 · gNB ↔ AMF</h3>
              <div className={styles.fieldGrid}>
                <Field label="Local interface" path="network.n2.interface" {...fieldProps} />
                <Field label="gNB IP CIDR" path="network.n2.cidr" {...fieldProps} />
                <Field label="First gNB IP" path="network.n2.startIp" {...fieldProps} />
                <Field label="AMF IP" path="network.n2.amfIp" {...fieldProps} />
                <Field label="AMF port" path="network.n2.amfPort" numeric {...fieldProps} />
              </div>
              <p className={styles.hint}>fru-tester adds one IP per gNB to this interface when the run starts and removes them when it stops. IPs already on the host and the AMF IP are skipped.</p>
            </section>

            <section className={styles.card}>
              <h3 className={styles.cardTitle}>N3 · gNB ↔ UPF</h3>
              <div className={styles.fieldGrid}>
                <Field label="Local interface" path="network.n3.interface" {...fieldProps} />
                <Field label="gNB IP CIDR" path="network.n3.cidr" {...fieldProps} />
                <Field label="First gNB IP" path="network.n3.startIp" {...fieldProps} />
                <Field label="UPF IP" path="network.n3.upfIp" {...fieldProps} />
                <Field label="UPF port" path="network.n3.upfPort" numeric {...fieldProps} />
              </div>
              <p className={styles.hint}>Checked and planned now; N3 IPs are not configured until the data-plane phase.</p>
            </section>

            <section className={styles.card}>
              <h3 className={styles.cardTitle}>N2 setup timing</h3>
              <div className={styles.fieldGrid}>
                <Field label="Timeout per attempt (ms)" path="rates.n2.timeoutMs" numeric {...fieldProps} />
                <Field label="Retries" path="rates.n2.retries" numeric {...fieldProps} />
              </div>
              <p className={styles.hint}>A failed attempt with retries left goes to the back of the queue.</p>
            </section>

            <section className={styles.card}>
              <h3 className={styles.cardTitle}>Plan preview</h3>
              {!validation && <p className={styles.hint}>Checking…</p>}
              {validation && !validation.valid && (
                <p className={styles.fieldError}>Fix the {fieldErrors.length} highlighted field{fieldErrors.length === 1 ? '' : 's'} to see the plan.</p>
              )}
              {plan && (
                <div className={styles.tableWrap}>
                  <table className={styles.table}>
                    <thead>
                      <tr><th>#</th><th>Name</th><th>gNB ID</th><th>N2 IP</th><th>N3 IP</th><th>UEs</th></tr>
                    </thead>
                    <tbody>
                      {plan.gnbs.slice(0, PREVIEW_ROWS).map((g) => (
                        <tr key={g.index}>
                          <td>{g.index}</td>
                          <td>{g.name}</td>
                          <td className={styles.mono}>{g.gnbId}</td>
                          <td className={styles.mono}>{g.n2Ip}/{plan.n2Prefix}</td>
                          <td className={styles.mono}>{g.n3Ip}/{plan.n3Prefix}</td>
                          <td>{g.ueCount ? `${g.ueCount} (#${g.ueFirst}–${g.ueLast})` : '0'}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                  {plan.gnbs.length > PREVIEW_ROWS && (
                    <p className={styles.hint}>…and {plan.gnbs.length - PREVIEW_ROWS} more gNBs, last one {plan.gnbs[plan.gnbs.length - 1].name} at {plan.gnbs[plan.gnbs.length - 1].n2Ip}.</p>
                  )}
                </div>
              )}
            </section>
          </>
        )}
      </main>
    </div>
  )
}
```

- [ ] **Step 3: Route.** In `App.tsx`, import `TesterSetupPage from './page/tester/TesterSetupPage'` and add before the catch-all `path="*"` route:

```tsx
      <Route
        path="/tester"
        element={(
          <RequireAuth>
            <TesterSetupPage />
          </RequireAuth>
        )}
      />
```

- [ ] **Step 4: Verify**

Run: `cd web/frontend && yarn build`
Expected: the build succeeds.

Manual check: run `make run` and `make run-tester`, then open `http://<host>:8888/tester`.
- The sidebar shows a "THROUGHPUT TESTER" group with Setup and Run.
- Clearing "Profile name" shows "must not be empty" under that field within about half a second.
- With a real interface name in N2 and N3, "Plan preview" lists gNB-1…gNB-10 with consecutive N2 IPs starting at the First gNB IP. The host's own IPs and the AMF IP are skipped.
- A wrong interface name shows `no interface named "…" on this host`.
- Save, then reload: the edited values persist.

- [ ] **Step 5: Commit**

```bash
git add web/frontend/src/components/sidebar web/frontend/src/App.tsx web/frontend/src/page/tester
git commit -m "$(cat <<'EOF'
feat: throughput tester setup page

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 16: Run page

**Files:**
- Create: `web/frontend/src/page/tester/testerFormat.ts`, `web/frontend/src/page/tester/TesterRunPage.tsx`
- Modify: `web/frontend/src/App.tsx`

**Interfaces:**
- Consumes: Task 14's client and `TesterRunSnapshot`; `tester.module.css` from Task 15; fru-lab's `/api/tester/run/stream?token=` (Task 13).
- Produces: route `/tester/run`.

Run page behaviour:
- It does one `GET /api/tester/run`, then keeps a WebSocket open and reconnects 2 s after any drop. While the socket is down it shows "Live updates disconnected".
- Stop is enabled in `configuring`, `n2` and `running`.
- The N2 stage card shows accepted/expected, a stacked bar (accepted / failed / in flight), rejected, timed out, connection errors, retries, total time, average, p50/p95/p99, max and failure causes.
- The gNB table shows state, attempts, setup time and the last error.

- [ ] **Step 1: Write** `web/frontend/src/page/tester/testerFormat.ts`:

```ts
// Shared number formatting for the tester pages.
export function formatMs(ms: number): string {
  if (!ms) return '—'
  if (ms >= 1000) return `${(ms / 1000).toFixed(2)} s`
  return `${ms.toFixed(ms < 10 ? 2 : 0)} ms`
}

// A browser WebSocket can't set an Authorization header, so the stream
// takes the JWT as ?token= (checked by fru-lab's handleTesterStream),
// mirroring dashboard/terminalSocket.ts.
export function buildTesterStreamUrl(): string {
  const httpBase = import.meta.env.VITE_API_BASE_URL || `${window.location.protocol}//${window.location.hostname}:8888`
  const wsBase = httpBase.replace(/^http/, 'ws')
  const token = localStorage.getItem('token') ?? ''
  return `${wsBase}/api/tester/run/stream?token=${encodeURIComponent(token)}`
}
```

`web/frontend/src/page/tester/TesterRunPage.tsx`:

```tsx
import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import Sidebar from '../../components/sidebar/Sidebar'
import Button from '../../components/button/button'
import NotificationContainer from '../../components/notifications/NotificationContainer'
import { useNotifications } from '../../hooks/useNotifications'
import { api, extractErrorMessage } from '../../apiClient'
import type { TesterRunSnapshot, TesterStageSnapshot } from '../../api'
import { buildTesterStreamUrl, formatMs } from './testerFormat'
import styles from './tester.module.css'

const RECONNECT_MS = 2000

const STATE_LABELS: Record<TesterRunSnapshot['state'], string> = {
  idle: 'No run yet',
  configuring: 'Configuring gNB IPs',
  n2: 'N2 setup in progress',
  running: 'Holding N2',
  stopping: 'Stopping',
  stopped: 'Stopped',
  failed: 'Failed',
}

const ACTIVE_STATES: TesterRunSnapshot['state'][] = ['configuring', 'n2', 'running']

function StageCard({ title, stage }: { title: string, stage: TesterStageSnapshot }) {
  const pct = (n: number) => (stage.expected ? `${(n / stage.expected) * 100}%` : '0%')
  return (
    <section className={styles.card}>
      <div className={styles.stageTop}>
        <h3 className={styles.cardTitle}>{title}</h3>
        <span className={`${styles.pill} ${stage.done ? styles.pillOk : styles.pillActive}`}>
          {stage.done ? 'Done' : `${stage.inFlight} in flight`}
        </span>
      </div>
      <div className={styles.bigNumber}>
        {stage.accepted}<small> / {stage.expected} accepted</small>
      </div>
      <div className={styles.bar} aria-hidden="true">
        <span className={styles.barOk} style={{ width: pct(stage.accepted) }} />
        <span className={styles.barBad} style={{ width: pct(stage.rejected + stage.timedOut + stage.failed) }} />
        <span className={styles.barActive} style={{ width: pct(stage.inFlight) }} />
      </div>
      <dl className={styles.kv}>
        <dt>Rejected</dt><dd>{stage.rejected}</dd>
        <dt>Timed out</dt><dd>{stage.timedOut}</dd>
        <dt>Connection errors</dt><dd>{stage.failed}</dd>
        <dt>Retries</dt><dd>{stage.retries}</dd>
        <dt>Total time</dt><dd>{formatMs(stage.totalTimeMs)}</dd>
        <dt>Average</dt><dd>{formatMs(stage.avgMs)}</dd>
        <dt>p50 / p95 / p99</dt><dd>{formatMs(stage.p50Ms)} / {formatMs(stage.p95Ms)} / {formatMs(stage.p99Ms)}</dd>
        <dt>Max</dt><dd>{formatMs(stage.maxMs)}</dd>
      </dl>
      {stage.causes.length > 0 && (
        <div className={styles.causes}>
          <h4>Failure causes</h4>
          <ul>
            {stage.causes.map((c) => <li key={c.cause}><span className={styles.mono}>{c.cause}</span> × {c.count}</li>)}
          </ul>
        </div>
      )}
    </section>
  )
}

export default function TesterRunPage() {
  const { errors, successes, addError, addSuccess, removeNotification } = useNotifications()
  const [snapshot, setSnapshot] = useState<TesterRunSnapshot | null>(null)
  const [isConnected, setIsConnected] = useState(false)
  const [isStopping, setIsStopping] = useState(false)

  useEffect(() => {
    api.testerRunGet()
      .then((response) => setSnapshot(response.data))
      .catch((error) => addError(extractErrorMessage(error, 'Failed to load the run')))
  }, [addError])

  // Keep one stream open while the page is mounted; reconnect after drops
  // (fru-tester restart, network blip) so the numbers never silently freeze.
  useEffect(() => {
    let socket: WebSocket | null = null
    let retryTimer: number | undefined
    let closedByPage = false

    const connect = () => {
      socket = new WebSocket(buildTesterStreamUrl())
      socket.onopen = () => setIsConnected(true)
      socket.onmessage = (event) => setSnapshot(JSON.parse(event.data) as TesterRunSnapshot)
      socket.onclose = () => {
        setIsConnected(false)
        if (!closedByPage) retryTimer = window.setTimeout(connect, RECONNECT_MS)
      }
    }
    connect()
    return () => {
      closedByPage = true
      window.clearTimeout(retryTimer)
      socket?.close()
    }
  }, [])

  const handleStop = useCallback(async () => {
    setIsStopping(true)
    try {
      await api.testerRunStop()
      addSuccess('Stopping: closing N2 and removing gNB IPs')
    } catch (error) {
      addError(extractErrorMessage(error, 'Failed to stop the run'))
    } finally {
      setIsStopping(false)
    }
  }, [addError, addSuccess])

  const isActive = snapshot ? ACTIVE_STATES.includes(snapshot.state) : false

  return (
    <div className={styles.layout}>
      <NotificationContainer errors={errors} successes={successes} onClose={removeNotification} />
      <Sidebar />

      <main className={styles.content}>
        <header className={styles.header}>
          <div>
            <h2 className={styles.title}>Throughput Tester · Run</h2>
            <p className={styles.subtitle}>
              {snapshot && snapshot.state !== 'idle'
                ? <>Run <span className={styles.mono}>{snapshot.runId}</span> · {snapshot.profileName}{snapshot.startedAt && ` · started ${new Date(snapshot.startedAt).toLocaleString()}`}</>
                : <>No run has been started yet. <Link to="/tester">Set one up</Link>.</>}
            </p>
          </div>
          <div className={styles.headerActions}>
            {snapshot && <span className={`${styles.pill} ${isActive ? styles.pillActive : snapshot.state === 'failed' ? styles.pillBad : styles.pillMuted}`}>{STATE_LABELS[snapshot.state]}</span>}
            {!isConnected && <span className={`${styles.pill} ${styles.pillBad}`}>Live updates disconnected</span>}
            <Button variant="danger" onClick={handleStop} disabled={!isActive || isStopping}>
              {isStopping ? 'Stopping…' : 'Stop'}
            </Button>
          </div>
        </header>

        {snapshot?.error && <p className={styles.runError}>{snapshot.error}</p>}

        {snapshot && snapshot.state !== 'idle' && (
          <>
            <StageCard title="N2 setup" stage={snapshot.n2} />

            <section className={styles.card}>
              <h3 className={styles.cardTitle}>gNBs</h3>
              <div className={styles.tableWrap}>
                <table className={styles.table}>
                  <thead>
                    <tr><th>#</th><th>Name</th><th>N2 IP</th><th>State</th><th>Attempts</th><th>Setup time</th><th>Last error</th></tr>
                  </thead>
                  <tbody>
                    {snapshot.gnbs.map((g) => (
                      <tr key={g.index}>
                        <td>{g.index}</td>
                        <td>{g.name}</td>
                        <td className={styles.mono}>{g.n2Ip}</td>
                        <td><span className={`${styles.pill} ${g.state === 'up' ? styles.pillOk : g.state === 'failed' || g.state === 'lost' ? styles.pillBad : g.state === 'connecting' ? styles.pillActive : styles.pillMuted}`}>{g.state}</span></td>
                        <td>{g.attempts}</td>
                        <td>{g.state === 'up' || g.state === 'closed' ? formatMs(g.latencyMs) : '—'}</td>
                        <td className={styles.causeCell}>{g.cause || '—'}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </section>
          </>
        )}
      </main>
    </div>
  )
}
```

- [ ] **Step 2: Route.** In `App.tsx`, import `TesterRunPage from './page/tester/TesterRunPage'` and add after the `/tester` route:

```tsx
      <Route
        path="/tester/run"
        element={(
          <RequireAuth>
            <TesterRunPage />
          </RequireAuth>
        )}
      />
```

- [ ] **Step 3: Verify**

Run: `cd web/frontend && yarn build`
Expected: the build succeeds.

Manual check without a core:
1. Set AMF IP to an address nothing listens on (e.g. the first gNB IP + 50), set timeout to 1000 and retries to 1, then press Start run.
2. The page lands on `/tester/run` and moves `Configuring gNB IPs` → `N2 setup in progress` → `Holding N2`.
3. Every gNB ends `failed` with 2 attempts. The card shows Connection errors = 10 or Timed out = 10, depending on whether the peer answers with an SCTP ABORT, and the cause list groups them.
4. `ip -4 addr show <iface>` lists the 10 gNB IPs.
5. Press Stop: the state goes to `Stopped` and `ip -4 addr show <iface>` no longer lists them.

- [ ] **Step 4: Commit**

```bash
git add web/frontend/src/page/tester web/frontend/src/App.tsx
git commit -m "$(cat <<'EOF'
feat: throughput tester run page with live n2 stats

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 17: Docker packaging and docs

**Files:**
- Create: `docker/tester.Dockerfile`, `docker/tester.yaml`, `docs/tester-guide.md`
- Modify: `docker/docker-compose.yaml`, `docker/config.yaml`, `docker/build_image.sh`, `README.md`

**Interfaces:**
- Produces: image `alonza0314/fru-tester:<tag>`, and a compose service `fru-tester` on the host network with `NET_ADMIN`. fru-lab reaches it at `http://host.docker.internal:9100`.

fru-tester has to use the host network: it adds gNB IPs to host interfaces and dials the AMF from them. fru-lab stays on its bridge network and reaches the host through `host-gateway`. Because the tester API then listens on every host interface, `apiToken` is what protects it. Change it from the default in both config files.

- [ ] **Step 1: Write** `docker/tester.Dockerfile`:

```
# -------------------------------
# Stage 1: Build fru-tester
# -------------------------------
FROM golang:1.26.2 AS builder
WORKDIR /src/tester

COPY tester/go.mod tester/go.sum ./
RUN go mod download

COPY tester/ .
RUN CGO_ENABLED=0 go build -o /out/fru-tester .

# -------------------------------
# Stage 2: Runtime
# -------------------------------
FROM debian:bookworm-slim
WORKDIR /frutester

COPY --from=builder /out/fru-tester ./fru-tester
```

`docker/tester.yaml`:

```yaml
# fru-tester runs on the host network, so this port is reachable from the
# host's interfaces; apiToken must match docker/config.yaml backend.tester.
listen: "0.0.0.0:9100"
apiToken: "frulab-tester"

logger:
  level: "info"
```

- [ ] **Step 2: Compose.** In `docker/docker-compose.yaml`, add to the `fru-lab` service:

```yaml
    extra_hosts:
      # fru-tester runs on the host network; this name reaches the host
      - "host.docker.internal:host-gateway"
```

Add a new service under `services:`:

```yaml
  fru-tester:
    container_name: fru-tester
    image: alonza0314/fru-tester:latest
    command: ./fru-tester -c tester.yaml
    # host network: gNB IPs are added to host interfaces and SCTP to the
    # AMF leaves from them. NET_ADMIN is for adding/removing those IPs.
    # The sctp kernel module must be loaded on the host (modprobe sctp).
    network_mode: host
    cap_add:
      - NET_ADMIN
    volumes:
      - ./tester.yaml:/frutester/tester.yaml
```

In `docker/config.yaml` add under `backend:` after `frontendFilePath`:

```yaml

  tester:
    # fru-tester runs on the host network (see docker-compose.yaml)
    url: "http://host.docker.internal:9100"
    apiToken: "frulab-tester"
```

- [ ] **Step 3: Build both images.** In `docker/build_image.sh`, change `IMAGE_NAME` to keep both names, and build both:

```bash
IMAGE_NAME="alonza0314/fru-lab"
TESTER_IMAGE_NAME="alonza0314/fru-tester"
```

```bash
build_docker_image() {
    if ! docker build -f ${SCRIPT_DIR}/Dockerfile -t $IMAGE_NAME:$image_tag ${REPO_ROOT}; then
        echo "Failed to build the docker image"
        return 1
    fi
    if ! docker build -f ${SCRIPT_DIR}/tester.Dockerfile -t $TESTER_IMAGE_NAME:$image_tag ${REPO_ROOT}; then
        echo "Failed to build the fru-tester docker image"
        return 1
    fi
}
```

Also update the header comment's description to say it builds fru-lab and fru-tester.

- [ ] **Step 4: Docs.** Create `docs/tester-guide.md` covering:
- What Phase 1 does: N2 for N gNBs, with live statistics.
- Prerequisites: `modprobe sctp`, root or `NET_ADMIN`, and choosing a CIDR and start IP that no other container or host uses. fru-tester only skips IPs configured on *this* host, so it cannot see container IPs on a bridge.
- Running in dev: `make tester && make run-tester`, plus `backend.tester` in `config.yaml`.
- Running with compose.
- What every number on the N2 card means, using the definitions from `metrics.StageSnapshot`.
- The known limitations from this plan's Review Focus section.

In `README.md`, add `| Tester | \`make tester\` |`, `| Run tester | \`make run-tester\` |` and `| Test | \`make test\` |` rows to the Make table. Under "Guide", add `- [Throughput Tester Guide](docs/tester-guide.md) - configure and run the N2 load test.`

- [ ] **Step 5: Verify**

Run: `./docker/build_image.sh plancheck && docker run --rm alonza0314/fru-tester:plancheck ./fru-tester --help && docker rmi alonza0314/fru-lab:plancheck alonza0314/fru-tester:plancheck`
Expected: both images build, and `--help` prints `5G throughput tester engine (driven by fru-lab)`.

- [ ] **Step 6: Commit**

```bash
git add docker docs/tester-guide.md README.md
git commit -m "$(cat <<'EOF'
feat: fru-tester docker image, compose service and guide

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 18: End-to-end acceptance against free5GC

This is the spec's Phase 1 done criterion: "設定 10 個 gNB，畫面顯示 10 條 N2 的成功數、總時長與平均". It needs a running core, so it is a manual check and adds no code.

- [ ] **Step 1: Bring up a core.** Use fru-lab's Dashboard to deploy the basic free5GC template, or use any reachable free5GC. Note the AMF N2 IP (for fru-lab's template it lives on the `frulab-cn-ran` bridge, host side `10.0.1.1`, interface `docker-cn-ran`). Pick a start IP that no container on that bridge uses, e.g. `10.0.1.100`.
- [ ] **Step 2: Configure.** On `/tester`, set 10 gNBs, the N2 interface and CIDR (`docker-cn-ran`, `10.0.1.0/24`, start `10.0.1.100`), the AMF IP and port 38412, and an N3 interface and CIDR. Set PLMN 208/93, TAC 000001 and S-NSSAI 1/010203 to match the AMF's config. Confirm the preview shows 10 gNBs.
- [ ] **Step 3: Run.** Press Start run, and expect:
  - The state reaches `Holding N2`.
  - The N2 card shows `10 / 10 accepted`, Total time and Average are non-zero, and there are no failure causes.
  - All 10 rows show `up`.
  - The AMF log shows 10 NG Setup Requests from gNB-1…gNB-10.
- [ ] **Step 4: Negative check.** Stop, change MCC to `001` (a PLMN the AMF does not serve), and start again. Expect 10 rejected with a cause such as `misc(4)`, and every row `failed`.
- [ ] **Step 5: Drop check.** Start a valid run, then restart the AMF container. Expect every row to become `lost` with `association lost: …` while the run stays `Holding N2`. Then Stop.
- [ ] **Step 6: Cleanup check.** After Stop, `ip -4 addr show docker-cn-ran` must show none of the 10.0.1.100+ addresses.
- [ ] **Step 7:** Record the results (screenshots or numbers) in the PR description. Any deviation is a bug to fix before merging.
