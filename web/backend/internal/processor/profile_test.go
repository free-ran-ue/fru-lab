package processor

import (
	"backend/model"
	"testing"
)

func validUe() model.UeProfile {
	return model.UeProfile{
		Name: "u", Mcc: "208", Mnc: "93", MsinStart: "0000000001", Count: 5,
		PermanentKey: "8baf473f2f8fd09487cccbd7097c6862", OpValue: "8e27b6af0e692e750f32667a3b14605d",
		Amf: "8000", Sqn: "000000000023", Sst: "1", Sd: "010203",
	}
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
		"short key": func(p *model.UeProfile) { p.PermanentKey = "abc" },
		"bad mcc":   func(p *model.UeProfile) { p.Mcc = "20" },
		"overflow":  func(p *model.UeProfile) { p.MsinStart = "9999999999"; p.Count = 2 },
		"too many":  func(p *model.UeProfile) { p.Count = 501 },
		"no name":   func(p *model.UeProfile) { p.Name = " " },
	} {
		p := validUe()
		mutate(&p)
		if normalizeUeProfile(&p) == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestNormalizeGnbProfile(t *testing.T) {
	p := model.GnbProfile{Name: "g", GnbID: "000314", Mcc: "208", Mnc: "93", Tac: "000001", Sst: "1", Sd: "010203", AmfIp: "1.2.3.4"}
	if d := normalizeGnbProfile(&p); d != nil {
		t.Fatalf("lab profile rejected: %s", d.Detail)
	}
	if p.Core != model.GnbCoreLab || p.AmfIp != "" {
		t.Errorf("lab profile kept external fields: %+v", p)
	}
	p.Core = model.GnbCoreExternal
	if normalizeGnbProfile(&p) == nil {
		t.Error("external profile without IPs accepted")
	}
	p.AmfIp, p.UpfIp, p.HostIp = "10.0.0.1", "10.0.0.2", "10.0.0.3"
	if d := normalizeGnbProfile(&p); d != nil || p.AmfPort != 38412 {
		t.Fatalf("external profile: %v %+v", d, p)
	}
}
