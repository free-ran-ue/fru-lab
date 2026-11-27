package processor

import (
	flctx "backend/internal/context"
	"backend/model"
	"fmt"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
)

// Every gNB profile is one gNB and every UE profile one UE, so the fields
// that identify one must be unique among the saved profiles: gNB name,
// gNB ID, N2 IP (lab core) and host IP (external core); UE name and IMSI.
// A conflict suggests the next free value where there is one.

const maxBulk = 500

// conflictDetail is the 409 a save with conflicts fails with.
func conflictDetail(cs []model.ProfileConflict) *model.ErrorDetail {
	if len(cs) == 0 {
		return nil
	}
	msgs := make([]string, len(cs))
	for i, c := range cs {
		msgs[i] = c.Message
	}
	return &model.ErrorDetail{HttpStatus: http.StatusConflict, Detail: "Conflicts with saved profiles: " + strings.Join(msgs, "; ")}
}

// nextFreeName is base-2, base-3, ... the first one not taken.
func nextFreeName(base string, taken func(string) bool) string {
	for i := 2; ; i++ {
		if name := fmt.Sprintf("%s-%d", base, i); !taken(name) {
			return name
		}
	}
}

// nextFreeGnbID is the first 6-hex-digit gNB ID after after that is not
// taken, counting up (and wrapping around).
func nextFreeGnbID(after string, taken func(string) bool) (string, bool) {
	const size = 1 << 24
	start, err := strconv.ParseUint(after, 16, 32)
	if err != nil {
		start = 0
	}
	for i := uint64(1); i < size; i++ {
		if id := fmt.Sprintf("%06x", (start+i)%size); !taken(id) {
			return id, true
		}
	}
	return "", false
}

// nextFreeN2Ip is the first N2 IP of the lab range after after that is not
// taken (wrapping around to the start of the range).
func nextFreeN2Ip(after string, taken func(string) bool) (string, bool) {
	first, last := flctx.GnbN2IpFirst, flctx.GnbN2IpLast
	start, err := netip.ParseAddr(after)
	if err != nil || start.Compare(first) < 0 || start.Compare(last) > 0 {
		start = last
	}
	for a := start.Next(); a.Compare(last) <= 0; a = a.Next() {
		if !taken(a.String()) {
			return a.String(), true
		}
	}
	for a := first; a.Compare(start) < 0; a = a.Next() {
		if !taken(a.String()) {
			return a.String(), true
		}
	}
	return "", false
}

// nextFreeMsin is the first MSIN after after, with as many digits, that is
// not taken (wrapping around).
func nextFreeMsin(after string, taken func(string) bool) (string, bool) {
	digits := len(after)
	start, err := strconv.ParseUint(after, 10, 64)
	if err != nil || digits == 0 || digits > 18 {
		return "", false
	}
	size := uint64(1)
	for range digits {
		size *= 10
	}
	for i := uint64(1); i < size; i++ {
		if m := fmt.Sprintf("%0*d", digits, (start+i)%size); !taken(m) {
			return m, true
		}
	}
	return "", false
}

type set map[string]bool

func (s set) has(v string) bool { return s[v] }

// gnbConflicts are the fields of prof another of the saved profiles uses
// (prof itself, by ID, is skipped).
func gnbConflicts(prof *model.GnbProfile, saved []model.GnbProfile) []model.ProfileConflict {
	names, ids, n2Ips := set{}, set{}, set{}
	var byName, byID, byN2, byHost *model.GnbProfile
	for i := range saved {
		o := &saved[i]
		if prof.ID != "" && o.ID == prof.ID {
			continue
		}
		names[o.Name] = true
		ids[strings.ToLower(o.GnbID)] = true
		if o.Core != model.GnbCoreExternal && o.N2Ip != "" {
			n2Ips[o.N2Ip] = true
		}
		if byName == nil && o.Name == prof.Name {
			byName = o
		}
		if byID == nil && strings.EqualFold(o.GnbID, prof.GnbID) {
			byID = o
		}
		if byN2 == nil && prof.Core != model.GnbCoreExternal && o.Core != model.GnbCoreExternal && o.N2Ip == prof.N2Ip {
			byN2 = o
		}
		if byHost == nil && prof.Core == model.GnbCoreExternal && o.Core == model.GnbCoreExternal && o.HostIp == prof.HostIp {
			byHost = o
		}
	}

	var out []model.ProfileConflict
	if byName != nil {
		out = append(out, model.ProfileConflict{
			Field:      "name",
			Message:    fmt.Sprintf("name %s is used by another gNB profile", prof.Name),
			Suggestion: nextFreeName(prof.Name, names.has),
		})
	}
	if byID != nil {
		s, _ := nextFreeGnbID(prof.GnbID, ids.has)
		out = append(out, model.ProfileConflict{
			Field:      "gnbId",
			Message:    fmt.Sprintf("gNB ID %s is used by profile %s", prof.GnbID, byID.Name),
			Suggestion: s,
		})
	}
	if byN2 != nil {
		s, _ := nextFreeN2Ip(prof.N2Ip, n2Ips.has)
		out = append(out, model.ProfileConflict{
			Field:      "n2Ip",
			Message:    fmt.Sprintf("N2 IP %s is used by profile %s", prof.N2Ip, byN2.Name),
			Suggestion: s,
		})
	}
	if byHost != nil {
		out = append(out, model.ProfileConflict{
			Field:   "hostIp",
			Message: fmt.Sprintf("host IP %s is used by profile %s", prof.HostIp, byHost.Name),
		})
	}
	return out
}

