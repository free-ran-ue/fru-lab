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
		Network: Network{
			N2: N2Network{Interface: "ens19", Cidr: "10.0.1.0/24", StartIP: "10.0.1.1", AmfIP: "10.0.1.1", AmfPort: 38412},
			N3: N3Network{Interface: "ens20", Cidr: "10.0.2.0/24", StartIP: "10.0.2.2", UpfIP: "10.0.2.1", UpfPort: 2152},
		},
		Rates: Rates{N2: StageRate{TimeoutMs: 5000, Retries: 1}},
	}
}

func TestExpandFillsGnbsInOrder(t *testing.T) {
	plan, err := Expand(sampleProfile(), []netip.Addr{netip.MustParseAddr("10.0.1.3")})
	require.NoError(t, err)
	require.Equal(t, 4, plan.UesPerGnb)
	require.Equal(t, 24, plan.N2Prefix)
	require.Equal(t, []GnbSpec{
		// .1 is the AMF, .3 is already on the host
		{Index: 1, Name: "gNB-1", GnbID: "000314", N2IP: "10.0.1.2", N3IP: "10.0.2.2", UeCount: 4, UeFirst: 1, UeLast: 4},
		{Index: 2, Name: "gNB-2", GnbID: "000315", N2IP: "10.0.1.4", N3IP: "10.0.2.3", UeCount: 4, UeFirst: 5, UeLast: 8},
		{Index: 3, Name: "gNB-3", GnbID: "000316", N2IP: "10.0.1.5", N3IP: "10.0.2.4", UeCount: 2, UeFirst: 9, UeLast: 10},
	}, plan.Gnbs)
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
