package processor

import (
	flctx "backend/internal/context"
	"backend/model"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"regexp"
	"strings"
)

// reProfileID is a saved profile's ID (see newProfileID).
var reProfileID = regexp.MustCompile(`^[0-9a-f]{12}$`)

// isGnbTarget tells whether target materializes a gNB container: a
// profile gNB, gnb-<profile id>.
func isGnbTarget(target string) bool {
	return flctx.IsGnbInstance(target)
}

// runningGnb is a gNB that runs now and what it runs with.
type runningGnb struct {
	Target      string
	ProfileName string
	Core        string
	Sd          string
	GnbID       string
	N2Ip        string
	HostIp      string
	RanIp       string
}

// runningGnbs are the profile gNBs (by profile ID) that run now.
func (p *Processor) runningGnbs(ctx context.Context) ([]runningGnb, error) {
	var out []runningGnb
	ids, err := p.FlContext.ListGnbInstances()
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		target := flctx.GnbInstanceTarget(id)
		d := p.FlContext.GnbInstanceDeployment(id)
		if d == nil {
			continue
		}
		running, err := p.isTargetRunning(ctx, target)
		if err != nil {
			return nil, err
		}
		if running {
			out = append(out, runningGnb{
				Target: target, ProfileName: d.ProfileName, Core: d.Core, Sd: d.Sd,
				GnbID: d.GnbID, N2Ip: d.N2Ip, HostIp: d.HostIp, RanIp: d.RanIp,
			})
		}
	}
	return out, nil
}

// pickUeGnb picks the gNB a UE serving sd attaches to among the running
// gNBs: the requested one, which must run; else the first one with its SD;
// else the first one. The problem is why there is none.
func pickUeGnb(sd, requested string, gnbs []runningGnb) (runningGnb, string) {
	if requested != "" {
		for _, g := range gnbs {
			if g.Target == requested {
				return g, ""
			}
		}
		if !isGnbTarget(requested) {
			return runningGnb{}, "Unknown gNB " + requested
		}
		return runningGnb{}, "gNB " + requested + " is not running"
	}
	for _, g := range gnbs {
		if g.Sd == sd {
			return g, ""
		}
	}
	if len(gnbs) > 0 {
		return gnbs[0], ""
	}
	return runningGnb{}, "Deploy a gNB first: no gNB is running"
}

// gnbDeployConflicts are why gNB profile prof cannot run next to the
// running gNBs: a gNB ID another one uses, its N2 IP held by another
// container on cn-ran (cnRan maps address to container), or an external
// host address another external gNB listens on.
func gnbDeployConflicts(prof *model.GnbProfile, gnbs []runningGnb, cnRan map[netip.Addr]string) []string {
	self := flctx.GnbInstanceTarget(prof.ID)
	var out []string
	for _, g := range gnbs {
		if g.Target == self {
			continue
		}
		if g.GnbID != "" && strings.EqualFold(g.GnbID, prof.GnbID) {
			out = append(out, fmt.Sprintf("gNB ID %s is used by running gNB %s (%s)", prof.GnbID, g.Target, g.ProfileName))
		}
		if prof.Core == model.GnbCoreExternal && g.Core == model.GnbCoreExternal && g.HostIp == prof.HostIp {
			out = append(out, fmt.Sprintf("host address %s is used by running gNB %s (%s)", prof.HostIp, g.Target, g.ProfileName))
		}
	}
	if prof.Core != model.GnbCoreExternal {
		if a, err := netip.ParseAddr(prof.N2Ip); err == nil {
			if holder, ok := cnRan[a]; ok && holder != self {
				out = append(out, fmt.Sprintf("N2 IP %s is held by container %s", prof.N2Ip, holder))
			}
		}
	}
	return out
}

func conflict(format string, args ...any) *model.ErrorDetail {
	return &model.ErrorDetail{HttpStatus: http.StatusConflict, Detail: fmt.Sprintf(format, args...)}
}

// checkHostAddress checks that ip is an IPv4 address of host interface
// name.
func checkHostAddress(name, ip string) *model.ErrorDetail {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return conflict("Host interface %s does not exist on this host", name)
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return internal(err)
	}
	for _, a := range addrs {
		if n, ok := a.(*net.IPNet); ok && n.IP.To4() != nil && n.IP.String() == ip {
			return nil
		}
	}
	return conflict("%s is not an address of host interface %s", ip, name)
}

