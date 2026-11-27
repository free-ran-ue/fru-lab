package context

import (
	"backend/constant"
	"backend/logger"
	"backend/model"
	"bufio"
	"context"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// newTestIpam is an IPAM with its leases in a fresh database, the saved
// core networks cores, and no Docker network or host address (a test sets
// inspect / dockerNetworks / hostPrefixes when it needs them).
func newTestIpam(t *testing.T, cores []model.CoreNetwork) *ipamContext {
	t.Helper()
	db, err := newBboltDb(filepath.Join(t.TempDir(), "ipam.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Release() })
	m := newIpamContext(db, func() ([]model.CoreNetwork, error) { return cores, nil }, logger.NewBackendLogger("error", "", true))
	m.hostPrefixes = nil
	return m
}

// fixedNetworks stands in for Docker: network -> address -> container.
func fixedNetworks(nets map[string]map[string]string) func(context.Context, string) (map[netip.Addr]string, error) {
	return func(_ context.Context, name string) (map[netip.Addr]string, error) {
		out := map[netip.Addr]string{}
		for a, c := range nets[name] {
			out[netip.MustParseAddr(a)] = c
		}
		return out, nil
	}
}

var testCore = model.CoreNetwork{ID: "abcdefabcdef", Name: "second", Kind: model.CoreKindFree5gc, Slot: 2}

func poolIDs(plan []IpamPool) []string {
	out := make([]string, len(plan))
	for i, p := range plan {
		out[i] = p.ID
	}
	return out
}

// The plan is today's numbers: the lab core's cn-ran / SBI, ran-ue, and
// each free5gc core's pools from its slot; external cores and bad slots
// have none.
func TestIpamPlan(t *testing.T) {
	plan := ipamPlan([]model.CoreNetwork{
		{ID: "bbbbbbbbbbbb", Name: "third", Kind: model.CoreKindFree5gc, Slot: 3},
		testCore,
		{ID: "cccccccccccc", Name: "ext", Kind: model.CoreKindExternal, Slot: 4},
		{ID: "dddddddddddd", Name: "bad", Kind: model.CoreKindFree5gc, Slot: 10},
	})
	want := []string{"cn-ran", "sbi", "ran-ue", "cn-ran-abcdefabcdef", "sbi-abcdefabcdef", "cn-ran-bbbbbbbbbbbb", "sbi-bbbbbbbbbbbb"}
	if got := poolIDs(plan); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("pools %v", got)
	}
	for _, tc := range []struct {
		pool                    IpamPool
		network, subnet, bridge string
		amf, upf, iUpf, psaUpf  string
		n2First, n2Last, coreID string
	}{
		{plan[0], "frulab-cn-ran", "10.0.1.0/24", "docker-cn-ran", "10.0.1.3", "10.0.1.5", "10.0.1.5", "10.0.1.7", "10.0.1.20", "10.0.1.254", ""},
		{plan[3], "frulab-cn-ran-2", "10.0.102.0/24", "docker-cnran2", "10.0.102.3", "10.0.102.5", "10.0.102.5", "10.0.102.7", "10.0.102.20", "10.0.102.254", "abcdefabcdef"},
		{plan[5], "frulab-cn-ran-3", "10.0.103.0/24", "docker-cnran3", "10.0.103.3", "10.0.103.5", "10.0.103.5", "10.0.103.7", "10.0.103.20", "10.0.103.254", "bbbbbbbbbbbb"},
	} {
		p := tc.pool
		addr := func(role string) string { a, _ := p.ReservedAddr(role); return a.String() }
		r, _ := p.Range(RangeGnbN2)
		if p.Network != tc.network || p.Subnet.String() != tc.subnet || p.BridgeName != tc.bridge || p.CoreID != tc.coreID ||
			addr(RoleGateway) != strings.TrimSuffix(tc.subnet, "0/24")+"1" || addr(RoleAmf) != tc.amf || addr(RoleUpf) != tc.upf ||
			addr(RoleIUpf) != tc.iUpf || addr(RolePsaUpf) != tc.psaUpf || r.First.String() != tc.n2First || r.Last.String() != tc.n2Last || !p.managed {
			t.Errorf("%s: %+v", p.ID, p)
		}
	}
	for _, tc := range []struct {
		pool                    IpamPool
		network, subnet, bridge string
	}{
		{plan[1], "frulab-free5gc_sbi", "10.0.10.0/24", "docker-sbi"},
		{plan[4], "frulab-core-abcdefabcdef_sbi", "10.0.112.0/24", "docker-sbi2"},
	} {
		p := tc.pool
		if p.Network != tc.network || p.Subnet.String() != tc.subnet || p.BridgeName != tc.bridge || p.managed || len(p.Reserved) != 1 {
			t.Errorf("%s: %+v", p.ID, p)
		}
	}
	ranUe := plan[2]
	docker, _ := ranUe.Range(RangeDocker)
	gnb, _ := ranUe.Range(RangeGnbRan)
	if ranUe.Network != "frulab-ran-ue" || ranUe.Subnet.String() != "10.0.2.0/24" || gnb.First.String() != "10.0.2.200" || gnb.Last.String() != "10.0.2.250" ||
		docker.First.String() != "10.0.2.2" || docker.Last.String() != "10.0.2.127" || !ranUe.managed {
		t.Fatalf("ran-ue %+v", ranUe)
	}
	if plan[3].Purpose != "N2 / N3 between core network second and its gNBs" {
		t.Errorf("purpose %q", plan[3].Purpose)
	}
	// every slot's bridge names fit a Linux interface name
	for s := 1; s <= MaxCoreSlot; s++ {
		for _, p := range []IpamPool{coreCnRanPool("abcdefabcdef", s), coreSbiPool("abcdefabcdef", s)} {
			if len(p.BridgeName) > maxIfNameLen {
				t.Errorf("slot %d: bridge %s", s, p.BridgeName)
			}
		}
	}
	// the pools of the plan never overlap one another
	full := ipamPlan([]model.CoreNetwork{testCore})
	for i := range full {
		for j := i + 1; j < len(full); j++ {
			if full[i].Subnet.Overlaps(full[j].Subnet) {
				t.Errorf("%s overlaps %s", full[i].ID, full[j].ID)
			}
		}
	}
}

// ran-ue is created with the docker range as its IPAM ip-range: a clean
// CIDR inside .2 - .199, clear of the gNB range, 126 UEs.
func TestRanUeIPRange(t *testing.T) {
	p := ranUePool()
	n := p.network()
	if n.ipRange != "10.0.2.0/25" || n.gateway != "10.0.2.1" || n.subnet != "10.0.2.0/24" || n.bridgeName != "docker-ran-ue" {
		t.Fatalf("network %+v", n)
	}
	cfg, err := n.ipamConfig()
	if err != nil || cfg.IPRange.String() != "10.0.2.0/25" || cfg.Gateway.String() != "10.0.2.1" || cfg.Subnet.String() != "10.0.2.0/24" {
		t.Fatalf("ipam config %+v %v", cfg, err)
	}
	docker, _ := p.Range(RangeDocker)
	gnb, _ := p.Range(RangeGnbRan)
	if !p.DockerIPRange.Contains(docker.First) || !p.DockerIPRange.Contains(docker.Last) || p.DockerIPRange.Contains(gnb.First) ||
		docker.Last.Compare(netip.MustParseAddr("10.0.2.199")) > 0 {
		t.Fatalf("docker range %+v ip-range %s gnb %+v", docker, p.DockerIPRange, gnb)
	}
	capacity := 0
	for a := docker.First; a.Compare(docker.Last) <= 0; a = a.Next() {
		capacity++
	}
	if capacity != 126 {
		t.Fatalf("UE capacity %d", capacity)
	}
	// the cn-ran networks have no ip-range (every address there is fixed)
	if c := labCnRanPool().network(); c.ipRange != "" || c.gateway != "10.0.1.1" {
		t.Fatalf("cn-ran %+v", c)
	}
}

// testPool is a cn-ran pool whose gnb-n2 range is .1 - .8 (the gateway,
// AMF, UPF and PSA-UPF inside), to see reserved addresses skipped.
func testPool() IpamPool {
	p := labCnRanPool()
	p.Ranges = []IpamRange{{RangeGnbN2, netip.MustParseAddr("10.0.1.1"), netip.MustParseAddr("10.0.1.8")}}
	return p
}

func TestIpamAllocate(t *testing.T) {
	m := newTestIpam(t, nil)
	nets := map[string]map[string]string{"frulab-cn-ran": {"10.0.1.2": "someone", "10.0.1.3": "amf"}}
	m.inspect = fixedNetworks(nets)
	ctx := context.Background()
	p := testPool()
	alloc := func(owner string) string {
		t.Helper()
		a, err := m.Allocate(ctx, p, RangeGnbN2, owner, LeaseRoleN2)
		if err != nil {
			return "error: " + err.Error()
		}
		return a.String()
	}
	// .1 gateway, .2 held, .3 amf, .5 upf / i-upf, .7 psa-upf
	if got := alloc("gnb-aaaaaaaaaaaa"); got != "10.0.1.4" {
		t.Fatalf("first: %s", got)
	}
	if got := alloc("gnb-bbbbbbbbbbbb"); got != "10.0.1.6" {
		t.Fatalf("second: %s; want the next one past the leased and reserved ones", got)
	}
	// stable: the same owner gets its lease back, even with a lower one free
	delete(nets["frulab-cn-ran"], "10.0.1.2")
	if got := alloc("gnb-bbbbbbbbbbbb"); got != "10.0.1.6" {
		t.Fatalf("re-allocation: %s", got)
	}
	// the owner's own container holding an address does not make it taken
	nets["frulab-cn-ran"]["10.0.1.2"] = "gnb-cccccccccccc"
	if got := alloc("gnb-cccccccccccc"); got != "10.0.1.2" {
		t.Fatalf("own address: %s", got)
	}
	if got := alloc("gnb-dddddddddddd"); got != "10.0.1.8" {
		t.Fatalf("last one: %s", got)
	}
	if got := alloc("gnb-eeeeeeeeeeee"); !strings.Contains(got, "no free address left in range gnb-n2 (10.0.1.1 - 10.0.1.8) of frulab-cn-ran") {
		t.Fatalf("exhausted: %s", got)
	}
	// a lease another container took meanwhile is given up for a new one
	if err := m.IpamReleaseOwner("gnb-dddddddddddd"); err != nil {
		t.Fatal(err)
	}
	nets["frulab-cn-ran"]["10.0.1.4"] = "squatter"
	if got := alloc("gnb-aaaaaaaaaaaa"); got != "10.0.1.8" {
		t.Fatalf("squatted lease: %s", got)
	}
	if _, err := m.Allocate(ctx, p, "nope", "gnb-aaaaaaaaaaaa", LeaseRoleN2); err == nil {
		t.Fatal("an unknown range was accepted")
	}

	// a gNB moving to another core gives up its lease on the old one
	core := coreCnRanPool("abcdefabcdef", 2)
	if a, err := m.Allocate(ctx, core, RangeGnbN2, "gnb-bbbbbbbbbbbb", LeaseRoleN2); err != nil || a.String() != "10.0.102.20" {
		t.Fatalf("moved: %s %v", a, err)
	}
	l, ok, err := m.Lookup("gnb-bbbbbbbbbbbb", LeaseRoleN2)
	if err != nil || !ok || l.Pool != core.ID || l.Addr != "10.0.102.20" {
		t.Fatalf("lookup %+v %v %v", l, ok, err)
	}
	all, _ := m.leases()
	for _, l := range all {
		if l.Owner == "gnb-bbbbbbbbbbbb" && l.Pool == PoolCnRan {
			t.Fatalf("old lease kept: %+v", l)
		}
	}
	// an owner's leases of another role stay
	if a, err := m.Allocate(ctx, ranUePool(), RangeGnbRan, "gnb-bbbbbbbbbbbb", LeaseRoleRan); err != nil || a.String() != "10.0.2.200" {
		t.Fatalf("ran: %s %v", a, err)
	}
	if _, ok, _ := m.Lookup("gnb-bbbbbbbbbbbb", LeaseRoleN2); !ok {
		t.Fatal("the n2 lease went with the ran one")
	}
}

// Leases survive a restart (the database is reopened), and are released by
// owner, by pool and by core network.
func TestIpamLeasePersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ipam.db")
	open := func() (*ipamContext, func()) {
		db, err := newBboltDb(path)
		if err != nil {
			t.Fatal(err)
		}
		m := newIpamContext(db, func() ([]model.CoreNetwork, error) { return []model.CoreNetwork{testCore}, nil }, logger.NewBackendLogger("error", "", true))
		m.hostPrefixes = nil
		at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
		m.now = func() time.Time { return at }
		return m, func() { _ = db.Release() }
	}
	ctx := context.Background()
	m, closeDb := open()
	core := coreCnRanPool(testCore.ID, testCore.Slot)
	for _, step := range []struct {
		pool  IpamPool
		rng   string
		owner string
		role  string
	}{
		{labCnRanPool(), RangeGnbN2, "gnb-aaaaaaaaaaaa", LeaseRoleN2},
		{ranUePool(), RangeGnbRan, "gnb-aaaaaaaaaaaa", LeaseRoleRan},
		{core, RangeGnbN2, "gnb-bbbbbbbbbbbb", LeaseRoleN2},
		{ranUePool(), RangeGnbRan, "gnb-bbbbbbbbbbbb", LeaseRoleRan},
	} {
		if _, err := m.Allocate(ctx, step.pool, step.rng, step.owner, step.role); err != nil {
			t.Fatal(err)
		}
	}
	closeDb()

	m, closeDb = open()
	defer closeDb()
	l, ok, err := m.Lookup("gnb-aaaaaaaaaaaa", LeaseRoleRan)
	if err != nil || !ok || l != (IpamLease{Pool: PoolRanUe, Addr: "10.0.2.200", Owner: "gnb-aaaaaaaaaaaa", Role: LeaseRoleRan, Since: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)}) {
		t.Fatalf("after restart %+v %v %v", l, ok, err)
	}
	if a, _ := m.Allocate(ctx, ranUePool(), RangeGnbRan, "gnb-bbbbbbbbbbbb", LeaseRoleRan); a.String() != "10.0.2.201" {
		t.Fatalf("stable across a restart: %s", a)
	}

	if err := m.IpamReleaseCore(testCore.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := m.Lookup("gnb-bbbbbbbbbbbb", LeaseRoleN2); ok {
		t.Fatal("the core's pool kept its lease")
	}
	if _, ok, _ := m.Lookup("gnb-bbbbbbbbbbbb", LeaseRoleRan); !ok {
		t.Fatal("releasing a core released a ran-ue lease")
	}
	if err := m.IpamReleaseOwner("gnb-aaaaaaaaaaaa"); err != nil {
		t.Fatal(err)
	}
	if all, _ := m.leases(); len(all) != 1 || all[0].Owner != "gnb-bbbbbbbbbbbb" {
		t.Fatalf("left %+v", all)
	}
	if err := m.IpamReleasePools(PoolRanUe); err != nil {
		t.Fatal(err)
	}
	if all, _ := m.leases(); len(all) != 0 {
		t.Fatalf("left %+v", all)
	}
}

func findPool(t *testing.T, v *model.IpamView, id string) model.IpamPool {
	t.Helper()
	for _, p := range v.Pools {
		if p.ID == id {
			return p
		}
	}
	t.Fatalf("no pool %s", id)
	return model.IpamPool{}
}

func leaseAt(p model.IpamPool, addr string) *model.IpamLease {
	for i := range p.Leases {
		if p.Leases[i].Addr == addr {
			return &p.Leases[i]
		}
	}
	return nil
}

func TestIpamReconcileView(t *testing.T) {
	m := newTestIpam(t, []model.CoreNetwork{testCore})
	ctx := context.Background()
	for _, l := range []IpamLease{
		{Pool: PoolCnRan, Addr: "10.0.1.20", Owner: "gnb-aaaaaaaaaaaa", Role: LeaseRoleN2},
		{Pool: PoolCnRan, Addr: "10.0.1.21", Owner: "gnb-bbbbbbbbbbbb", Role: LeaseRoleN2},
		{Pool: PoolCnRan, Addr: "10.0.1.22", Owner: "gnb-cccccccccccc", Role: LeaseRoleN2},
		{Pool: "cn-ran-000000000000", Addr: "10.0.105.20", Owner: "gnb-dddddddddddd", Role: LeaseRoleN2},
	} {
		if err := m.putLease(l); err != nil {
			t.Fatal(err)
		}
	}
	m.inspect = fixedNetworks(map[string]map[string]string{
		"frulab-cn-ran": {
			"10.0.1.3":  "amf",
			"10.0.1.5":  "intruder",
			"10.0.1.20": "gnb-aaaaaaaaaaaa", // live
			"10.0.1.21": "squatter",         // leased to b
			"10.0.1.30": "gnb-eeeeeeeeeeee", // a gNB of fru-lab's without a lease
			"10.0.1.31": "stranger",
		},
		"frulab-ran-ue": {
			"10.0.2.2":   "ue-imsi-208930000000001",
			"10.0.2.210": "ue-imsi-208930000000002",
		},
		"frulab-cn-ran-2":    {"10.0.102.3": "c2-amf", "10.0.102.5": "amf"},
		"frulab-free5gc_sbi": {"10.0.10.2": "nrf"},
	})
	m.dockerNetworks = func(context.Context) (map[string][]netip.Prefix, error) {
		return map[string][]netip.Prefix{
			"corp":               {netip.MustParsePrefix("10.0.0.0/16")},
			"frulab-free5gc_sbi": {netip.MustParsePrefix("10.0.10.0/24")},
			"bridge":             {netip.MustParsePrefix("172.17.0.0/16")},
		}, nil
	}
	m.hostPrefixes = func() ([]namedPrefix, error) {
		return []namedPrefix{{What: "host route via tun0", Prefix: netip.MustParsePrefix("10.0.2.128/25")}}, nil
	}
	v, err := m.IpamView(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(v.Pools); got != 5 {
		t.Fatalf("%d pools", got)
	}
	cnRan := findPool(t, v, PoolCnRan)
	if len(cnRan.Reserved) != 5 || len(cnRan.Ranges) != 1 || cnRan.Ranges[0] != (model.IpamRange{Name: RangeGnbN2, First: "10.0.1.20", Last: "10.0.1.254"}) {
		t.Fatalf("cn-ran %+v", cnRan)
	}
	for addr, want := range map[string]model.IpamLease{
		"10.0.1.3":  {Owner: "free5gc/amf", Role: "amf", Live: true},
		"10.0.1.5":  {Owner: "intruder", Live: true},
		"10.0.1.20": {Owner: "gnb-aaaaaaaaaaaa", Role: LeaseRoleN2, Live: true},
		"10.0.1.21": {Owner: "gnb-bbbbbbbbbbbb", Role: LeaseRoleN2},
		"10.0.1.22": {Owner: "gnb-cccccccccccc", Role: LeaseRoleN2}, // stopped: kept
		"10.0.1.30": {Owner: "gnb-eeeeeeeeeeee", Role: LeaseRoleN2, Live: true},
		"10.0.1.31": {Owner: "stranger", Live: true},
	} {
		l := leaseAt(cnRan, addr)
		if l == nil || l.Owner != want.Owner || l.Role != want.Role || l.Live != want.Live {
			t.Errorf("%s: %+v; want %+v", addr, l, want)
		}
	}
	if l := leaseAt(cnRan, "10.0.1.22"); l == nil || l.Since == nil {
		t.Error("a recorded lease has no since")
	}
	if l := leaseAt(cnRan, "10.0.1.3"); l.Since != nil {
		t.Error("a discovered lease has a since")
	}
	if len(cnRan.Leases) != 7 || cnRan.Leases[0].Addr != "10.0.1.3" || cnRan.Leases[6].Addr != "10.0.1.31" {
		t.Errorf("leases not sorted: %+v", cnRan.Leases)
	}
	ranUe := findPool(t, v, PoolRanUe)
	if l := leaseAt(ranUe, "10.0.2.2"); l == nil || l.Owner != "ue-imsi-208930000000001" || !l.Live {
		t.Errorf("ue %+v", l)
	}
	core := findPool(t, v, "cn-ran-abcdefabcdef")
	if core.CoreID != testCore.ID || leaseAt(core, "10.0.102.3").Owner != "core-abcdefabcdef/amf" {
		t.Errorf("core pool %+v", core)
	}
	if l := leaseAt(findPool(t, v, PoolSbi), "10.0.10.2"); l == nil || l.Owner != "free5gc/nrf" {
		t.Errorf("sbi %+v", l)
	}

	wantWarnings := []string{
		"Container intruder holds 10.0.1.5 on frulab-cn-ran, reserved for upf / i-upf",
		"10.0.1.21 on frulab-cn-ran is leased to gnb-bbbbbbbbbbbb but held by container squatter",
		"Container stranger holds 10.0.1.31 in the gnb-n2 range of frulab-cn-ran, outside fru-lab's leases",
		"Pool cn-ran (frulab-cn-ran, 10.0.1.0/24) overlaps docker network corp (10.0.0.0/16)",
		"Pool ran-ue (frulab-ran-ue, 10.0.2.0/24) overlaps host route via tun0 (10.0.2.128/25)",
		"Container ue-imsi-208930000000002 holds 10.0.2.210 in the gnb-ran range of frulab-ran-ue, outside fru-lab's leases",
		"Container amf holds 10.0.102.5 on frulab-cn-ran-2, reserved for upf / i-upf",
		"10.0.105.20 is leased to gnb-dddddddddddd on pool cn-ran-000000000000, which is no longer in the address plan",
	}
	got := strings.Join(v.Warnings, "\n")
	for _, w := range wantWarnings {
		if !strings.Contains(got, w) {
			t.Errorf("missing warning %q in\n%s", w, got)
		}
	}
	for _, w := range []string{"gnb-eeeeeeeeeeee", "Container amf holds 10.0.1.3", "c2-amf", "frulab-free5gc_sbi (10.0.10", "172.17"} {
		if strings.Contains(got, w) {
			t.Errorf("unwanted warning about %s in\n%s", w, got)
		}
	}
	// a reconcile drops nothing
	if all, _ := m.leases(); len(all) != 4 {
		t.Fatalf("leases after reconcile %+v", all)
	}
}

// The deployed gNBs of a version without IPAM become leases, unless the
// address or the owner's role is already leased.
func TestIpamAdopt(t *testing.T) {
	m := newTestIpam(t, []model.CoreNetwork{testCore})
	if _, err := m.Allocate(context.Background(), labCnRanPool(), RangeGnbN2, "gnb-aaaaaaaaaaaa", LeaseRoleN2); err != nil {
		t.Fatal(err)
	}
	err := m.adopt([]IpamLease{
		{Pool: PoolCnRan, Addr: "10.0.1.40", Owner: "gnb-aaaaaaaaaaaa", Role: LeaseRoleN2}, // has one
		{Pool: PoolCnRan, Addr: "10.0.1.20", Owner: "gnb-bbbbbbbbbbbb", Role: LeaseRoleN2}, // address leased
		{Pool: "cn-ran-abcdefabcdef", Addr: "10.0.102.33", Owner: "gnb-bbbbbbbbbbbb", Role: LeaseRoleN2},
		{Pool: PoolRanUe, Addr: "10.0.2.203", Owner: "gnb-bbbbbbbbbbbb", Role: LeaseRoleRan},
		{Pool: PoolRanUe, Addr: "10.0.9.9", Owner: "gnb-cccccccccccc", Role: LeaseRoleRan},               // outside the pool
		{Pool: "cn-ran-000000000000", Addr: "10.0.105.20", Owner: "gnb-cccccccccccc", Role: LeaseRoleN2}, // no such pool
	})
	if err != nil {
		t.Fatal(err)
	}
	all, _ := m.leases()
	got := map[string]string{}
	for _, l := range all {
		got[l.Owner+" "+l.Role] = l.Pool + " " + l.Addr
	}
	want := map[string]string{
		"gnb-aaaaaaaaaaaa n2":  "cn-ran 10.0.1.20",
		"gnb-bbbbbbbbbbbb n2":  "cn-ran-abcdefabcdef 10.0.102.33",
		"gnb-bbbbbbbbbbbb ran": "ran-ue 10.0.2.203",
	}
	if len(got) != len(want) {
		t.Fatalf("leases %v", got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: %s; want %s", k, got[k], v)
		}
	}
}

func TestPoolOverlaps(t *testing.T) {
	plan := ipamPlan(nil)
	subnet := netip.MustParsePrefix("10.0.1.0/24")
	got := poolOverlaps(subnet, plan, map[string][]netip.Prefix{
		"frulab-cn-ran":  {subnet},                              // its own network
		"frulab-old_sbi": {netip.MustParsePrefix("10.0.0.0/8")}, // fru-lab's
		"vpn":            {netip.MustParsePrefix("10.0.1.128/25")},
		"other":          {netip.MustParsePrefix("10.0.3.0/24")},
		"wide":           {netip.MustParsePrefix("10.0.0.0/16")},
	}, []namedPrefix{
		{What: "host interface eth1", Prefix: netip.MustParsePrefix("10.0.1.0/26")},
		{What: "host interface eth0", Prefix: netip.MustParsePrefix("192.168.1.0/24")},
	})
	want := []string{"docker network vpn (10.0.1.128/25)", "docker network wide (10.0.0.0/16)", "host interface eth1 (10.0.1.0/26)"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v", got)
	}
}

// A network whose subnet overlaps a docker network or a host network is
// not created; the error names what it overlaps.
func TestEnsureNetworkRefusesAnOverlap(t *testing.T) {
	c := newTestCompose(t, &fakeCompose{})
	var created []sharedNetwork
	c.createNetworkFn = func(_ context.Context, n sharedNetwork) error {
		created = append(created, n)
		return nil
	}
	c.ipam.hostPrefixes = func() ([]namedPrefix, error) {
		return []namedPrefix{{What: "host interface wlan0", Prefix: netip.MustParsePrefix("10.0.2.0/24")}}, nil
	}
	err := c.ensureSharedNetworks(context.Background())
	if err == nil || !strings.Contains(err.Error(), "refusing to create network frulab-ran-ue: its subnet 10.0.2.0/24 overlaps host interface wlan0 (10.0.2.0/24)") {
		t.Fatalf("err %v", err)
	}
	if len(created) != 1 || created[0].name != "frulab-cn-ran" {
		t.Fatalf("created %+v", created)
	}

	c.ipam.dockerNetworks = func(context.Context) (map[string][]netip.Prefix, error) {
		return map[string][]netip.Prefix{"lab-of-someone": {netip.MustParsePrefix("10.0.102.0/24")}}, nil
	}
	err = c.UpCoreInstance(context.Background(), "abcdefabcdef", 2, 5002, constant.FREE5GC_TEMPLATE_BASIC)
	if err == nil || !strings.Contains(err.Error(), "docker network lab-of-someone (10.0.102.0/24)") {
		t.Fatalf("core: %v", err)
	}
	// the lab core's SBI network (its compose project's) is checked too
	c.ipam.dockerNetworks = func(context.Context) (map[string][]netip.Prefix, error) {
		return map[string][]netip.Prefix{"x": {netip.MustParsePrefix("10.0.10.0/24")}}, nil
	}
	if err := c.Up(context.Background(), constant.DEPLOY_TARGET_FREE5GC, constant.FREE5GC_TEMPLATE_BASIC); err == nil || !strings.Contains(err.Error(), "frulab-free5gc_sbi") {
		t.Fatalf("lab sbi: %v", err)
	}
}

func TestParseProcRoutes(t *testing.T) {
	in := "Iface\tDestination\tGateway \tFlags\tRefCnt\tUse\tMetric\tMask\t\tMTU\tWindow\tIRTT\n" +
		"eth0\t00000000\t0101A8C0\t0003\t0\t0\t100\t00000000\t0\t0\t0\n" + // default
		"eth0\t0001A8C0\t00000000\t0001\t0\t0\t100\t00FFFFFF\t0\t0\t0\n" + // 192.168.1.0/24
		"tun0\t0000000A\t00000000\t0001\t0\t0\t0\t000000FF\t0\t0\t0\n" + // 10.0.0.0/8
		"docker-cn-ran\t0001000A\t00000000\t0001\t0\t0\t0\t00FFFFFF\t0\t0\t0\n" +
		"br-1234\t000012AC\t00000000\t0001\t0\t0\t0\t0000FFFF\t0\t0\t0\n"
	got := parseProcRoutes(bufio.NewScanner(strings.NewReader(in)))
	if len(got) != 2 || got[0] != (namedPrefix{"host route via eth0", netip.MustParsePrefix("192.168.1.0/24")}) ||
		got[1] != (namedPrefix{"host route via tun0", netip.MustParsePrefix("10.0.0.0/8")}) {
		t.Fatalf("got %+v", got)
	}
}

// labTemplateLines are the lines of the free5gc templates (as they were
// before they became Go templates) with an address or a host-global name:
// the lab free5GC must render exactly these.
var labTemplateLines = []struct {
	variant, file string
	lines         []string
}{
	{"free5gc", "docker-compose.yaml", []string{
		"container_name: mongodb",
		"container_name: amf",
		"ipv4_address: 10.0.1.3",
		"container_name: ausf",
		"container_name: chf",
		"container_name: nrf",
		"container_name: nssf",
		"container_name: pcf",
		"container_name: smf",
		"container_name: udm",
		"container_name: udr",
		"container_name: upf",
		"ipv4_address: 10.0.1.5",
		"container_name: webconsole",
		"- subnet: 10.0.10.0/24",
		"com.docker.network.bridge.name: docker-sbi",
		"# (docker network create frulab-cn-ran --subnet 10.0.1.0/24",
		"#  -o com.docker.network.bridge.name=docker-cn-ran)",
		"name: frulab-cn-ran",
	}},
	{"free5gc", "config/amfcfg.yaml", []string{
		"- 10.0.1.3",
	}},
	{"free5gc", "config/smfcfg.yaml", []string{
		"- 10.0.1.5",
	}},
	{"free5gc", "config/upfcfg.yaml", []string{
		"- addr: 10.0.1.5",
	}},
	{"free5gc-ulcl", "docker-compose.yaml", []string{
		"container_name: mongodb",
		"container_name: amf",
		"ipv4_address: 10.0.1.3",
		"container_name: ausf",
		"container_name: chf",
		"container_name: nrf",
		"container_name: nssf",
		"container_name: pcf",
		"container_name: smf",
		"container_name: udm",
		"container_name: udr",
		"container_name: i-upf",
		"ipv4_address: 10.0.1.5",
		"container_name: psa-upf",
		"ipv4_address: 10.0.1.7",
		"container_name: webconsole",
		"- subnet: 10.0.10.0/24",
		"com.docker.network.bridge.name: docker-sbi",
		"# (docker network create frulab-cn-ran --subnet 10.0.1.0/24",
		"#  -o com.docker.network.bridge.name=docker-cn-ran)",
		"name: frulab-cn-ran",
	}},
	{"free5gc-ulcl", "config/amfcfg.yaml", []string{
		"- 10.0.1.3",
	}},
	{"free5gc-ulcl", "config/i-upfcfg.yaml", []string{
		"- addr: 10.0.1.5",
		"- addr: 10.0.1.5",
	}},
	{"free5gc-ulcl", "config/psa-upfcfg.yaml", []string{
		"- addr: 10.0.1.7",
	}},
	{"free5gc-ulcl", "config/smfcfg.yaml", []string{
		"- 10.0.1.5",
		"- 10.0.1.5",
		"- 10.0.1.7",
	}},
}

var reTemplateLine = regexp.MustCompile(`10\.0\.(1|10)\.|container_name|bridge\.name|frulab-cn-ran`)

// The lab free5GC renders the same addresses and names as before the
// templates had placeholders: no prefix, 10.0.1.x, 10.0.10.x,
// frulab-cn-ran, docker-sbi, docker-cn-ran.
func TestLabRenderKeepsTheTemplateAddresses(t *testing.T) {
	for _, variant := range []string{constant.FREE5GC_TEMPLATE_BASIC, constant.FREE5GC_TEMPLATE_ULCL} {
		c := newTestCompose(t, &fakeCompose{})
		if _, err := c.loadProject(context.Background(), constant.DEPLOY_TARGET_FREE5GC, variant, true); err != nil {
			t.Fatalf("%s: %v", variant, err)
		}
		dir := c.workDir + "/" + constant.DEPLOY_TARGET_FREE5GC
		dirVariant := map[string]string{constant.FREE5GC_TEMPLATE_BASIC: "free5gc", constant.FREE5GC_TEMPLATE_ULCL: "free5gc-ulcl"}[variant]
		checked := map[string]bool{}
		for _, g := range labTemplateLines {
			if g.variant != dirVariant {
				continue
			}
			checked[g.file] = true
			raw, err := os.ReadFile(filepath.Join(dir, g.file))
			if err != nil {
				t.Fatalf("%s: %v", variant, err)
			}
			var got []string
			for _, l := range strings.Split(string(raw), "\n") {
				if reTemplateLine.MatchString(l) {
					got = append(got, strings.TrimSpace(l))
				}
			}
			if strings.Join(got, "\n") != strings.Join(g.lines, "\n") {
				t.Errorf("%s %s:\n%s\nwant\n%s", variant, g.file, strings.Join(got, "\n"), strings.Join(g.lines, "\n"))
			}
		}
		// no other file has any, and no placeholder is left anywhere
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return err
			}
			rel, _ := filepath.Rel(dir, path)
			raw, _ := os.ReadFile(path)
			if strings.HasSuffix(rel, ".tmpl") || strings.Contains(string(raw), "{{") {
				t.Errorf("%s %s: not rendered", variant, rel)
			}
			if !checked[rel] && reTemplateLine.Match(raw) {
				t.Errorf("%s %s: unexpected address or name", variant, rel)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
