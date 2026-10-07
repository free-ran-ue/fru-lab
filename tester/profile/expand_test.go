package profile

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
)

func sampleProfile() Profile {
	return Profile{
		Name:  "baseline",
		Scale: Scale{GnbCount: 3, UeCount: 10},
		Gnb: GnbTemplate{
			GnbIDStart: "000314", NamePattern: "gNB-{i}",
			Mcc: "208", Mnc: "93", Tac: "000001", Sst: 1, Sd: "010203",
		},
		Ue: UeTemplate{
			MsinStart: "0000000001", Key: "8baf473f2f8fd09487cccbd7097c6862",
			Opc: "8e27b6af0e692e750f32667a3b14605d", Amf: "8000", Sqn: "000000000023",
			Integrity: "nia2", Ciphering: "nea0", Dnn: "internet", Sst: 1, Sd: "010203",
		},
		Network: Network{
			N2: N2Network{Interface: "ens19", Cidr: "10.0.1.0/24", StartIP: "10.0.1.1", AmfIP: "10.0.1.1", AmfPort: 38412},
			N3: N3Network{Interface: "ens20", Cidr: "10.0.2.0/24", StartIP: "10.0.2.2", UpfIP: "10.0.2.1", UpfPort: 2152},
			N6: N6Network{Interface: "ens21", SinkIP: "10.0.3.2/24", UpfIP: "10.0.3.1", UePool: "10.60.0.0/16"},
		},
		Traffic: Traffic{UlMbps: 1, DlMbps: 5, PacketSize: 1400, Port: 9200},
		Rates: Rates{
			N2:             StageRate{TimeoutMs: 5000, Retries: 1},
			Registration:   ProcedureRate{RatePerSec: 50, MaxInFlight: 200, TimeoutMs: 10000, Retries: 1},
			Pdu:            ProcedureRate{RatePerSec: 50, MaxInFlight: 200, TimeoutMs: 10000, Retries: 1},
			Deregistration: ProcedureRate{RatePerSec: 50, MaxInFlight: 200, TimeoutMs: 5000, Retries: 1},
		},
	}
}

func TestExpandFillsGnbsInOrder(t *testing.T) {
	plan, err := Expand(sampleProfile(), []netip.Addr{netip.MustParseAddr("10.0.1.3")})
	require.NoError(t, err)
	require.Equal(t, 4, plan.UesPerGnb)
	require.Equal(t, 24, plan.N2Prefix)
	require.Equal(t, []GnbSpec{
		// .1 is the AMF, .3 is already on the host
		{Index: 1, Name: "gNB-1", GnbID: "000314", N2IP: "10.0.1.2", N3IP: "10.0.2.2", UeCount: 4, UeFirst: 1, UeLast: 4,
			FirstSupi: "imsi-208930000000001", LastSupi: "imsi-208930000000004"},
		{Index: 2, Name: "gNB-2", GnbID: "000315", N2IP: "10.0.1.4", N3IP: "10.0.2.3", UeCount: 4, UeFirst: 5, UeLast: 8,
			FirstSupi: "imsi-208930000000005", LastSupi: "imsi-208930000000008"},
		{Index: 3, Name: "gNB-3", GnbID: "000316", N2IP: "10.0.1.5", N3IP: "10.0.2.4", UeCount: 2, UeFirst: 9, UeLast: 10,
			FirstSupi: "imsi-208930000000009", LastSupi: "imsi-208930000000010"},
	}, plan.Gnbs)
	require.Len(t, plan.Ues, 10)
	require.Equal(t, UeSpec{Index: 10, Gnb: 2, Msin: "0000000010", Supi: "imsi-208930000000010"}, plan.Ues[9])
}

func TestExpandMoreGnbsThanUes(t *testing.T) {
	p := sampleProfile()
	p.Scale = Scale{GnbCount: 3, UeCount: 2}
	plan, err := Expand(p, nil)
	require.NoError(t, err)
	require.Equal(t, []int{1, 1, 0}, []int{plan.Gnbs[0].UeCount, plan.Gnbs[1].UeCount, plan.Gnbs[2].UeCount})
	require.Equal(t, 0, plan.Gnbs[2].UeFirst)
}

func TestExpandReportsEveryFieldError(t *testing.T) {
	p := sampleProfile()
	p.Name = " "
	p.Scale.GnbCount = 0
	p.Gnb.NamePattern = "gNB"
	p.Gnb.Tac = "1"
	p.Network.N2.AmfIP = "amf"
	p.Rates.N2.TimeoutMs = 0
	_, err := Expand(p, nil)
	var verr *ValidationError
	require.ErrorAs(t, err, &verr)
	fields := []string{}
	for _, fe := range verr.Errors {
		fields = append(fields, fe.Field)
	}
	require.ElementsMatch(t, []string{"name", "scale.gnbCount", "gnb.namePattern", "gnb.tac", "network.n2.amfIp", "rates.n2.timeoutMs"}, fields)
}

