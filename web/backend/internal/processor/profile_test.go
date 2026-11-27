package processor

import (
	"backend/model"
	"testing"
)

func validUe() model.UeProfile {
	return model.UeProfile{
		Name: "u", Mcc: "208", Mnc: "93", Msin: "0000000001",
		PermanentKey: "8baf473f2f8fd09487cccbd7097c6862", OpValue: "8e27b6af0e692e750f32667a3b14605d",
		Amf: "8000", Sqn: "000000000023", Sst: "1", Sd: "010203",
	}
}

func validLabGnb() model.GnbProfile {
	return model.GnbProfile{Name: "g", GnbID: "000314", Mcc: "208", Mnc: "93", Tac: "000001", Sst: "1", Sd: "010203", N2Ip: "10.0.1.20"}
}

func TestNormalizeUeProfile(t *testing.T) {
	p := validUe()
	if d := normalizeUeProfile(&p); d != nil {
		t.Fatalf("valid profile rejected: %s", d.Detail)
	}
	if p.Dnn != "internet" {
		t.Errorf("dnn default %q", p.Dnn)
	}
	for name, mutate := range map[string]func(*model.UeProfile){
		"short key":  func(p *model.UeProfile) { p.PermanentKey = "abc" },
		"bad mcc":    func(p *model.UeProfile) { p.Mcc = "20" },
		"short msin": func(p *model.UeProfile) { p.Msin = "12345678" },
		"no msin":    func(p *model.UeProfile) { p.Msin = "" },
		"no name":    func(p *model.UeProfile) { p.Name = " " },
	} {
		p := validUe()
		mutate(&p)
		if normalizeUeProfile(&p) == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestNormalizeGnbProfile(t *testing.T) {
	p := validLabGnb()
	p.AmfIp, p.HostIp, p.HostInterface = "1.2.3.4", "1.2.3.5", "eth0"
	if d := normalizeGnbProfile(&p); d != nil {
		t.Fatalf("lab profile rejected: %s", d.Detail)
	}
	if p.Core != model.GnbCoreLab || p.AmfIp != "" || p.HostIp != "" || p.HostInterface != "" {
		t.Errorf("lab profile kept external fields: %+v", p)
	}
	for _, n2 := range []string{"", "10.0.1.19", "10.0.1.3", "10.0.2.20", "10.0.1.255", "nope"} {
		p := validLabGnb()
		p.N2Ip = n2
		if normalizeGnbProfile(&p) == nil {
			t.Errorf("lab profile with n2Ip %q accepted", n2)
		}
	}

	p = validLabGnb()
	p.Core = model.GnbCoreExternal
	if normalizeGnbProfile(&p) == nil {
		t.Error("external profile without IPs accepted")
	}
	p.AmfIp, p.UpfIp, p.HostIp = "10.0.0.1", "10.0.0.2", "10.0.0.3"
	if normalizeGnbProfile(&p) == nil {
		t.Error("external profile without hostInterface accepted")
	}
	p.HostInterface = "eth0"
	if d := normalizeGnbProfile(&p); d != nil || p.AmfPort != 38412 || p.N2Ip != "" {
		t.Fatalf("external profile: %v %+v", d, p)
	}
	p.HostIp = "fe80::1"
	if normalizeGnbProfile(&p) == nil {
		t.Error("external profile with an IPv6 hostIp accepted")
	}
}