// DeployGnbProfileUp deploys the gNB of gNB profile profileID.
func (p *Processor) DeployGnbProfileUp(ctx context.Context, profileID string) (*model.ResponseDeployAction, *model.ErrorDetail) {
	if !reProfileID.MatchString(profileID) {
		return nil, notFound("gNB profile", profileID)
	}
	prof, err := p.FlContext.GetGnbProfile(profileID)
	if err != nil {
		return nil, internal(err)
	}
	if prof == nil {
		return nil, notFound("gNB profile", profileID)
	}
	if detail := normalizeGnbProfile(prof); detail != nil {
		detail.HttpStatus = http.StatusConflict
		detail.Detail = "gNB profile " + prof.Name + " cannot be deployed: " + detail.Detail
		return nil, detail
	}
	target := flctx.GnbInstanceTarget(profileID)

	unlock := p.FlContext.LockGnbDeploys()
	defer unlock()

	if running, err := p.isTargetRunning(ctx, target); err != nil {
		return nil, internal(err)
	} else if running {
		return nil, conflict("gNB %s (%s) is already running; stop it first", prof.Name, target)
	}

	var cnRan map[netip.Addr]string
	if prof.Core == model.GnbCoreExternal {
		if detail := checkHostAddress(prof.HostInterface, prof.HostIp); detail != nil {
			return nil, detail
		}
	} else {
		if detail := p.checkDeployPrerequisite(ctx, target); detail != nil {
			return nil, detail
		}
		if cnRan, err = p.FlContext.NetworkAddresses(ctx, flctx.CnRanNetwork); err != nil {
			return nil, internal(err)
		}
	}
	gnbs, err := p.runningGnbs(ctx)
	if err != nil {
		return nil, internal(err)
	}
	if problems := gnbDeployConflicts(prof, gnbs, cnRan); len(problems) > 0 {
		return nil, conflict("Cannot deploy gNB %s: %s", prof.Name, strings.Join(problems, "; "))
	}

	d, err := p.FlContext.UpGnbInstance(ctx, prof)
	if err != nil {
		p.ProcLog.Errorf("Failed to deploy %s with profile %q: %v", target, prof.Name, err)
		return nil, &model.ErrorDetail{
			HttpStatus: http.StatusInternalServerError,
			Detail:     "Failed to deploy gNB " + prof.Name + ", so it was taken down again: " + err.Error(),
		}
	}
	return &model.ResponseDeployAction{
		Message: fmt.Sprintf("Deploy started for gNB %s (%s, UEs dial %s)", prof.Name, target, d.RanIp),
	}, nil
}

// DeployGnbProfileDown stops the gNB of gNB profile profileID and removes
// it from the list.
func (p *Processor) DeployGnbProfileDown(ctx context.Context, profileID string) (*model.ResponseDeployAction, *model.ErrorDetail) {
	if !reProfileID.MatchString(profileID) {
		return nil, notFound("gNB", profileID)
	}
	target := flctx.GnbInstanceTarget(profileID)
	if detail := p.checkStopDependents(ctx, target); detail != nil {
		return nil, detail
	}
	if err := p.FlContext.DownGnbInstance(ctx, profileID); err != nil {
		p.ProcLog.Errorf("Failed to stop %s: %v", target, err)
		return nil, &model.ErrorDetail{HttpStatus: http.StatusInternalServerError, Detail: "Failed to stop " + target + ": " + err.Error()}
	}
	return &model.ResponseDeployAction{Message: "Stopped " + target}, nil
}

// DeployGnbList lists the deployed profile gNBs.
func (p *Processor) DeployGnbList(ctx context.Context) (*model.ResponseDeployGnbList, *model.ErrorDetail) {
	ids, err := p.FlContext.ListGnbInstances()
	if err != nil {
		return nil, internal(err)
	}
	out := &model.ResponseDeployGnbList{Instances: make([]model.GnbInstance, 0, len(ids))}
	for _, id := range ids {
		target := flctx.GnbInstanceTarget(id)
		result, err := p.FlContext.Status(ctx, target)
		if err != nil {
			p.ProcLog.Warnf("Failed to get status for %s: %v", target, err)
			continue
		}
		inst := model.GnbInstance{
			Instance:     target,
			ProfileID:    id,
			Status:       aggregateServiceStatus(result.Services),
			Services:     result.Services,
			LastDeployed: result.LastDeployed,
		}
		if d := p.FlContext.GnbInstanceDeployment(id); d != nil {
			inst.Gnb = *d
		}
		out.Instances = append(out.Instances, inst)
	}
	return out, nil
}

