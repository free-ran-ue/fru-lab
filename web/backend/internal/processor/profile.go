package processor

import (
	flctx "backend/internal/context"
	"backend/model"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strconv"
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
		p.AmfIp, p.AmfPort, p.UpfIp, p.UpfPort, p.HostIp = "", 0, "", 0, ""
	case model.GnbCoreExternal:
		if p.AmfPort == 0 {
			p.AmfPort = 38412
		}
		if p.UpfPort == 0 {
			p.UpfPort = 2152
		}
		for name, ip := range map[string]string{"amfIp": p.AmfIp, "upfIp": p.UpfIp, "hostIp": p.HostIp} {
			if net.ParseIP(ip) == nil {
				return bad("%s must be an IP address", name)
			}
		}
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
	if p.Count == 0 {
		p.Count = 1
	}
	if p.Dnn == "" {
		p.Dnn = "internet"
	}
	switch {
	case !reMcc.MatchString(p.Mcc):
		return bad("mcc must be 3 digits")
	case !reMnc.MatchString(p.Mnc):
		return bad("mnc must be 2 or 3 digits")
	case !reMsin.MatchString(p.MsinStart):
		return bad("msinStart must be 9 or 10 digits")
	case p.Count < 1 || p.Count > 500:
		return bad("count must be 1 to 500")
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
	// the last MSIN must still fit the same number of digits
	start, _ := strconv.ParseUint(p.MsinStart, 10, 64)
	if len(strconv.FormatUint(start+uint64(p.Count)-1, 10)) > len(p.MsinStart) {
		return bad("msinStart + count overflows %d digits", len(p.MsinStart))
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
	yaml, err := flctx.RenderGnbConfig(prof)
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

// RenderUeProfile is the uecfg.yaml of the profile's first UE, dialing the
// gNB that is deployed now (or the lab gNB's address).
func (p *Processor) RenderUeProfile(prof *model.UeProfile) (*model.ResponseRenderedConfig, *model.ErrorDetail) {
	if prof.Name == "" {
		prof.Name = "preview"
	}
	if detail := normalizeUeProfile(prof); detail != nil {
		return nil, detail
	}
	yaml, err := flctx.RenderUeConfig(&flctx.UeInstanceConfig{
		Mcc: prof.Mcc, Mnc: prof.Mnc, Msin: prof.MsinStart,
		PermanentKey: prof.PermanentKey, OpValue: prof.OpValue, Amf: prof.Amf, Sqn: prof.Sqn,
		Dnn: prof.Dnn, Sst: prof.Sst, Sd: prof.Sd, RanIp: p.FlContext.GnbRanIp(),
	})
	if err != nil {
		return nil, internal(err)
	}
	return &model.ResponseRenderedConfig{Yaml: yaml}, nil
}
