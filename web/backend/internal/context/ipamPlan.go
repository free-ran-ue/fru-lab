package context

import (
	"backend/constant"
	"backend/model"
	"fmt"
	"net/netip"
	"sort"
	"text/template"
)

// The address plan: every Docker network fru-lab puts containers on is an
// IPAM pool with its subnet, its reserved addresses (the gateway and every
// core NF's fixed address the free5gc templates use) and its named ranges
// addresses are handed out from.
//
//	pool              network                 subnet             reserved                      ranges
//	cn-ran            frulab-cn-ran           10.0.1.0/24        .1 gw .3 amf .5 upf/i-upf     gnb-n2 .20-.254
//	                                                             .7 psa-upf
//	sbi               frulab-free5gc_sbi      10.0.10.0/24       .1 gw                         docker .2-.254
//	ran-ue            frulab-ran-ue           10.0.2.0/24        .1 gw                         docker .2-.127 (UEs)
//	                                                                                           gnb-ran .200-.250
//	cn-ran-<coreId>   frulab-cn-ran-<slot>    10.0.(100+slot).0/24  like cn-ran                like cn-ran
//	sbi-<coreId>      frulab-core-<id>_sbi    10.0.(110+slot).0/24  .1 gw                      docker .2-.254
//
// The SBI networks belong to their core's compose project (created and
// removed with it); fru-lab creates the cn-ran and ran-ue networks itself.
const (
	PoolCnRan = "cn-ran"
	PoolSbi   = "sbi"
	PoolRanUe = "ran-ue"

	// gNB N2/N3 addresses on a cn-ran pool
	RangeGnbN2 = "gnb-n2"
	// where UEs dial a gNB, on ran-ue
	RangeGnbRan = "gnb-ran"
	// what Docker hands out itself (UEs on ran-ue, the NFs on SBI)
	RangeDocker = "docker"

	RoleGateway = "gateway"
	RoleAmf     = "amf"
	// the basic template's UPF
	RoleUpf = "upf"
	// the ulcl template's UPFs: i-upf takes the basic UPF's address
	RoleIUpf   = "i-upf"
	RolePsaUpf = "psa-upf"

	// the roles of a gNB's leases
	LeaseRoleN2  = "n2"
	LeaseRoleRan = "ran"

	CnRanNetwork = "frulab-cn-ran"
	RanUeNetwork = "frulab-ran-ue"

	labFree5gcProject = "frulab-free5gc"
)

// IpamReserved is a fixed address of a pool and who it is for.
type IpamReserved struct {
	Addr netip.Addr
	Role string
}

// IpamRange is a named range of a pool, first .. last.
type IpamRange struct {
	Name        string
	First, Last netip.Addr
}

// Contains tells whether a is in r.
func (r IpamRange) Contains(a netip.Addr) bool {
	return a.Compare(r.First) >= 0 && a.Compare(r.Last) <= 0
}

// IpamPool is one Docker network of the plan.
type IpamPool struct {
	ID      string
	Network string
	Subnet  netip.Prefix
	Purpose string
	// the core network it belongs to; "" for the lab free5GC's and the
	// shared ones
	CoreID     string
	BridgeName string
	Reserved   []IpamReserved
	Ranges     []IpamRange
	// the IPAM ip-range fru-lab creates the network with, so Docker hands
	// out addresses from the docker range only; invalid for none
	DockerIPRange netip.Prefix
	// the deploy target the core NFs on it belong to (free5gc, core-<id>)
	// and their container name prefix ("" for the lab core, c<slot>-)
	coreTarget      string
	containerPrefix string
	// fru-lab creates and removes the network itself
	managed bool
}

// Gateway is the pool's gateway address.
func (p IpamPool) Gateway() netip.Addr {
	a, _ := p.ReservedAddr(RoleGateway)
	return a
}

// ReservedAddr is the address reserved for role.
func (p IpamPool) ReservedAddr(role string) (netip.Addr, bool) {
	for _, r := range p.Reserved {
		if r.Role == role {
			return r.Addr, true
		}
	}
	return netip.Addr{}, false
}

// reservedRoles are the roles reserved at a; none when a is not reserved.
func (p IpamPool) reservedRoles(a netip.Addr) []string {
	var out []string
	for _, r := range p.Reserved {
		if r.Addr == a {
			out = append(out, r.Role)
		}
	}
	return out
}

// Range is the pool's range name.
func (p IpamPool) Range(name string) (IpamRange, bool) {
	for _, r := range p.Ranges {
		if r.Name == name {
			return r, true
		}
	}
	return IpamRange{}, false
}