func imsi(p *model.UeProfile) string {
	return p.Mcc + p.Mnc + p.Msin
}

// ueConflicts are the fields of prof another of the saved profiles uses
// (prof itself, by ID, is skipped).
func ueConflicts(prof *model.UeProfile, saved []model.UeProfile) []model.ProfileConflict {
	names, imsis := set{}, set{}
	var byName, byImsi *model.UeProfile
	for i := range saved {
		o := &saved[i]
		if prof.ID != "" && o.ID == prof.ID {
			continue
		}
		names[o.Name] = true
		imsis[imsi(o)] = true
		if byName == nil && o.Name == prof.Name {
			byName = o
		}
		if byImsi == nil && imsi(o) == imsi(prof) {
			byImsi = o
		}
	}

	var out []model.ProfileConflict
	if byName != nil {
		out = append(out, model.ProfileConflict{
			Field:      "name",
			Message:    fmt.Sprintf("name %s is used by another UE profile", prof.Name),
			Suggestion: nextFreeName(prof.Name, names.has),
		})
	}
	if byImsi != nil {
		s, _ := nextFreeMsin(prof.Msin, func(m string) bool { return imsis[prof.Mcc+prof.Mnc+m] })
		out = append(out, model.ProfileConflict{
			Field:      "msin",
			Message:    fmt.Sprintf("IMSI %s is used by profile %s", imsi(prof), byImsi.Name),
			Suggestion: s,
		})
	}
	return out
}

func checkBulkCount(count int) *model.ErrorDetail {
	if count < 1 || count > maxBulk {
		return bad("count must be 1 to %d", maxBulk)
	}
	return nil
}

// planGnbBulk is count copies of lab-core profile src next to the saved
// profiles: named src-2, src-3, ... with the next free gNB IDs and N2 IPs.
func planGnbBulk(src model.GnbProfile, saved []model.GnbProfile, count int) ([]model.GnbProfile, *model.ErrorDetail) {
	if detail := checkBulkCount(count); detail != nil {
		return nil, detail
	}
	if src.Core == model.GnbCoreExternal {
		return nil, bad("only lab-core gNB profiles can be copied in bulk; an external-core gNB needs its own host address")
	}
	names, ids, n2Ips := set{}, set{}, set{}
	for _, o := range saved {
		names[o.Name] = true
		ids[strings.ToLower(o.GnbID)] = true
		if o.Core != model.GnbCoreExternal && o.N2Ip != "" {
			n2Ips[o.N2Ip] = true
		}
	}
	out := make([]model.GnbProfile, 0, count)
	gnbID, n2Ip := src.GnbID, src.N2Ip
	for i := 0; i < count; i++ {
		var ok bool
		if gnbID, ok = nextFreeGnbID(gnbID, ids.has); !ok {
			return nil, bad("no free gNB ID left")
		}
		if n2Ip, ok = nextFreeN2Ip(n2Ip, n2Ips.has); !ok {
			return nil, bad("only %d free N2 IPs are left in %s - %s", i, flctx.GnbN2IpFirst, flctx.GnbN2IpLast)
		}
		cp := src
		cp.ID = ""
		cp.Name = nextFreeName(src.Name, names.has)
		cp.GnbID, cp.N2Ip = gnbID, n2Ip
		names[cp.Name], ids[gnbID], n2Ips[n2Ip] = true, true, true
		out = append(out, cp)
	}
	return out, nil
}

