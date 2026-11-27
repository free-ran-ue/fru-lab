package processor

import (
	flctx "backend/internal/context"
	"backend/model"
	"context"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/netip"
	"regexp"
	"strings"
)

var (
	reDigits = func(min, max int) *regexp.Regexp {
		return regexp.MustCompile(fmt.Sprintf(`^[0-9]{%d,%d}$`, min, max))
	}
	reMcc     = reDigits(3, 3)
	reMnc     = reDigits(2, 3)
	reMsin    = reDigits(9, 10)
	reSst     = regexp.MustCompile(`^[0-9]{1,3}$`)
	reHex     = func(n int) *regexp.Regexp { return regexp.MustCompile(fmt.Sprintf(`^[0-9a-fA-F]{%d}$`, n)) }
	reSd      = reHex(6)
	reGnbID   = reHex(6)
	reTac     = reHex(6)
	reKey     = reHex(32)
	reAmf     = reHex(4)
	reSqn     = reHex(12)
	logLevels = map[string]bool{"error": true, "warn": true, "info": true, "debug": true, "trace": true, "test": true}
)

func bad(format string, args ...any) *model.ErrorDetail {
	return &model.ErrorDetail{HttpStatus: http.StatusBadRequest, Detail: fmt.Sprintf(format, args...)}
}

func notFound(what, id string) *model.ErrorDetail {
	return &model.ErrorDetail{HttpStatus: http.StatusNotFound, Detail: what + " " + id + " not found"}
}

func internal(err error) *model.ErrorDetail {
	return &model.ErrorDetail{HttpStatus: http.StatusInternalServerError, Detail: err.Error()}
}

// normalizeGnbProfile fills defaults and checks every field.
//
// Stored forms: core lab is the lab free5GC (coreNetwork empty) or a
// free5gc-kind core network (coreNetwork its id) - the N2/N3 address is
// assigned on deploy, so n2Ip and every external field are cleared. Core
// external is either given by address (amfIp / upfIp / hostInterface /
// hostIp) or an external-kind core network (coreNetwork its id; amfIp /
// upfIp come from the core on deploy and are cleared, hostInterface is
// the core's N2 interface, hostIp is typed in and may still be empty).
// Which kind coreNetwork is, is checked against the saved core networks
// by applyProfileCoreNetwork, not here.
func normalizeGnbProfile(p *model.GnbProfile) *model.ErrorDetail {
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return bad("name is required")
	}
	if p.GnbName == "" {
		p.GnbName = "gNB"
	}
	if p.LogLevel == "" {
		p.LogLevel = "info"
	}
	if p.Core == "" {
		p.Core = model.GnbCoreLab
	}
	p.GnbID = strings.ToLower(p.GnbID)
	switch {
	case !reGnbID.MatchString(p.GnbID):
		return bad("gnbId must be 6 hex digits")
	case !reMcc.MatchString(p.Mcc):
		return bad("mcc must be 3 digits")
	case !reMnc.MatchString(p.Mnc):
		return bad("mnc must be 2 or 3 digits")
	case !reTac.MatchString(p.Tac):
		return bad("tac must be 6 hex digits")
	case !reSst.MatchString(p.Sst):
		return bad("sst must be a number")
	case p.Sd != "" && !reSd.MatchString(p.Sd):
		return bad("sd must be 6 hex digits")
	case !logLevels[p.LogLevel]:
		return bad("unknown log level %s", p.LogLevel)
	case p.RadioRange < 0 || p.RadioRange > 1e6 || math.IsNaN(p.RadioRange):
		return bad("radioRange must be 0 (unlimited) to 1000000 metres")
	}
	switch p.Core {
	case model.GnbCoreLab:
		// the N2/N3 address is assigned on deploy; an old profile's n2Ip
		// is dropped
		p.AmfIp, p.AmfPort, p.UpfIp, p.UpfPort, p.HostIp, p.HostInterface, p.N2Ip = "", 0, "", 0, "", "", ""
	case model.GnbCoreExternal:
		p.N2Ip = ""
		if p.AmfPort == 0 {
			p.AmfPort = 38412
		}
		if p.UpfPort == 0 {
			p.UpfPort = 2152
		}
		p.HostInterface = strings.TrimSpace(p.HostInterface)
		p.HostIp = strings.TrimSpace(p.HostIp)
		// on a saved core network the AMF / UPF come from that core on
		// deploy and the host interface is the core's N2 one; the host IP
		// is typed in, and checked against that interface on deploy
		if p.CoreNetwork != "" {
			p.AmfIp, p.UpfIp = "", ""
			if p.HostIp != "" {
				a, err := netip.ParseAddr(p.HostIp)
				if err != nil || !a.Is4() {
					return bad("hostIp must be an IPv4 address")
				}
				p.HostIp = a.String()
			}
			break
		}
		for _, f := range []struct{ name, ip string }{{"amfIp", p.AmfIp}, {"upfIp", p.UpfIp}} {
			if net.ParseIP(f.ip) == nil {
				return bad("%s must be an IP address", f.name)
			}
		}
		if p.HostInterface == "" {
			return bad("hostInterface is required on an external core")
		}
		a, err := netip.ParseAddr(p.HostIp)
		if err != nil || !a.Is4() {
			return bad("hostIp must be an IPv4 address of %s", p.HostInterface)
		}
		p.HostIp = a.String()
	default:
		return bad("core must be %q or %q", model.GnbCoreLab, model.GnbCoreExternal)
	}
	return nil
}