// rangeOf is the name of the range a is in; "" for none.
func (p IpamPool) rangeOf(a netip.Addr) string {
	for _, r := range p.Ranges {
		if r.Contains(a) {
			return r.Name
		}
	}
	return ""
}

// network is how fru-lab creates the pool's Docker network.
func (p IpamPool) network() sharedNetwork {
	n := sharedNetwork{name: p.Network, subnet: p.Subnet.String(), gateway: p.Gateway().String(), bridgeName: p.BridgeName}
	if p.DockerIPRange.IsValid() {
		n.ipRange = p.DockerIPRange.String()
	}
	return n
}

// subnet24 is 10.0.<third>.0/24.
func subnet24(third int) netip.Prefix {
	return netip.PrefixFrom(netip.AddrFrom4([4]byte{10, 0, byte(third), 0}), 24)
}

// hostOf is address .host of /24 subnet p.
func hostOf(p netip.Prefix, host byte) netip.Addr {
	b := p.Masked().Addr().As4()
	b[3] = host
	return netip.AddrFrom4(b)
}

// cnRanPool is a core's cn-ran pool: the AMF / UPF addresses the free5gc
// templates use and the gNB N2/N3 range.
func cnRanPool(id, coreID, network, bridge string, subnet netip.Prefix, purpose, coreTarget, containerPrefix string) IpamPool {
	return IpamPool{
		ID: id, Network: network, Subnet: subnet, Purpose: purpose, CoreID: coreID, BridgeName: bridge,
		Reserved: []IpamReserved{
			{hostOf(subnet, 1), RoleGateway},
			{hostOf(subnet, 3), RoleAmf},
			{hostOf(subnet, 5), RoleUpf},
			{hostOf(subnet, 5), RoleIUpf},
			{hostOf(subnet, 7), RolePsaUpf},
		},
		Ranges:     []IpamRange{{RangeGnbN2, hostOf(subnet, 20), hostOf(subnet, 254)}},
		coreTarget: coreTarget, containerPrefix: containerPrefix, managed: true,
	}
}

// sbiPool is a core's SBI pool: no fixed address, Docker hands them out.
func sbiPool(id, coreID, network, bridge string, subnet netip.Prefix, purpose, coreTarget, containerPrefix string) IpamPool {
	return IpamPool{
		ID: id, Network: network, Subnet: subnet, Purpose: purpose, CoreID: coreID, BridgeName: bridge,
		Reserved:   []IpamReserved{{hostOf(subnet, 1), RoleGateway}},
		Ranges:     []IpamRange{{RangeDocker, hostOf(subnet, 2), hostOf(subnet, 254)}},
		coreTarget: coreTarget, containerPrefix: containerPrefix,
	}
}

// labCnRanPool is the lab free5GC's cn-ran network.
func labCnRanPool() IpamPool {
	return cnRanPool(PoolCnRan, "", CnRanNetwork, "docker-cn-ran", subnet24(1),
		"N2 / N3 between the lab free5GC and its gNBs", constant.DEPLOY_TARGET_FREE5GC, "")
}

// labSbiPool is the lab free5GC's SBI network (its compose project's).
func labSbiPool() IpamPool {
	return sbiPool(PoolSbi, "", labFree5gcProject+"_sbi", "docker-sbi", subnet24(10),
		"Service-based interface between the lab free5GC's NFs", constant.DEPLOY_TARGET_FREE5GC, "")
}

// ranUeDockerIPRange is where Docker hands out ran-ue addresses (to the
// UEs): .0 - .127, of which .2 - .127 are usable (.0 is the network, .1
// the gateway), so 126 UEs; .128 - .199 and .251 - .254 stay unused, the
// gNBs' .200 - .250 cannot be handed out by Docker.
var ranUeDockerIPRange = netip.MustParsePrefix("10.0.2.0/25")

// ranUePool is the shared network between every gNB and the UEs.
func ranUePool() IpamPool {
	subnet := subnet24(2)
	return IpamPool{
		ID: PoolRanUe, Network: RanUeNetwork, Subnet: subnet, BridgeName: "docker-ran-ue",
		Purpose:  "Between the gNBs and the UEs (shared by every core)",
		Reserved: []IpamReserved{{hostOf(subnet, 1), RoleGateway}},
		Ranges: []IpamRange{
			{RangeDocker, hostOf(subnet, 2), hostOf(subnet, 127)},
			{RangeGnbRan, hostOf(subnet, 200), hostOf(subnet, 250)},
		},
		DockerIPRange: ranUeDockerIPRange,
		managed:       true,
	}
}

