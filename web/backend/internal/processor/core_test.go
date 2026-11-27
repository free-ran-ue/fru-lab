package processor

import (
	"backend/model"
	"testing"
)

func validCore() model.CoreNetwork {
	return model.CoreNetwork{Name: "lab-b", N2Ip: "192.168.1.10", N3Ip: "192.168.1.11", Dns: []model.CoreDn{{Name: "internet"}}}
}

func TestNormalizeCoreNetwork(t *testing.T) {
	ok := validCore()
	ok.Name = "  lab-b  "
	ok.Dns = []model.CoreDn{{Name: "internet", Cidr: "10.60.0.0/16"}, {Name: "ims"}}
	if d := normalizeCoreNetwork(&ok); d != nil || ok.Name != "lab-b" {
		t.Fatalf("valid core rejected or not trimmed: %v %q", d, ok.Name)
	}
	cases := map[string]func(n *model.CoreNetwork){
		"no name":      func(n *model.CoreNetwork) { n.Name = "" },
		"bad n2":       func(n *model.CoreNetwork) { n.N2Ip = "10.0.0" },
		"ipv6 n3":      func(n *model.CoreNetwork) { n.N3Ip = "::1" },
		"bad iface":    func(n *model.CoreNetwork) { n.N2Interface = "eth 0" },
		"no dn":        func(n *model.CoreNetwork) { n.Dns = nil },
		"dup dn":       func(n *model.CoreNetwork) { n.Dns = []model.CoreDn{{Name: "internet"}, {Name: "Internet"}} },
		"bad dn cidr":  func(n *model.CoreNetwork) { n.Dns = []model.CoreDn{{Name: "internet", Cidr: "10.60.0.0"}} },
		"bad dnn name": func(n *model.CoreNetwork) { n.Dns = []model.CoreDn{{Name: "-x"}} },
	}
	for name, mutate := range cases {
		n := validCore()
		mutate(&n)
		if normalizeCoreNetwork(&n) == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestCoreNameTaken(t *testing.T) {
	others := []model.CoreNetwork{{ID: "a", Name: "Core-A"}}
	if coreNameTaken("free5gc", "", others) == "" {
		t.Error("the lab core's name was free")
	}
	if coreNameTaken("core-a", "", others) == "" {
		t.Error("a taken name (other case) was free")
	}
	if coreNameTaken("core-a", "a", others) != "" {
		t.Error("a core clashed with itself")
	}
	if coreNameTaken("core-b", "", others) != "" {
		t.Error("a free name was taken")
	}
}

func TestOnCoreLab(t *testing.T) {
	prof := model.GnbProfile{Name: "g", Core: model.GnbCoreExternal, AmfIp: "1.2.3.4", AmfPort: 38412, UpfIp: "1.2.3.5", UpfPort: 2152, HostInterface: "eth0", HostIp: "1.2.3.6", N2Ip: "10.0.1.20"}
	if d := (&Processor{}).onCore(&prof, "lab"); d != nil {
		t.Fatalf("lab override failed: %v", d)
	}
	if prof.Core != model.GnbCoreLab || prof.AmfIp != "" || prof.HostIp != "" || prof.N2Ip != "10.0.1.20" {
		t.Fatalf("lab override left %+v", prof)
	}
	same := model.GnbProfile{Core: model.GnbCoreLab, N2Ip: "10.0.1.21"}
	if d := (&Processor{}).onCore(&same, ""); d != nil || same.N2Ip != "10.0.1.21" {
		t.Fatal("an empty core changed the profile")
	}
}

func TestNormalizeGnbOnSavedCore(t *testing.T) {
	p := model.GnbProfile{Name: "g", GnbID: "000314", Mcc: "208", Mnc: "93", Tac: "000001", Sst: "1", Core: model.GnbCoreExternal, CoreNetwork: "abcdefabcdef", AmfIp: "9.9.9.9"}
	if d := normalizeGnbProfile(&p); d != nil {
		t.Fatalf("a gNB on a saved core needs no AMF / UPF / host: %v", d.Detail)
	}
	if p.AmfIp != "" {
		t.Error("the AMF was kept; it comes from the core on deploy")
	}
	p.HostInterface = "eth0"
	p.HostIp = "nope"
	if normalizeGnbProfile(&p) == nil {
		t.Error("a bad host IP with a host interface was accepted")
	}
	lab := model.GnbProfile{Name: "g", GnbID: "000314", Mcc: "208", Mnc: "93", Tac: "000001", Sst: "1", Core: model.GnbCoreLab, N2Ip: "10.0.1.20", CoreNetwork: "abcdefabcdef"}
	if d := normalizeGnbProfile(&lab); d != nil || lab.CoreNetwork != "" {
		t.Errorf("a lab gNB kept a core network: %v %q", d, lab.CoreNetwork)
	}
}