func TestExpandReportsCidrShortfallPerInterface(t *testing.T) {
	p := sampleProfile()
	p.Scale.GnbCount = 10
	p.Network.N2.Cidr = "10.0.1.0/29"
	_, err := Expand(p, nil)
	var verr *ValidationError
	require.ErrorAs(t, err, &verr)
	require.Equal(t, []FieldError{{Field: "network.n2.cidr", Message: "10.0.1.0/29 from 10.0.1.1 has only 5 usable IPs, need 10 (short by 5)"}}, verr.Errors)
}

func TestExpandReportsGnbIDOverflow(t *testing.T) {
	p := sampleProfile()
	p.Gnb.GnbIDStart = "fffffe"
	_, err := Expand(p, nil)
	var verr *ValidationError
	require.ErrorAs(t, err, &verr)
	require.Equal(t, "gnb.gnbIdStart", verr.Errors[0].Field)
}

func TestExpandSharedN2N3CidrNeverReusesAnIP(t *testing.T) {
	p := sampleProfile()
	p.Network.N3 = N3Network{Interface: "ens19", Cidr: "10.0.1.0/24", StartIP: "10.0.1.1", UpfIP: "10.0.1.6", UpfPort: 2152}
	plan, err := Expand(p, nil)
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, g := range plan.Gnbs {
		require.False(t, seen[g.N2IP], "duplicate %s", g.N2IP)
		seen[g.N2IP] = true
	}
	for _, g := range plan.Gnbs {
		require.False(t, seen[g.N3IP], "N3 IP %s reuses an N2 IP", g.N3IP)
		seen[g.N3IP] = true
	}
	for _, core := range []string{"10.0.1.1", "10.0.1.6"} { // AMF, UPF
		require.False(t, seen[core], "core IP %s was handed to a gNB", core)
	}
	// N2 takes .2-.4 (AMF is .1); N3 skips those and the UPF at .6
	require.Equal(t, []string{"10.0.1.5", "10.0.1.7", "10.0.1.8"},
		[]string{plan.Gnbs[0].N3IP, plan.Gnbs[1].N3IP, plan.Gnbs[2].N3IP})
}

func TestExpandBoundsN2Timeout(t *testing.T) {
	p := sampleProfile()
	p.Rates.N2.TimeoutMs = 60001
	_, err := Expand(p, nil)
	var verr *ValidationError
	require.ErrorAs(t, err, &verr)
	require.Equal(t, []FieldError{{Field: "rates.n2.timeoutMs", Message: "must be between 1 and 60000"}}, verr.Errors)
}

func TestExpandValidatesUeTemplateAndRates(t *testing.T) {
	p := sampleProfile()
	p.Ue.MsinStart = "000001" // 3+2+6 = 11 digits
	p.Ue.Key = "xyz"
	p.Ue.Integrity = "nia9"
	p.Ue.Dnn = ""
	p.Rates.Registration.RatePerSec = 0
	p.Rates.Pdu.TimeoutMs = 60001
	_, err := Expand(p, nil)
	var verr *ValidationError
	require.ErrorAs(t, err, &verr)
	require.ElementsMatch(t, []FieldError{
		{Field: "ue.msinStart", Message: "must be 10 digits so MCC+MNC+MSIN is 15"},
		{Field: "ue.key", Message: "must be 32 hex digits"},
		{Field: "ue.integrity", Message: "must be one of nia0, nia1, nia2, nia3"},
		{Field: "ue.dnn", Message: "must not be empty"},
		{Field: "rates.registration.ratePerSec", Message: "must be between 1 and 100000"},
		{Field: "rates.pdu.timeoutMs", Message: "must be between 1 and 60000"},
	}, verr.Errors)
}

func TestExpandReportsMsinOverflow(t *testing.T) {
	p := sampleProfile()
	p.Ue.MsinStart = "9999999995" // 10 UEs need ...04
	_, err := Expand(p, nil)
	var verr *ValidationError
	require.ErrorAs(t, err, &verr)
	require.Equal(t, "ue.msinStart", verr.Errors[0].Field)
	require.Contains(t, verr.Errors[0].Message, "overflows")
}

func TestExpandValidatesTrafficAndN6(t *testing.T) {
	p := sampleProfile()
	p.Traffic = Traffic{UlMbps: -1, DlMbps: 20000, PacketSize: 32, Port: 0}
	p.Network.N6 = N6Network{Interface: "", SinkIP: "x", UpfIP: "10.0.3.1", UePool: "10.60.0.0"}
	_, err := Expand(p, nil)
	var verr *ValidationError
	require.ErrorAs(t, err, &verr)
	require.ElementsMatch(t, []FieldError{
		{Field: "traffic.ulMbps", Message: "must be between 0 and 10000"},
		{Field: "traffic.dlMbps", Message: "must be between 0 and 10000"},
		{Field: "traffic.packetSize", Message: "must be between 64 and 9000"},
		{Field: "traffic.port", Message: "must be between 1 and 65535"},
		{Field: "network.n6.interface", Message: "must not be empty"},
		{Field: "network.n6.sinkIp", Message: `"x" is not an IPv4 address with a prefix length, e.g. 10.0.1.1/24`},
		{Field: "network.n6.uePool", Message: `"10.60.0.0" is not an IPv4 CIDR`},
	}, verr.Errors)
}

