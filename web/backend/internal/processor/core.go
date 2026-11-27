package processor

import (
	"backend/constant"
	flctx "backend/internal/context"
	"backend/model"
	"context"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"
)

var (
	reCoreName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 ._-]{0,31}$`)
	reDnn      = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.-]{0,62}$`)
	reIfName   = regexp.MustCompile(`^[A-Za-z0-9@._:-]{1,15}$`)
)

func isIPv4(s string) bool {
	ip := net.ParseIP(s)
	return ip != nil && ip.To4() != nil
}

// normalizeCoreNetwork trims n and says what is malformed, if anything. A
// free5gc-kind core only needs a name: everything else is assigned (see
// assignFree5gcCore).
func normalizeCoreNetwork(n *model.CoreNetwork) *model.ErrorDetail {
	n.Name = strings.TrimSpace(n.Name)
	n.Kind = strings.TrimSpace(n.Kind)
	if n.Kind == "" {
		n.Kind = model.CoreKindExternal
	}
	if n.Kind != model.CoreKindExternal && n.Kind != model.CoreKindFree5gc {
		return bad("kind must be %q or %q", model.CoreKindExternal, model.CoreKindFree5gc)
	}
	if !reCoreName.MatchString(n.Name) {
		return bad("name must be 1-32 letters, digits, spaces, dots, dashes or underscores, starting with a letter or digit")
	}
	if n.IsFree5gc() {
		return nil
	}
	n.Slot, n.WebconsolePort = 0, 0
	n.N2Ip, n.N3Ip = strings.TrimSpace(n.N2Ip), strings.TrimSpace(n.N3Ip)
	n.N2Interface, n.N3Interface = strings.TrimSpace(n.N2Interface), strings.TrimSpace(n.N3Interface)
	switch {
	case !isIPv4(n.N2Ip):
		return bad("n2Ip must be an IPv4 address")
	case !isIPv4(n.N3Ip):
		return bad("n3Ip must be an IPv4 address")
	case n.N2Interface != "" && !reIfName.MatchString(n.N2Interface):
		return bad("n2Interface is not an interface name")
	case n.N3Interface != "" && !reIfName.MatchString(n.N3Interface):
		return bad("n3Interface is not an interface name")
	case len(n.Dns) == 0:
		return bad("add at least one data network")
	}
	seen := map[string]bool{}
	for i := range n.Dns {
		dn := &n.Dns[i]
		dn.Name, dn.Cidr = strings.TrimSpace(dn.Name), strings.TrimSpace(dn.Cidr)
		if !reDnn.MatchString(dn.Name) {
			return bad("data network %d: the DNN must be letters, digits, dots or dashes", i+1)
		}
		if seen[strings.ToLower(dn.Name)] {
			return bad("data network %s is listed twice", dn.Name)
		}
		seen[strings.ToLower(dn.Name)] = true
		if dn.Cidr != "" {
			if _, _, err := net.ParseCIDR(dn.Cidr); err != nil {
				return bad("data network %s: %s is not an address range like 10.60.0.0/16", dn.Name, dn.Cidr)
			}
		}
	}
	return nil
}

// assignFree5gcCore fills in what fru-lab assigns a free5gc-kind core n:
// its slot (kept from existing, the saved core n updates, else the lowest
// one no other free5gc core takes), and from the slot its AMF / UPF
// addresses, cn-ran network, data networks and webconsole port
// (basePort + slot).
func assignFree5gcCore(n *model.CoreNetwork, existing *model.CoreNetwork, others []model.CoreNetwork, basePort int) *model.ErrorDetail {
	slot := 0
	if existing != nil && existing.Slot >= 1 && existing.Slot <= flctx.MaxCoreSlot {
		slot = existing.Slot
	} else {
		taken := map[int]bool{}
		for _, o := range others {
			if o.ID != n.ID && o.IsFree5gc() {
				taken[o.Slot] = true
			}
		}
		for s := 1; s <= flctx.MaxCoreSlot; s++ {
			if !taken[s] {
				slot = s
				break
			}
		}
		if slot == 0 {
			return conflict("all %d free5GC core network slots are taken; delete one first", flctx.MaxCoreSlot)
		}
	}
	n.Slot = slot
	cnRan := flctx.CoreCnRan(n.ID, slot)
	n.N2Ip, n.N3Ip = cnRan.AmfIp, cnRan.UpfIp
	n.N2Interface = flctx.CoreCnRanNetwork(slot)
	n.N3Interface = n.N2Interface
	n.Dns = []model.CoreDn{{Name: "internet"}}
	n.WebconsolePort = basePort + slot
	return nil
}