// DeployGnbProfileLogs is the recent log of the gNB of profile profileID.
func (p *Processor) DeployGnbProfileLogs(ctx context.Context, profileID string) (*model.ResponseDeployLogs, *model.ErrorDetail) {
	if !reProfileID.MatchString(profileID) {
		return nil, notFound("gNB", profileID)
	}
	return p.DeployLogs(ctx, flctx.GnbInstanceTarget(profileID), nil)
}

// UeInstanceID is the UE instance of a UE profile, imsi-<mcc><mnc><msin>.
func UeInstanceID(prof *model.UeProfile) string {
	return "imsi-" + prof.Mcc + prof.Mnc + prof.Msin
}

// DeployUeProfileUp deploys the UE of UE profile profileID onto gNB gnb
// (a gNB target), or one fru-lab picks when gnb is empty.
func (p *Processor) DeployUeProfileUp(ctx context.Context, profileID, gnb string) (*model.ResponseDeployAction, *model.ErrorDetail) {
	prof, err := p.FlContext.GetUeProfile(profileID)
	if err != nil {
		return nil, internal(err)
	}
	if prof == nil {
		return nil, notFound("UE profile", profileID)
	}
	if detail := normalizeUeProfile(prof); detail != nil {
		detail.HttpStatus = http.StatusConflict
		detail.Detail = "UE profile " + prof.Name + " cannot be deployed: " + detail.Detail
		return nil, detail
	}
	instance := UeInstanceID(prof)
	if running, err := p.ueRunning(ctx, instance); err != nil {
		return nil, internal(err)
	} else if running {
		return nil, conflict("UE %s (%s) is already running; stop it first", prof.Name, instance)
	}
	return p.deployUe(ctx, instance, gnb, &flctx.UeInstanceConfig{
		Mcc: prof.Mcc, Mnc: prof.Mnc, Msin: prof.Msin,
		PermanentKey: prof.PermanentKey, OpValue: prof.OpValue, Amf: prof.Amf, Sqn: prof.Sqn,
		Dnn: prof.Dnn, Sst: prof.Sst, Sd: prof.Sd,
	})
}

func (p *Processor) ueRunning(ctx context.Context, instance string) (bool, error) {
	ids, err := p.FlContext.ListUeInstances()
	if err != nil {
		return false, err
	}
	for _, id := range ids {
		if id != instance {
			continue
		}
		result, err := p.FlContext.StatusUe(ctx, instance)
		if err != nil {
			return false, err
		}
		return aggregateServiceStatus(result.Services) != "stopped", nil
	}
	return false, nil
}

// usableHostInterface tells whether an external-core gNB may listen on
// host interface name: up, not loopback, not one of Docker's.
func usableHostInterface(name string, flags net.Flags) bool {
	if flags&net.FlagUp == 0 || flags&net.FlagLoopback != 0 {
		return false
	}
	for _, prefix := range []string{"docker", "br-", "veth"} {
		if strings.HasPrefix(name, prefix) {
			return false
		}
	}
	return true
}

// HostInterfaces lists the host interfaces with an IPv4 address an
// external-core gNB can listen on.
func (p *Processor) HostInterfaces() ([]model.HostInterface, *model.ErrorDetail) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, internal(err)
	}
	out := make([]model.HostInterface, 0, len(ifaces))
	for _, iface := range ifaces {
		if !usableHostInterface(iface.Name, iface.Flags) {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		v4 := make([]string, 0, len(addrs))
		for _, a := range addrs {
			if n, ok := a.(*net.IPNet); ok && n.IP.To4() != nil {
				v4 = append(v4, n.IP.String())
			}
		}
		if len(v4) > 0 {
			out = append(out, model.HostInterface{Name: iface.Name, Addresses: v4})
		}
	}
	return out, nil
}
