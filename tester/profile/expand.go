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