// planUeBulk is count copies of src next to the saved profiles: named
// src-2, src-3, ... with the next free IMSIs.
func planUeBulk(src model.UeProfile, saved []model.UeProfile, count int) ([]model.UeProfile, *model.ErrorDetail) {
	if detail := checkBulkCount(count); detail != nil {
		return nil, detail
	}
	names, imsis := set{}, set{}
	for i := range saved {
		names[saved[i].Name] = true
		imsis[imsi(&saved[i])] = true
	}
	plmn := src.Mcc + src.Mnc
	out := make([]model.UeProfile, 0, count)
	msin := src.Msin
	for i := 0; i < count; i++ {
		var ok bool
		if msin, ok = nextFreeMsin(msin, func(m string) bool { return imsis[plmn+m] }); !ok {
			return nil, bad("no free MSIN with %d digits left", len(src.Msin))
		}
		cp := src
		cp.ID = ""
		cp.Name = nextFreeName(src.Name, names.has)
		cp.Msin = msin
		names[cp.Name], imsis[plmn+msin] = true, true
		out = append(out, cp)
	}
	return out, nil
}

// CheckGnbProfile is prof's conflicts with the saved gNB profiles.
func (p *Processor) CheckGnbProfile(prof *model.GnbProfile) (*model.ResponseProfileCheck, *model.ErrorDetail) {
	if detail := normalizeGnbProfile(prof); detail != nil {
		return nil, detail
	}
	saved, err := p.FlContext.ListGnbProfiles()
	if err != nil {
		return nil, internal(err)
	}
	cs := gnbConflicts(prof, saved)
	if cs == nil {
		cs = []model.ProfileConflict{}
	}
	return &model.ResponseProfileCheck{Conflicts: cs}, nil
}

// CheckUeProfile is prof's conflicts with the saved UE profiles.
func (p *Processor) CheckUeProfile(prof *model.UeProfile) (*model.ResponseProfileCheck, *model.ErrorDetail) {
	if detail := normalizeUeProfile(prof); detail != nil {
		return nil, detail
	}
	saved, err := p.FlContext.ListUeProfiles()
	if err != nil {
		return nil, internal(err)
	}
	cs := ueConflicts(prof, saved)
	if cs == nil {
		cs = []model.ProfileConflict{}
	}
	return &model.ResponseProfileCheck{Conflicts: cs}, nil
}

// BulkGnbProfiles saves req.Count copies of a gNB profile.
func (p *Processor) BulkGnbProfiles(req *model.RequestProfileBulk) ([]model.GnbProfile, *model.ErrorDetail) {
	src, err := p.FlContext.GetGnbProfile(req.ProfileID)
	if err != nil {
		return nil, internal(err)
	}
	if src == nil {
		return nil, notFound("gNB profile", req.ProfileID)
	}
	if detail := normalizeGnbProfile(src); detail != nil {
		return nil, detail
	}
	saved, err := p.FlContext.ListGnbProfiles()
	if err != nil {
		return nil, internal(err)
	}
	plan, detail := planGnbBulk(*src, saved, req.Count)
	if detail != nil {
		return nil, detail
	}
	for i := range plan {
		if err := p.FlContext.SaveGnbProfile(&plan[i]); err != nil {
			return nil, internal(err)
		}
	}
	return plan, nil
}

// BulkUeProfiles saves req.Count copies of a UE profile.
func (p *Processor) BulkUeProfiles(req *model.RequestProfileBulk) ([]model.UeProfile, *model.ErrorDetail) {
	src, err := p.FlContext.GetUeProfile(req.ProfileID)
	if err != nil {
		return nil, internal(err)
	}
	if src == nil {
		return nil, notFound("UE profile", req.ProfileID)
	}
	if detail := normalizeUeProfile(src); detail != nil {
		return nil, detail
	}
	saved, err := p.FlContext.ListUeProfiles()
	if err != nil {
		return nil, internal(err)
	}
	plan, detail := planUeBulk(*src, saved, req.Count)
	if detail != nil {
		return nil, detail
	}
	for i := range plan {
		if err := p.FlContext.SaveUeProfile(&plan[i]); err != nil {
			return nil, internal(err)
		}
	}
	return plan, nil
}