// sharedPools are the networks fru-lab creates up front: the lab core's
// cn-ran and ran-ue.
func sharedPools() []IpamPool {
	return []IpamPool{labCnRanPool(), ranUePool()}
}

// coreCnRanPool is the cn-ran pool of free5gc core network coreID in slot
// slot: frulab-cn-ran-<slot>, 10.0.(100+slot).0/24.
func coreCnRanPool(coreID string, slot int) IpamPool {
	return cnRanPool(PoolCnRan+"-"+coreID, coreID, CoreCnRanNetwork(slot), fmt.Sprintf("docker-cnran%d", slot), subnet24(100+slot),
		fmt.Sprintf("N2 / N3 between free5GC core network slot %d and its gNBs", slot), CoreInstanceTarget(coreID), coreContainerPrefix(slot))
}

// coreSbiPool is the SBI pool of free5gc core network coreID in slot slot:
// 10.0.(110+slot).0/24, its compose project's network.
func coreSbiPool(coreID string, slot int) IpamPool {
	target := CoreInstanceTarget(coreID)
	return sbiPool(PoolSbi+"-"+coreID, coreID, coreInstanceProjectName(target)+"_sbi", fmt.Sprintf("docker-sbi%d", slot), subnet24(110+slot),
		fmt.Sprintf("Service-based interface between the NFs of free5GC core network slot %d", slot), target, coreContainerPrefix(slot))
}

// coreContainerPrefix starts every container name of the core in slot.
func coreContainerPrefix(slot int) string {
	return fmt.Sprintf("c%d-", slot)
}

// ipamPlan is every pool: the lab core's, ran-ue, then each free5gc core
// network's (by slot).
func ipamPlan(cores []model.CoreNetwork) []IpamPool {
	plan := []IpamPool{labCnRanPool(), labSbiPool(), ranUePool()}
	list := make([]model.CoreNetwork, 0, len(cores))
	for _, n := range cores {
		if n.IsFree5gc() && n.Slot >= 1 && n.Slot <= MaxCoreSlot && IsCoreInstance(CoreInstanceTarget(n.ID)) {
			list = append(list, n)
		}
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].Slot < list[j].Slot })
	seen := map[int]bool{}
	for _, n := range list {
		if seen[n.Slot] {
			continue
		}
		seen[n.Slot] = true
		cnRan, sbi := coreCnRanPool(n.ID, n.Slot), coreSbiPool(n.ID, n.Slot)
		cnRan.Purpose = "N2 / N3 between core network " + n.Name + " and its gNBs"
		sbi.Purpose = "Service-based interface between the NFs of core network " + n.Name
		plan = append(plan, cnRan, sbi)
	}
	return plan
}

// coreRenderNet is one network of a core as its templates see it.
type coreRenderNet struct {
	Network, Subnet, Bridge string
}

// coreRender is what the free5gc templates (*.tmpl) are rendered with:
// the container name prefix, the cn-ran and SBI networks, and {{addr
// "<role>"}} the address reserved for an NF on cn-ran.
type coreRender struct {
	Prefix string
	CnRan  coreRenderNet
	Sbi    coreRenderNet
	cnRan  IpamPool
}

// newCoreRender is the render data of the core whose networks are cnRan
// and sbi.
func newCoreRender(cnRan, sbi IpamPool) (*coreRender, error) {
	for _, p := range []IpamPool{cnRan, sbi} {
		if len(p.BridgeName) > maxIfNameLen {
			return nil, fmt.Errorf("bridge name %s is longer than %d characters", p.BridgeName, maxIfNameLen)
		}
	}
	return &coreRender{
		Prefix: cnRan.containerPrefix,
		CnRan:  coreRenderNet{Network: cnRan.Network, Subnet: cnRan.Subnet.String(), Bridge: cnRan.BridgeName},
		Sbi:    coreRenderNet{Network: sbi.Network, Subnet: sbi.Subnet.String(), Bridge: sbi.BridgeName},
		cnRan:  cnRan,
	}, nil
}

// labCoreRender renders the lab free5GC.
func labCoreRender() *coreRender {
	r, _ := newCoreRender(labCnRanPool(), labSbiPool())
	return r
}

func (r *coreRender) funcs() template.FuncMap {
	return template.FuncMap{
		"addr": func(role string) (string, error) {
			a, ok := r.cnRan.ReservedAddr(role)
			if !ok {
				return "", fmt.Errorf("no address reserved for %s on %s", role, r.cnRan.ID)
			}
			return a.String(), nil
		},
	}
}