// fru-lab's core puts N2, N3 and N6 on one bridge: the sink and the UPF's
// N6 address must never be handed to a gNB.
func TestExpandNeverAllocatesTheSinkOrUpfN6IP(t *testing.T) {
	p := sampleProfile()
	p.Network.N6.SinkIP, p.Network.N6.UpfIP = "10.0.1.2/24", "10.0.1.4"
	plan, err := Expand(p, nil)
	require.NoError(t, err)
	for _, g := range plan.Gnbs {
		require.NotContains(t, []string{"10.0.1.2", "10.0.1.4"}, g.N2IP)
	}
}

func TestExpandValidatesDeregistrationAndMaxDuration(t *testing.T) {
	p := sampleProfile()
	p.Traffic.MaxDurationMin = 10080 // 7 days is allowed
	_, err := Expand(p, nil)
	require.NoError(t, err)

	for _, bad := range []int{-1, 10081} {
		p := sampleProfile()
		p.Rates.Deregistration.RatePerSec = 0
		p.Traffic.MaxDurationMin = bad
		p.Traffic.DlBatchMs = bad
		_, err := Expand(p, nil)
		var verr *ValidationError
		require.ErrorAs(t, err, &verr)
		require.ElementsMatch(t, []FieldError{
			{Field: "rates.deregistration.ratePerSec", Message: "must be between 1 and 100000"},
			{Field: "traffic.maxDurationMin", Message: "must be between 0 (no limit) and 10080 (7 days)"},
			{Field: "traffic.dlBatchMs", Message: "must be between 0 (off) and 10"},
		}, verr.Errors, "maxDurationMin %d", bad)
	}
}

// The sink is an address with its prefix length, as it goes on the
// interface; a bare IP or a network address is refused.
func TestExpandWantsTheSinkWithItsPrefix(t *testing.T) {
	for sink, msg := range map[string]string{
		"10.0.3.2":    `"10.0.3.2" is not an IPv4 address with a prefix length, e.g. 10.0.1.1/24`,
		"10.0.3.0/24": `10.0.3.0/24 is the network's own address; use a host address in it, e.g. 10.0.3.1/24`,
		"::1/64":      `"::1/64" is not an IPv4 address with a prefix length, e.g. 10.0.1.1/24`,
	} {
		p := sampleProfile()
		p.Network.N6.SinkIP = sink
		_, err := Expand(p, nil)
		var verr *ValidationError
		require.ErrorAs(t, err, &verr, sink)
		require.Equal(t, []FieldError{{Field: "network.n6.sinkIp", Message: msg}}, verr.Errors, sink)
	}
	p := sampleProfile()
	p.Network.N6.SinkIP = "172.26.6.1/16"
	_, err := Expand(p, nil)
	require.NoError(t, err)
	require.Equal(t, netip.MustParsePrefix("172.26.6.1/16"), p.Network.N6.Sink())
}

func TestPingTargetUsesTheFirstGnbIPOrTheSink(t *testing.T) {
	p := sampleProfile()
	n2, err := PingTargetFor(p, PlaneN2, []netip.Addr{netip.MustParseAddr("10.0.1.3")})
	require.NoError(t, err)
	// .1 is the AMF and .3 is on the host, as in TestExpandFillsGnbsInOrder
	require.Equal(t, PingTarget{Interface: p.Network.N2.Interface, Source: netip.MustParsePrefix("10.0.1.2/24"), Peer: netip.MustParseAddr("10.0.1.1")}, n2)

	n3, err := PingTargetFor(p, PlaneN3, nil)
	require.NoError(t, err)
	require.Equal(t, netip.MustParseAddr(p.Network.N3.UpfIP), n3.Peer)
	require.Equal(t, p.Network.N3.Interface, n3.Interface)

	n6, err := PingTargetFor(p, PlaneN6, nil)
	require.NoError(t, err)
	require.Equal(t, PingTarget{Interface: "ens21", Source: netip.MustParsePrefix("10.0.3.2/24"), Peer: netip.MustParseAddr("10.0.3.1")}, n6)

	_, err = PingTargetFor(p, "n4", nil)
	require.ErrorIs(t, err, ErrUnknownPlane)
}

// A ping test checks only its own network's fields: a broken UE template
// or another network does not stop it.
func TestPingTargetChecksOnlyItsOwnNetwork(t *testing.T) {
	p := sampleProfile()
	p.Ue.Key = "nope"
	p.Network.N6.SinkIP = "10.0.3.2"
	_, err := PingTargetFor(p, PlaneN2, nil)
	require.NoError(t, err)

	_, err = PingTargetFor(p, PlaneN6, nil)
	var verr *ValidationError
	require.ErrorAs(t, err, &verr)
	require.Equal(t, "network.n6.sinkIp", verr.Errors[0].Field)

	p.Network.N3.UpfIP = "x"
	_, err = PingTargetFor(p, PlaneN3, nil)
	require.ErrorAs(t, err, &verr)
	require.Equal(t, []FieldError{{Field: "network.n3.upfIp", Message: `"x" is not an IPv4 address`}}, verr.Errors)
}
