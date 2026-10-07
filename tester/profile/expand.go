package profile

import (
	"fmt"
	"net/netip"
	"regexp"
	"strings"
)

// GnbSpec is one concrete gNB a run will bring up. UeFirst/UeLast are the
// 1-based UE indexes it owns (both 0 when it owns none); FirstSupi and
// LastSupi are those UEs' SUPIs, so the user can check them against the
// subscribers in the core.
type GnbSpec struct {
	Index     int    `json:"index"`
	Name      string `json:"name"`
	GnbID     string `json:"gnbId"`
	N2IP      string `json:"n2Ip"`
	N3IP      string `json:"n3Ip"`
	UeCount   int    `json:"ueCount"`
	UeFirst   int    `json:"ueFirst"`
	UeLast    int    `json:"ueLast"`
	FirstSupi string `json:"firstSupi"`
	LastSupi  string `json:"lastSupi"`
}

// UeSpec is one concrete UE. Gnb is the 0-based index into Plan.Gnbs.
type UeSpec struct {
	Index int    // 1-based
	Gnb   int    // 0-based index into Plan.Gnbs
	Msin  string // incremented from UeTemplate.MsinStart
	Supi  string // "imsi-" + MCC + MNC + MSIN
}

// Plan is a profile expanded against a specific host. Ues is left out of
// the JSON: the setup page only needs the per-gNB SUPI ranges, and a full
// UE list would make every validate response O(UE count).
type Plan struct {
	Gnbs      []GnbSpec `json:"gnbs"`
	Ues       []UeSpec  `json:"-"`
	UesPerGnb int       `json:"uesPerGnb"`
	N2Prefix  int       `json:"n2Prefix"`
	N3Prefix  int       `json:"n3Prefix"`
}