// coreNameTaken says whether name clashes with the lab core or another
// saved core network (names are compared ignoring case).
func coreNameTaken(name, id string, others []model.CoreNetwork) string {
	if strings.EqualFold(name, model.LabCoreName) {
		return fmt.Sprintf("%s is the lab core's name", model.LabCoreName)
	}
	for _, o := range others {
		if o.ID != id && strings.EqualFold(o.Name, name) {
			return fmt.Sprintf("another core network is already named %s", o.Name)
		}
	}
	return ""
}

func (p *Processor) ListCoreNetworks() ([]model.CoreNetwork, *model.ErrorDetail) {
	list, err := p.FlContext.ListCoreNetworks()
	if err != nil {
		return nil, internal(err)
	}
	return list, nil
}

func (p *Processor) SaveCoreNetwork(id string, n *model.CoreNetwork) (*model.CoreNetwork, *model.ErrorDetail) {
	var existing *model.CoreNetwork
	if id != "" {
		var err error
		if existing, err = p.FlContext.GetCoreNetwork(id); err != nil {
			return nil, internal(err)
		}
		if existing == nil {
			return nil, notFound("core network", id)
		}
		// an update that leaves the kind out keeps it
		if strings.TrimSpace(n.Kind) == "" {
			n.Kind = existing.Kind
		}
	}
	n.ID = id
	if detail := normalizeCoreNetwork(n); detail != nil {
		return nil, detail
	}
	if existing != nil && existing.Kind != n.Kind {
		return nil, bad("the kind of core network %s cannot change from %s to %s; add a new core network instead", existing.Name, existing.Kind, n.Kind)
	}
	others, err := p.FlContext.ListCoreNetworks()
	if err != nil {
		return nil, internal(err)
	}
	if msg := coreNameTaken(n.Name, n.ID, others); msg != "" {
		return nil, &model.ErrorDetail{HttpStatus: http.StatusConflict, Detail: msg}
	}
	if n.IsFree5gc() {
		if detail := assignFree5gcCore(n, existing, others, p.FlContext.WebconsoleBasePort()); detail != nil {
			return nil, detail
		}
	}
	if err := p.FlContext.SaveCoreNetwork(n); err != nil {
		return nil, internal(err)
	}
	return n, nil
}

// DeleteCoreNetwork removes a core network; a free5gc-kind one only while
// it is not deployed, releasing its IPAM pools (its gNBs' cn-ran leases).
func (p *Processor) DeleteCoreNetwork(ctx context.Context, id string) *model.ErrorDetail {
	n, err := p.FlContext.GetCoreNetwork(id)
	if err != nil {
		return internal(err)
	}
	if n != nil && n.IsFree5gc() && reProfileID.MatchString(id) {
		target := flctx.CoreInstanceTarget(id)
		status, err := p.targetStatus(ctx, target)
		if err != nil {
			return internal(err)
		}
		if status != "stopped" {
			return conflict("core network %s is deployed; stop it first", n.Name)
		}
	}
	if err := p.FlContext.DeleteCoreNetwork(id); err != nil {
		return internal(err)
	}
	if n != nil && n.IsFree5gc() && reProfileID.MatchString(id) {
		p.FlContext.RemoveCoreInstanceDir(id)
		if err := p.FlContext.IpamReleaseCore(id); err != nil {
			p.ProcLog.Warnf("Failed to release the address pools of core network %s: %v", n.Name, err)
		}
	}
	return nil
}

// free5gcCore is free5gc-kind core network id: 404 when there is no such
// core network, 409 when it is not of kind free5gc.
func (p *Processor) free5gcCore(id string) (*model.CoreNetwork, *model.ErrorDetail) {
	if !reProfileID.MatchString(id) {
		return nil, notFound("core network", id)
	}
	n, err := p.FlContext.GetCoreNetwork(id)
	if err != nil {
		return nil, internal(err)
	}
	if n == nil {
		return nil, notFound("core network", id)
	}
	if !n.IsFree5gc() {
		return nil, conflict("core network %s is an external core; fru-lab only deploys free5gc core networks", n.Name)
	}
	return n, nil
}