// normalizeUeProfile fills defaults and checks every field.
func normalizeUeProfile(p *model.UeProfile) *model.ErrorDetail {
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return bad("name is required")
	}
	if p.Dnn == "" {
		p.Dnn = "internet"
	}
	switch {
	case !reMcc.MatchString(p.Mcc):
		return bad("mcc must be 3 digits")
	case !reMnc.MatchString(p.Mnc):
		return bad("mnc must be 2 or 3 digits")
	case !reMsin.MatchString(p.Msin):
		return bad("msin must be 9 or 10 digits")
	case !reKey.MatchString(p.PermanentKey):
		return bad("permanentKey must be 32 hex digits")
	case !reKey.MatchString(p.OpValue):
		return bad("opValue (OPc) must be 32 hex digits")
	case !reAmf.MatchString(p.Amf):
		return bad("amf must be 4 hex digits")
	case !reSqn.MatchString(p.Sqn):
		return bad("sqn must be 12 hex digits")
	case !reSst.MatchString(p.Sst):
		return bad("sst must be a number")
	case p.Sd != "" && !reSd.MatchString(p.Sd):
		return bad("sd must be 6 hex digits")
	}
	return nil
}

func (p *Processor) ListGnbProfiles() ([]model.GnbProfile, *model.ErrorDetail) {
	list, err := p.FlContext.ListGnbProfiles()
	if err != nil {
		return nil, internal(err)
	}
	return list, nil
}

func (p *Processor) SaveGnbProfile(id string, prof *model.GnbProfile) (*model.GnbProfile, *model.ErrorDetail) {
	if id != "" {
		existing, err := p.FlContext.GetGnbProfile(id)
		if err != nil {
			return nil, internal(err)
		}
		if existing == nil {
			return nil, notFound("gNB profile", id)
		}
	}
	prof.ID = id
	if detail := p.applyProfileCoreNetwork(prof); detail != nil {
		return nil, detail
	}
	if detail := normalizeGnbProfile(prof); detail != nil {
		return nil, detail
	}
	others, err := p.FlContext.ListGnbProfiles()
	if err != nil {
		return nil, internal(err)
	}
	if detail := conflictDetail(gnbConflicts(prof, others)); detail != nil {
		return nil, detail
	}
	if err := p.FlContext.SaveGnbProfile(prof); err != nil {
		return nil, internal(err)
	}
	return prof, nil
}

// DeleteGnbProfile removes a gNB profile and releases its gNB's IPAM
// leases (which it kept while stopped, to get the same addresses back).
func (p *Processor) DeleteGnbProfile(id string) *model.ErrorDetail {
	if err := p.FlContext.DeleteGnbProfile(id); err != nil {
		return internal(err)
	}
	if reProfileID.MatchString(id) {
		if err := p.FlContext.IpamReleaseOwner(flctx.GnbInstanceTarget(id)); err != nil {
			p.ProcLog.Warnf("Failed to release the addresses of gNB profile %s: %v", id, err)
		}
	}
	return nil
}

// applyProfileCoreNetwork makes prof's core follow its saved core network:
// a free5gc-kind one keeps the gNB lab-type (on that core's cn-ran
// network), an external-kind one makes it external on the core's N2
// interface. See normalizeGnbProfile for the stored forms.
func (p *Processor) applyProfileCoreNetwork(prof *model.GnbProfile) *model.ErrorDetail {
	prof.CoreNetwork = strings.TrimSpace(prof.CoreNetwork)
	if prof.CoreNetwork == "" {
		return nil
	}
	n, err := p.FlContext.GetCoreNetwork(prof.CoreNetwork)
	if err != nil {
		return internal(err)
	}
	if n == nil {
		return bad("core network %s does not exist", prof.CoreNetwork)
	}
	profileOnCoreNetwork(prof, n)
	return nil
}

// profileOnCoreNetwork sets prof's core for core network n.
func profileOnCoreNetwork(prof *model.GnbProfile, n *model.CoreNetwork) {
	prof.CoreNetwork = n.ID
	if n.IsFree5gc() {
		prof.Core = model.GnbCoreLab
		return
	}
	if prof.Core != model.GnbCoreExternal {
		// coming from lab: the ports are the defaults
		prof.AmfPort, prof.UpfPort = 0, 0
	}
	prof.Core = model.GnbCoreExternal
	prof.HostInterface = n.N2Interface
}

