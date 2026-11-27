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
	}
	switch p.Core {
	case model.GnbCoreLab:
		p.AmfIp, p.AmfPort, p.UpfIp, p.UpfPort, p.HostIp, p.HostInterface = "", 0, "", 0, "", ""
		if p.N2Ip == "" {
			return bad("n2Ip is required on the lab core: set the N2 IP (%s - %s)", flctx.GnbN2IpFirst, flctx.GnbN2IpLast)
		}
		a, err := netip.ParseAddr(p.N2Ip)
		if err != nil || !a.Is4() || a.Compare(flctx.GnbN2IpFirst) < 0 || a.Compare(flctx.GnbN2IpLast) > 0 {
			return bad("n2Ip must be an address in %s - %s (the lower ones are the lab core's)", flctx.GnbN2IpFirst, flctx.GnbN2IpLast)
		}
		p.N2Ip = a.String()
	case model.GnbCoreExternal:
		p.N2Ip = ""
		if p.AmfPort == 0 {
			p.AmfPort = 38412
		}
		if p.UpfPort == 0 {
			p.UpfPort = 2152
		}
		for _, f := range []struct{ name, ip string }{{"amfIp", p.AmfIp}, {"upfIp", p.UpfIp}} {
			if net.ParseIP(f.ip) == nil {
				return bad("%s must be an IP address", f.name)
			}
		}
		p.HostInterface = strings.TrimSpace(p.HostInterface)
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

func (p *Processor) DeleteGnbProfile(id string) *model.ErrorDetail {
	if err := p.FlContext.DeleteGnbProfile(id); err != nil {
		return internal(err)
	}
	return nil
}

func (p *Processor) RenderGnbProfile(prof *model.GnbProfile) (*model.ResponseRenderedConfig, *model.ErrorDetail) {
	if prof.Name == "" {
		prof.Name = "preview"
	}
	if detail := normalizeGnbProfile(prof); detail != nil {
		return nil, detail
	}
	ranIp := flctx.GnbRanIpPreview
	if reProfileID.MatchString(prof.ID) {
		if d := p.FlContext.GnbInstanceDeployment(prof.ID); d != nil && d.RanIp != "" {
			ranIp = d.RanIp
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