// DeployCoreList is the deploy status of every free5gc-kind core network.
func (p *Processor) DeployCoreList(ctx context.Context) (*model.ResponseDeployCoreList, *model.ErrorDetail) {
	list, err := p.FlContext.ListCoreNetworks()
	if err != nil {
		return nil, internal(err)
	}
	out := &model.ResponseDeployCoreList{Cores: make([]model.CoreDeployStatus, 0, len(list))}
	for _, n := range list {
		if !n.IsFree5gc() || !reProfileID.MatchString(n.ID) {
			continue
		}
		st := model.CoreDeployStatus{CoreID: n.ID, Status: "stopped"}
		target := flctx.CoreInstanceTarget(n.ID)
		result, err := p.FlContext.Status(ctx, target)
		if err != nil {
			p.ProcLog.Warnf("Failed to get status for %s: %v", target, err)
			st.Status = "unhealthy"
		} else {
			st.Status = aggregateServiceStatus(result.Services)
			st.Services, st.LastDeployed = result.Services, result.LastDeployed
		}
		if st.Status != "stopped" {
			st.Template = p.FlContext.CoreInstanceTemplate(n.ID)
		}
		out.Cores = append(out.Cores, st)
	}
	return out, nil
}

// DeployCoreUp deploys free5gc core network id from template (empty is
// basic).
func (p *Processor) DeployCoreUp(ctx context.Context, id, template string) (*model.ResponseDeployAction, *model.ErrorDetail) {
	n, detail := p.free5gcCore(id)
	if detail != nil {
		return nil, detail
	}
	if template == "" {
		template = constant.FREE5GC_TEMPLATE_BASIC
	}
	if !knownFree5gcTemplates[template] {
		return nil, bad("Unknown free5gc template %s", template)
	}
	target := flctx.CoreInstanceTarget(id)
	if running, err := p.isTargetRunning(ctx, target); err != nil {
		return nil, internal(err)
	} else if running {
		return nil, conflict("core network %s is already running; stop it first", n.Name)
	}
	if err := p.FlContext.UpCoreInstance(ctx, id, n.Slot, n.WebconsolePort, template); err != nil {
		p.ProcLog.Errorf("Failed to deploy %s: %v", target, err)
		return nil, &model.ErrorDetail{
			HttpStatus: http.StatusInternalServerError,
			Detail:     "Failed to deploy core network " + n.Name + ", so it was taken down again: " + err.Error(),
		}
	}
	return &model.ResponseDeployAction{Message: fmt.Sprintf("Deploy started for core network %s (%s, template %s)", n.Name, target, template)}, nil
}

// DeployCoreDown stops free5gc core network id, unless a gNB on it runs.
func (p *Processor) DeployCoreDown(ctx context.Context, id string) (*model.ResponseDeployAction, *model.ErrorDetail) {
	n, detail := p.free5gcCore(id)
	if detail != nil {
		return nil, detail
	}
	target := flctx.CoreInstanceTarget(id)
	if detail := p.checkStopDependents(ctx, target); detail != nil {
		return nil, detail
	}
	if err := p.FlContext.Down(ctx, target); err != nil {
		p.ProcLog.Errorf("Failed to stop %s: %v", target, err)
		return nil, &model.ErrorDetail{HttpStatus: http.StatusInternalServerError, Detail: "Failed to stop core network " + n.Name + ": " + err.Error()}
	}
	return &model.ResponseDeployAction{Message: "Stopped core network " + n.Name}, nil
}

// DeployCoreLogs is the recent log of free5gc core network id.
func (p *Processor) DeployCoreLogs(ctx context.Context, id string, services []string) (*model.ResponseDeployLogs, *model.ErrorDetail) {
	if _, detail := p.free5gcCore(id); detail != nil {
		return nil, detail
	}
	return p.DeployLogs(ctx, flctx.CoreInstanceTarget(id), services)
}

// CoreWebconsolePort is the host port the webconsole of free5gc core
// network id is published on; 404 for any other core network.
func (p *Processor) CoreWebconsolePort(id string) (int, *model.ErrorDetail) {
	n, detail := p.free5gcCore(id)
	if detail != nil {
		if detail.HttpStatus == http.StatusConflict {
			detail.HttpStatus = http.StatusNotFound
		}
		return 0, detail
	}
	return n.WebconsolePort, nil
}