func (p *Processor) RenderGnbProfile(prof *model.GnbProfile) (*model.ResponseRenderedConfig, *model.ErrorDetail) {
	if prof.Name == "" {
		prof.Name = "preview"
	}
	if detail := p.applyProfileCoreNetwork(prof); detail != nil {
		return nil, detail
	}
	if detail := normalizeGnbProfile(prof); detail != nil {
		return nil, detail
	}
	// a deployed gNB shows what it runs with; a stopped one the addresses
	// it keeps leased (it gets them back on deploy)
	var deployed *model.GnbDeployment
	var n2Lease, ranLease flctx.IpamLease
	var hasN2Lease, hasRanLease bool
	if reProfileID.MatchString(prof.ID) {
		deployed = p.FlContext.GnbInstanceDeployment(prof.ID)
		target := flctx.GnbInstanceTarget(prof.ID)
		n2Lease, hasN2Lease, _ = p.FlContext.Lookup(target, flctx.LeaseRoleN2)
		ranLease, hasRanLease, _ = p.FlContext.Lookup(target, flctx.LeaseRoleRan)
	}
	ranIp := flctx.GnbRanIpPreview
	switch {
	case deployed != nil && deployed.RanIp != "":
		ranIp = deployed.RanIp
	case hasRanLease:
		ranIp = ranLease.Addr
	}
	switch {
	case prof.Core != model.GnbCoreExternal:
		on := flctx.LabCnRan()
		if prof.CoreNetwork != "" {
			n, err := p.FlContext.GetCoreNetwork(prof.CoreNetwork)
			if err != nil {
				return nil, internal(err)
			}
			if n != nil && n.IsFree5gc() {
				on = flctx.CoreCnRan(n.ID, n.Slot)
			}
		}
		prof.AmfIp, prof.UpfIp, prof.N2Ip = on.AmfIp, on.UpfIp, on.N2Preview()
		switch {
		case deployed != nil && deployed.Core != model.GnbCoreExternal && deployed.CoreNetwork == on.CoreNetwork && deployed.N2Ip != "":
			prof.N2Ip = deployed.N2Ip
		case hasN2Lease && n2Lease.Pool == on.PoolID():
			prof.N2Ip = n2Lease.Addr
		}
	case prof.CoreNetwork != "":
		if n, err := p.FlContext.GetCoreNetwork(prof.CoreNetwork); err == nil && n != nil {
			prof.AmfIp, prof.UpfIp = n.N2Ip, n.N3Ip
		}
	}
	yaml, err := flctx.RenderGnbConfig(prof, ranIp)
	if err != nil {
		return nil, internal(err)
	}
	return &model.ResponseRenderedConfig{Yaml: yaml}, nil
}

func (p *Processor) ListUeProfiles() ([]model.UeProfile, *model.ErrorDetail) {
	list, err := p.FlContext.ListUeProfiles()
	if err != nil {
		return nil, internal(err)
	}
	return list, nil
}

func (p *Processor) SaveUeProfile(id string, prof *model.UeProfile) (*model.UeProfile, *model.ErrorDetail) {
	if id != "" {
		existing, err := p.FlContext.GetUeProfile(id)
		if err != nil {
			return nil, internal(err)
		}
		if existing == nil {
			return nil, notFound("UE profile", id)
		}
	}
	prof.ID = id
	if detail := normalizeUeProfile(prof); detail != nil {
		return nil, detail
	}
	others, err := p.FlContext.ListUeProfiles()
	if err != nil {
		return nil, internal(err)
	}
	if detail := conflictDetail(ueConflicts(prof, others)); detail != nil {
		return nil, detail
	}
	if err := p.FlContext.SaveUeProfile(prof); err != nil {
		return nil, internal(err)
	}
	return prof, nil
}

func (p *Processor) DeleteUeProfile(id string) *model.ErrorDetail {
	if err := p.FlContext.DeleteUeProfile(id); err != nil {
		return internal(err)
	}
	return nil
}

// RenderUeProfile is the uecfg.yaml of the profile's UE, dialing the gNB
// a deploy would pick now (or the first address of the gNB range).
func (p *Processor) RenderUeProfile(ctx context.Context, prof *model.UeProfile) (*model.ResponseRenderedConfig, *model.ErrorDetail) {
	if prof.Name == "" {
		prof.Name = "preview"
	}
	if detail := normalizeUeProfile(prof); detail != nil {
		return nil, detail
	}
	ranIp := flctx.GnbRanIpPreview
	if gnbs, err := p.runningGnbs(ctx); err == nil {
		if g, problem := pickUeGnb(prof.Sd, "", gnbs); problem == "" {
			ranIp = g.RanIp
		}
	}
	yaml, err := flctx.RenderUeConfig(&flctx.UeInstanceConfig{
		Mcc: prof.Mcc, Mnc: prof.Mnc, Msin: prof.Msin,
		PermanentKey: prof.PermanentKey, OpValue: prof.OpValue, Amf: prof.Amf, Sqn: prof.Sqn,
		Dnn: prof.Dnn, Sst: prof.Sst, Sd: prof.Sd, RanIp: ranIp,
	})
	if err != nil {
		return nil, internal(err)
	}
	return &model.ResponseRenderedConfig{Yaml: yaml}, nil
}
