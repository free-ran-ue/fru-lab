package context

import (
	"backend/logger"
	"backend/model"
	"bufio"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// ipamLeaseBucket holds every lease, keyed <pool>/<addr>.
const ipamLeaseBucket = "ipam-leases"

// IpamLease is an address of a pool fru-lab handed to owner (a deploy
// target, gnb-<profile id>) for role (n2, ran). An owner holds at most one
// lease per role; it keeps it while stopped, so it gets the same address
// back, until it is released (its profile or core network deleted).
type IpamLease struct {
	Pool  string    `json:"pool"`
	Addr  string    `json:"addr"`
	Owner string    `json:"owner"`
	Role  string    `json:"role"`
	Since time.Time `json:"since"`
}

func leaseKey(pool, addr string) string {
	return pool + "/" + addr
}

// namedPrefix is an address range on the host that is not fru-lab's: what
// it is (a Docker network, a host interface, a host route) and its prefix.
type namedPrefix struct {
	What   string
	Prefix netip.Prefix
}

// ipamContext is the single source of truth for addresses: the plan (see
// ipamPlan.go), the leases persisted in the database, and a reconcile
// against what Docker says holds what.
type ipamContext struct {
	// guards every read-modify-write of the leases
	mu sync.Mutex
	db DbIf

	// the saved core networks (whose free5gc ones have pools)
	cores func() ([]model.CoreNetwork, error)
	// the address -> container name of a docker network (see
	// composeContext.NetworkAddresses); nil means nothing holds anything
	inspect func(ctx context.Context, network string) (map[netip.Addr]string, error)
	// the subnets of every docker network by name; nil for none
	dockerNetworks func(ctx context.Context) (map[string][]netip.Prefix, error)
	// the host's interface addresses and routes, Docker's own left out;
	// nil for none
	hostPrefixes func() ([]namedPrefix, error)

	now func() time.Time

	*logger.BackendLogger
}

func newIpamContext(db DbIf, cores func() ([]model.CoreNetwork, error), log *logger.BackendLogger) *ipamContext {
	return &ipamContext{db: db, cores: cores, hostPrefixes: hostNetworkPrefixes, now: time.Now, BackendLogger: log}
}

// plan is every pool of the address plan now.
func (m *ipamContext) plan() ([]IpamPool, error) {
	var cores []model.CoreNetwork
	if m.cores != nil {
		var err error
		if cores, err = m.cores(); err != nil {
			return nil, err
		}
	}
	return ipamPlan(cores), nil
}

func (m *ipamContext) held(ctx context.Context, network string) (map[netip.Addr]string, error) {
	if m.inspect == nil {
		return map[netip.Addr]string{}, nil
	}
	return m.inspect(ctx, network)
}

func (m *ipamContext) leases() ([]IpamLease, error) {
	kvs, err := m.db.List(ipamLeaseBucket)
	if err != nil {
		return nil, err
	}
	out := make([]IpamLease, 0, len(kvs))
	for _, kv := range kvs {
		var l IpamLease
		if err := json.Unmarshal(kv.Value, &l); err != nil {
			m.CtxLog.Warnf("Skipping unreadable IPAM lease %s: %v", kv.Key, err)
			continue
		}
		out = append(out, l)
	}
	return out, nil
}

func (m *ipamContext) putLease(l IpamLease) error {
	raw, err := json.Marshal(l)
	if err != nil {
		return err
	}
	return m.db.Put(ipamLeaseBucket, leaseKey(l.Pool, l.Addr), raw)
}

func (m *ipamContext) deleteLease(l IpamLease) error {
	return m.db.Delete(ipamLeaseBucket, leaseKey(l.Pool, l.Addr))
}

// Allocate is owner's address for role in range rangeName of pool: its
// lease when it has one there (the same address across redeploys), else
// the lowest address of the range that is not reserved, not leased and not
// held by another container on the pool's network, leased to owner. Any
// lease owner holds for role in another pool is released (a gNB moved to
// another core). A lease whose address another container took meanwhile is
// given up for a new one.
func (m *ipamContext) Allocate(ctx context.Context, pool IpamPool, rangeName, owner, role string) (netip.Addr, error) {
	r, ok := pool.Range(rangeName)
	if !ok {
		return netip.Addr{}, fmt.Errorf("pool %s has no range %s", pool.ID, rangeName)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	held, err := m.held(ctx, pool.Network)
	if err != nil {
		return netip.Addr{}, err
	}
	all, err := m.leases()
	if err != nil {
		return netip.Addr{}, err
	}
	leased := map[netip.Addr]bool{}
	for _, l := range all {
		if l.Owner == owner && l.Role == role {
			a, err := netip.ParseAddr(l.Addr)
			holder := held[a]
			if l.Pool == pool.ID && err == nil && r.Contains(a) && len(pool.reservedRoles(a)) == 0 && (holder == "" || holder == owner) {
				return a, nil
			}
			if l.Pool == pool.ID && holder != "" && holder != owner {
				m.CtxLog.Warnf("IPAM: %s of %s on %s is held by %s; leasing it a new address", l.Addr, owner, pool.Network, holder)
			}
			if err := m.deleteLease(l); err != nil {
				return netip.Addr{}, err
			}
			continue
		}
		if l.Pool == pool.ID {
			if a, err := netip.ParseAddr(l.Addr); err == nil {
				leased[a] = true
			}
		}
	}

	a, ok := PickFreeAddr(r.First, r.Last, func(a netip.Addr) bool {
		if leased[a] || len(pool.reservedRoles(a)) > 0 {
			return true
		}
		holder := held[a]
		return holder != "" && holder != owner
	})
	if !ok {
		return netip.Addr{}, fmt.Errorf("no free address left in range %s (%s - %s) of %s", rangeName, r.First, r.Last, pool.Network)
	}
	if err := m.putLease(IpamLease{Pool: pool.ID, Addr: a.String(), Owner: owner, Role: role, Since: m.now()}); err != nil {
		return netip.Addr{}, err
	}
	m.CtxLog.Infof("IPAM: leased %s on %s to %s (%s)", a, pool.Network, owner, role)
	return a, nil
}

// Lookup is the lease owner holds for role; ok false when it has none.
func (m *ipamContext) Lookup(owner, role string) (IpamLease, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	all, err := m.leases()
	if err != nil {
		return IpamLease{}, false, err
	}
	for _, l := range all {
		if l.Owner == owner && l.Role == role {
			return l, true, nil
		}
	}
	return IpamLease{}, false, nil
}

// releaseWhere drops every lease drop says to.
func (m *ipamContext) releaseWhere(drop func(IpamLease) bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	all, err := m.leases()
	if err != nil {
		return err
	}
	for _, l := range all {
		if !drop(l) {
			continue
		}
		if err := m.deleteLease(l); err != nil {
			return err
		}
		m.CtxLog.Infof("IPAM: released %s of pool %s (%s %s)", l.Addr, l.Pool, l.Owner, l.Role)
	}
	return nil
}

// IpamReleaseOwner releases every lease of owner (gnb-<profile id> when
// its profile is deleted).
func (m *ipamContext) IpamReleaseOwner(owner string) error {
	return m.releaseWhere(func(l IpamLease) bool { return l.Owner == owner })
}

// IpamReleasePools releases every lease of the pools ids.
func (m *ipamContext) IpamReleasePools(ids ...string) error {
	drop := map[string]bool{}
	for _, id := range ids {
		drop[id] = true
	}
	return m.releaseWhere(func(l IpamLease) bool { return drop[l.Pool] })
}

// IpamReleaseCore releases the pools of core network coreID (when it is
// deleted): every gNB lease on its cn-ran network.
func (m *ipamContext) IpamReleaseCore(coreID string) error {
	return m.IpamReleasePools(PoolCnRan+"-"+coreID, PoolSbi+"-"+coreID)
}

// adopt records leases for addresses fru-lab handed out before it had
// IPAM (a gNB deployed by an older version), unless owner already holds a
// lease for the role or the address is leased.
func (m *ipamContext) adopt(found []IpamLease) error {
	if len(found) == 0 {
		return nil
	}
	plan, err := m.plan()
	if err != nil {
		return err
	}
	pools := map[string]IpamPool{}
	for _, p := range plan {
		pools[p.ID] = p
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	all, err := m.leases()
	if err != nil {
		return err
	}
	byOwnerRole, byAddr := map[string]bool{}, map[string]bool{}
	for _, l := range all {
		byOwnerRole[l.Owner+"\x00"+l.Role] = true
		byAddr[leaseKey(l.Pool, l.Addr)] = true
	}
	for _, l := range found {
		p, ok := pools[l.Pool]
		a, err := netip.ParseAddr(l.Addr)
		if !ok || err != nil || !p.Subnet.Contains(a) || len(p.reservedRoles(a)) > 0 ||
			byOwnerRole[l.Owner+"\x00"+l.Role] || byAddr[leaseKey(l.Pool, l.Addr)] {
			continue
		}
		if l.Since.IsZero() {
			l.Since = m.now()
		}
		if err := m.putLease(l); err != nil {
			return err
		}
		byOwnerRole[l.Owner+"\x00"+l.Role], byAddr[leaseKey(l.Pool, l.Addr)] = true, true
		m.CtxLog.Infof("IPAM: adopted %s on pool %s for %s (%s)", l.Addr, l.Pool, l.Owner, l.Role)
	}
	return nil
}

// isOwnedNetwork tells whether docker network name is fru-lab's: one of
// the plan's, or any network a fru-lab compose project or an older
// version made (frulab-*).
func isOwnedNetwork(name string, plan []IpamPool) bool {
	if strings.HasPrefix(name, "frulab-") {
		return true
	}
	for _, p := range plan {
		if p.Network == name {
			return true
		}
	}
	return false
}

// poolOverlaps says what on the host overlaps subnet: another docker
// network that is not fru-lab's, a host interface address or route.
func poolOverlaps(subnet netip.Prefix, plan []IpamPool, dockerNets map[string][]netip.Prefix, host []namedPrefix) []string {
	var out []string
	names := make([]string, 0, len(dockerNets))
	for name := range dockerNets {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if isOwnedNetwork(name, plan) {
			continue
		}
		for _, p := range dockerNets[name] {
			if p.Overlaps(subnet) {
				out = append(out, fmt.Sprintf("docker network %s (%s)", name, p))
			}
		}
	}
	for _, h := range host {
		if h.Prefix.Overlaps(subnet) {
			out = append(out, fmt.Sprintf("%s (%s)", h.What, h.Prefix))
		}
	}
	return out
}

// overlaps says what on the host overlaps pool's subnet.
func (m *ipamContext) overlaps(ctx context.Context, pool IpamPool) ([]string, error) {
	plan, err := m.plan()
	if err != nil {
		return nil, err
	}
	var dockerNets map[string][]netip.Prefix
	if m.dockerNetworks != nil {
		if dockerNets, err = m.dockerNetworks(ctx); err != nil {
			return nil, err
		}
	}
	var host []namedPrefix
	if m.hostPrefixes != nil {
		if host, err = m.hostPrefixes(); err != nil {
			return nil, err
		}
	}
	return poolOverlaps(pool.Subnet, plan, dockerNets, host), nil
}

// checkOverlaps refuses pool's network when its subnet overlaps something
// on the host that is not fru-lab's.
func (m *ipamContext) checkOverlaps(ctx context.Context, pool IpamPool) error {
	found, err := m.overlaps(ctx, pool)
	if err != nil {
		return fmt.Errorf("failed to check what %s (%s) overlaps: %v", pool.Network, pool.Subnet, err)
	}
	if len(found) > 0 {
		return fmt.Errorf("refusing to create network %s: its subnet %s overlaps %s", pool.Network, pool.Subnet, strings.Join(found, ", "))
	}
	return nil
}

// coreServices are the container names (without the core's prefix) of
// the free5gc templates' services.
var coreServices = map[string]bool{
	"mongodb": true, "amf": true, "ausf": true, "chf": true, "nrf": true, "nssf": true, "pcf": true,
	"smf": true, "udm": true, "udr": true, "upf": true, "i-upf": true, "psa-upf": true, "webconsole": true,
}

// discoveredOwner is who container name holding a on pool is: a gNB, a UE
// (ue-<instance>) or a core NF (owner <core target>/<service>); anything
// else is its container name.
func discoveredOwner(pool IpamPool, name string, a netip.Addr) (owner, role string) {
	switch {
	case IsGnbInstance(name):
		switch pool.rangeOf(a) {
		case RangeGnbN2:
			return name, LeaseRoleN2
		case RangeGnbRan:
			return name, LeaseRoleRan
		}
		return name, ""
	case strings.HasPrefix(name, "ue-"):
		return name, "ue"
	case pool.coreTarget != "" && strings.HasPrefix(name, pool.containerPrefix) && coreServices[strings.TrimPrefix(name, pool.containerPrefix)]:
		svc := strings.TrimPrefix(name, pool.containerPrefix)
		return pool.coreTarget + "/" + svc, svc
	}
	return name, ""
}

// ipamState is a reconcile's result: every pool with its leases (recorded
// and discovered), and the warnings.
type ipamState struct {
	pools    []model.IpamPool
	warnings []string
}

// reconcile compares the leases with what holds which address on every
// pool's network. Nothing is dropped: a stopped owner keeps its lease. It
// warns about an address held by a container that is not its lease's
// owner, a container that is not a gNB on a gNB range, one that is not
// the NF it is for on a reserved address, a lease of a pool no longer in
// the plan and a pool overlapping something on the host.
func (m *ipamContext) reconcile(ctx context.Context) (*ipamState, error) {
	plan, err := m.plan()
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	all, err := m.leases()
	m.mu.Unlock()
	if err != nil {
		return nil, err
	}
	byPool := map[string][]IpamLease{}
	for _, l := range all {
		byPool[l.Pool] = append(byPool[l.Pool], l)
	}

	st := &ipamState{pools: make([]model.IpamPool, 0, len(plan)), warnings: []string{}}
	warn := func(format string, args ...any) { st.warnings = append(st.warnings, fmt.Sprintf(format, args...)) }

	var dockerNets map[string][]netip.Prefix
	if m.dockerNetworks != nil {
		if dockerNets, err = m.dockerNetworks(ctx); err != nil {
			warn("Cannot list the Docker networks to check for overlaps: %v", err)
		}
	}
	var host []namedPrefix
	if m.hostPrefixes != nil {
		if host, err = m.hostPrefixes(); err != nil {
			warn("Cannot read the host's addresses and routes to check for overlaps: %v", err)
		}
	}

	inPlan := map[string]bool{}
	for _, pool := range plan {
		inPlan[pool.ID] = true
		view := model.IpamPool{
			ID: pool.ID, Network: pool.Network, Subnet: pool.Subnet.String(), Purpose: pool.Purpose, CoreID: pool.CoreID,
			Reserved: make([]model.IpamReserved, 0, len(pool.Reserved)),
			Ranges:   make([]model.IpamRange, 0, len(pool.Ranges)),
			Leases:   []model.IpamLease{},
		}
		for _, r := range pool.Reserved {
			view.Reserved = append(view.Reserved, model.IpamReserved{Addr: r.Addr.String(), Role: r.Role})
		}
		for _, r := range pool.Ranges {
			view.Ranges = append(view.Ranges, model.IpamRange{Name: r.Name, First: r.First.String(), Last: r.Last.String()})
		}

		held, err := m.held(ctx, pool.Network)
		if err != nil {
			warn("Cannot inspect network %s: %v", pool.Network, err)
			held = map[netip.Addr]string{}
		}
		covered := map[netip.Addr]bool{}
		for _, l := range byPool[pool.ID] {
			a, _ := netip.ParseAddr(l.Addr)
			holder := held[a]
			covered[a] = true
			since := l.Since
			view.Leases = append(view.Leases, model.IpamLease{Addr: l.Addr, Owner: l.Owner, Role: l.Role, Since: &since, Live: holder == l.Owner})
			if holder != "" && holder != l.Owner {
				warn("%s on %s is leased to %s but held by container %s", l.Addr, pool.Network, l.Owner, holder)
			}
		}
		addrs := make([]netip.Addr, 0, len(held))
		for a := range held {
			addrs = append(addrs, a)
		}
		sort.Slice(addrs, func(i, j int) bool { return addrs[i].Less(addrs[j]) })
		for _, a := range addrs {
			if covered[a] {
				continue
			}
			name := held[a]
			owner, role := discoveredOwner(pool, name, a)
			view.Leases = append(view.Leases, model.IpamLease{Addr: a.String(), Owner: owner, Role: role, Live: true})
			if roles := pool.reservedRoles(a); len(roles) > 0 {
				okHolder := false
				for _, r := range roles {
					okHolder = okHolder || name == pool.containerPrefix+r
				}
				if !okHolder {
					warn("Container %s holds %s on %s, reserved for %s", name, a, pool.Network, strings.Join(roles, " / "))
				}
				continue
			}
			// only gNBs belong in a gNB range (a UE there was handed it by
			// Docker on a ran-ue network created without an ip-range)
			if rng := pool.rangeOf(a); !IsGnbInstance(name) && (rng == RangeGnbN2 || rng == RangeGnbRan) {
				warn("Container %s holds %s in the %s range of %s, outside fru-lab's leases", name, a, rng, pool.Network)
			}
		}
		sort.SliceStable(view.Leases, func(i, j int) bool {
			ai, _ := netip.ParseAddr(view.Leases[i].Addr)
			aj, _ := netip.ParseAddr(view.Leases[j].Addr)
			return ai.Less(aj)
		})

		for _, o := range poolOverlaps(pool.Subnet, plan, dockerNets, host) {
			warn("Pool %s (%s, %s) overlaps %s", pool.ID, pool.Network, pool.Subnet, o)
		}
		st.pools = append(st.pools, view)
	}
	for _, l := range all {
		if !inPlan[l.Pool] {
			warn("%s is leased to %s on pool %s, which is no longer in the address plan", l.Addr, l.Owner, l.Pool)
		}
	}
	return st, nil
}

// IpamView is GET /api/ipam: the plan and its leases after a reconcile.
func (m *ipamContext) IpamView(ctx context.Context) (*model.IpamView, error) {
	st, err := m.reconcile(ctx)
	if err != nil {
		return nil, err
	}
	return &model.IpamView{Pools: st.pools, Warnings: st.warnings}, nil
}

// PickFreeAddr is the lowest address in first .. last that is not used.
func PickFreeAddr(first, last netip.Addr, used func(netip.Addr) bool) (netip.Addr, bool) {
	for a := first; a.IsValid() && a.Compare(last) <= 0; a = a.Next() {
		if !used(a) {
			return a, true
		}
	}
	return netip.Addr{}, false
}

// isDockerInterface tells whether host interface name is one Docker made
// (its bridges, fru-lab's docker-* ones included, and veths): their
// addresses and routes are the Docker networks themselves.
func isDockerInterface(name string) bool {
	for _, prefix := range []string{"docker", "br-", "veth"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// hostNetworkPrefixes are the host's IPv4 interface networks and routes
// (/proc/net/route, when there is one), loopback, the default route and
// Docker's own interfaces left out.
func hostNetworkPrefixes() ([]namedPrefix, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var out []namedPrefix
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 || isDockerInterface(iface.Name) {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			n, ok := a.(*net.IPNet)
			if !ok || n.IP.To4() == nil {
				continue
			}
			ip, _ := netip.AddrFromSlice(n.IP.To4())
			ones, _ := n.Mask.Size()
			out = append(out, namedPrefix{What: "host interface " + iface.Name, Prefix: netip.PrefixFrom(ip, ones).Masked()})
		}
	}
	if f, err := os.Open("/proc/net/route"); err == nil {
		defer func() { _ = f.Close() }()
		out = append(out, parseProcRoutes(bufio.NewScanner(f))...)
	}
	return out, nil
}

// parseProcRoutes reads /proc/net/route: the IPv4 routes but the default
// one and those over Docker's interfaces.
func parseProcRoutes(sc *bufio.Scanner) []namedPrefix {
	var out []namedPrefix
	first := true
	for sc.Scan() {
		if first {
			first = false
			continue
		}
		f := strings.Fields(sc.Text())
		if len(f) < 8 || isDockerInterface(f[0]) || f[0] == "lo" {
			continue
		}
		dst, err1 := hex.DecodeString(f[1])
		mask, err2 := hex.DecodeString(f[7])
		if err1 != nil || err2 != nil || len(dst) != 4 || len(mask) != 4 {
			continue
		}
		// the kernel writes them little-endian
		var mb, db [4]byte
		binary.BigEndian.PutUint32(mb[:], binary.LittleEndian.Uint32(mask))
		binary.BigEndian.PutUint32(db[:], binary.LittleEndian.Uint32(dst))
		ones, bits := net.IPMask(mb[:]).Size()
		if ones == 0 || bits == 0 {
			// the default route, or a mask that is not one
			continue
		}
		out = append(out, namedPrefix{What: "host route via " + f[0], Prefix: netip.PrefixFrom(netip.AddrFrom4(db), ones).Masked()})
	}
	return out
}