var (
	reMcc  = regexp.MustCompile(`^[0-9]{3}$`)
	reMnc  = regexp.MustCompile(`^[0-9]{2,3}$`)
	reHex6 = regexp.MustCompile(`^[0-9a-fA-F]{6}$`)
	reHex  = func(n int) *regexp.Regexp { return regexp.MustCompile(fmt.Sprintf(`^[0-9a-fA-F]{%d}$`, n)) }
	reKey  = reHex(32)
	reAmf  = reHex(4)
	reSqn  = reHex(12)
	reNia  = regexp.MustCompile(`^nia[0-3]$`)
	reNea  = regexp.MustCompile(`^nea[0-3]$`)
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
		netip.MustParseAddr(p.Network.N6.UpfIP),
		p.Network.N6.Sink().Addr(),
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
	if _, err := IncrementDecimal(p.Ue.MsinStart, p.Scale.UeCount-1); err != nil {
		verr.add("ue.msinStart", err.Error())
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
	plan.Ues = make([]UeSpec, 0, p.Scale.UeCount)
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
		for range ues {
			msin, _ := IncrementDecimal(p.Ue.MsinStart, nextUe-1)
			plan.Ues = append(plan.Ues, UeSpec{
				Index: nextUe, Gnb: i, Msin: msin,
				Supi: "imsi-" + p.Gnb.Mcc + p.Gnb.Mnc + msin,
			})
			nextUe++
		}
		if ues > 0 {
			spec.UeFirst, spec.UeLast = nextUe-ues, nextUe-1
			spec.FirstSupi = plan.Ues[spec.UeFirst-1].Supi
			spec.LastSupi = plan.Ues[spec.UeLast-1].Supi
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
	validateUe(p, verr)
	validateTraffic(p, verr)
	validateEndpoint(verr, "network.n2", p.Network.N2.Interface, p.Network.N2.Cidr, p.Network.N2.StartIP, "amfIp", p.Network.N2.AmfIP, "amfPort", p.Network.N2.AmfPort)
	validateEndpoint(verr, "network.n3", p.Network.N3.Interface, p.Network.N3.Cidr, p.Network.N3.StartIP, "upfIp", p.Network.N3.UpfIP, "upfPort", p.Network.N3.UpfPort)
	// The upper bound keeps Stop prompt: in-flight attempts finish within
	// their timeout before teardown can start.
	if p.Rates.N2.TimeoutMs < 1 || p.Rates.N2.TimeoutMs > 60000 {
		verr.add("rates.n2.timeoutMs", "must be between 1 and 60000")
	}
	if p.Rates.N2.Retries < 0 {
		verr.add("rates.n2.retries", "must not be negative")
	}
	validateProcedureRate(verr, "rates.registration", p.Rates.Registration)
	validateProcedureRate(verr, "rates.pdu", p.Rates.Pdu)
	validateProcedureRate(verr, "rates.deregistration", p.Rates.Deregistration)
}

func validateUe(p Profile, verr *ValidationError) {
	u := p.Ue
	if u.MsinStart == "" || strings.Trim(u.MsinStart, "0123456789") != "" {
		verr.add("ue.msinStart", "must be decimal digits")
	} else if n := len(p.Gnb.Mcc) + len(p.Gnb.Mnc) + len(u.MsinStart); reMcc.MatchString(p.Gnb.Mcc) && reMnc.MatchString(p.Gnb.Mnc) && n != 15 {
		verr.add("ue.msinStart", fmt.Sprintf("must be %d digits so MCC+MNC+MSIN is 15", 15-len(p.Gnb.Mcc)-len(p.Gnb.Mnc)))
	}
	if !reKey.MatchString(u.Key) {
		verr.add("ue.key", "must be 32 hex digits")
	}
	if !reKey.MatchString(u.Opc) {
		verr.add("ue.opc", "must be 32 hex digits")
	}
	if !reAmf.MatchString(u.Amf) {
		verr.add("ue.amf", "must be 4 hex digits")
	}
	if !reSqn.MatchString(u.Sqn) {
		verr.add("ue.sqn", "must be 12 hex digits")
	}
	if !reNia.MatchString(u.Integrity) {
		verr.add("ue.integrity", "must be one of nia0, nia1, nia2, nia3")
	}
	if !reNea.MatchString(u.Ciphering) {
		verr.add("ue.ciphering", "must be one of nea0, nea1, nea2, nea3")
	}
	if strings.TrimSpace(u.Dnn) == "" {
		verr.add("ue.dnn", "must not be empty")
	}
	if u.Sst < 0 || u.Sst > 255 {
		verr.add("ue.sst", "must be between 0 and 255")
	}
	if u.Sd != "" && !reHex6.MatchString(u.Sd) {
		verr.add("ue.sd", "must be empty or 6 hex digits")
	}
}

func validateTraffic(p Profile, verr *ValidationError) {
	t, n6 := p.Traffic, p.Network.N6
	for field, v := range map[string]float64{"traffic.ulMbps": t.UlMbps, "traffic.dlMbps": t.DlMbps} {
		if v < 0 || v > 10000 {
			verr.add(field, "must be between 0 and 10000")
		}
	}
	if t.PacketSize < 64 || t.PacketSize > MaxPacketSize {
		verr.add("traffic.packetSize", fmt.Sprintf("must be between 64 and %d", MaxPacketSize))
	}
	if t.Port < 1 || t.Port > 65535 {
		verr.add("traffic.port", "must be between 1 and 65535")
	}
	if t.MaxDurationMin < 0 || t.MaxDurationMin > 7*24*60 {
		verr.add("traffic.maxDurationMin", "must be between 0 (no limit) and 10080 (7 days)")
	}
	switch t.Engine {
	case "", "auto", "socket", "afxdp":
	default:
		verr.add("traffic.engine", `must be "auto", "socket" or "afxdp"`)
	}
	if t.DlBatchMs < 0 || t.DlBatchMs > 10 {
		verr.add("traffic.dlBatchMs", "must be between 0 (off) and 10")
	}
	if strings.TrimSpace(n6.Interface) == "" {
		verr.add("network.n6.interface", "must not be empty")
	}
	validateSink(verr, n6.SinkIP)
	if a, err := netip.ParseAddr(n6.UpfIP); err != nil || !a.Is4() {
		verr.add("network.n6.upfIp", fmt.Sprintf("%q is not an IPv4 address", n6.UpfIP))
	}
	if pre, err := netip.ParsePrefix(n6.UePool); err != nil || !pre.Addr().Is4() {
		verr.add("network.n6.uePool", fmt.Sprintf("%q is not an IPv4 CIDR", n6.UePool))
	}
}

// validateSink wants the sink as it goes on the interface: a host address
// with its prefix length.
func validateSink(verr *ValidationError, v string) {
	p, err := netip.ParsePrefix(v)
	if err != nil || !p.Addr().Is4() {
		verr.add("network.n6.sinkIp", fmt.Sprintf("%q is not an IPv4 address with a prefix length, e.g. 10.0.1.1/24", v))
		return
	}
	if p.Bits() < 31 && p.Addr() == p.Masked().Addr() {
		verr.add("network.n6.sinkIp", fmt.Sprintf("%s is the network's own address; use a host address in it, e.g. %s/%d",
			p, p.Addr().Next(), p.Bits()))
	}
}

func validateProcedureRate(verr *ValidationError, base string, r ProcedureRate) {
	if r.RatePerSec < 1 || r.RatePerSec > 100000 {
		verr.add(base+".ratePerSec", "must be between 1 and 100000")
	}
	if r.MaxInFlight < 1 || r.MaxInFlight > 100000 {
		verr.add(base+".maxInFlight", "must be between 1 and 100000")
	}
	if r.TimeoutMs < 1 || r.TimeoutMs > 60000 {
		verr.add(base+".timeoutMs", "must be between 1 and 60000")
	}
	if r.Retries < 0 {
		verr.add(base+".retries", "must not be negative")
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
