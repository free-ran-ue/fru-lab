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
