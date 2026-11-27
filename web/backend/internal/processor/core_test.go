package processor

import (
	"backend/model"
	"net/http"
	"strings"
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

func TestNormalizeCoreNetworkKind(t *testing.T) {
	n := validCore()
	if d := normalizeCoreNetwork(&n); d != nil || n.Kind != model.CoreKindExternal {
		t.Fatalf("no kind is external: %v %q", d, n.Kind)
	}
	n = validCore()
	n.Kind = "open5gs"
	if normalizeCoreNetwork(&n) == nil {
		t.Error("an unknown kind was accepted")
	}
	f := model.CoreNetwork{Name: "second", Kind: model.CoreKindFree5gc}
	if d := normalizeCoreNetwork(&f); d != nil {
		t.Fatalf("a free5gc core needs only a name: %v", d.Detail)
	}
	f.Name = "free5GC"
	if coreNameTaken(f.Name, "", nil) == "" {
		t.Error("a free5gc core took the lab core's name")
	}
}

func TestAssignFree5gcCore(t *testing.T) {
	others := []model.CoreNetwork{
		{ID: "a", Kind: model.CoreKindFree5gc, Slot: 1},
		{ID: "b", Kind: model.CoreKindFree5gc, Slot: 3},
		{ID: "c", Kind: model.CoreKindExternal, Slot: 2}, // external slots do not count
	}
	n := model.CoreNetwork{Name: "x", Kind: model.CoreKindFree5gc, N2Ip: "1.2.3.4", Dns: []model.CoreDn{{Name: "ims"}}, WebconsolePort: 1}
	if d := assignFree5gcCore(&n, nil, others, 5000); d != nil {
		t.Fatal(d.Detail)
	}
	if n.Slot != 2 || n.N2Ip != "10.0.102.3" || n.N3Ip != "10.0.102.5" || n.N2Interface != "frulab-cn-ran-2" || n.N3Interface != "frulab-cn-ran-2" ||
		n.WebconsolePort != 5002 || len(n.Dns) != 1 || n.Dns[0].Name != "internet" {
		t.Fatalf("assigned %+v", n)
	}

	// an update keeps its slot, even when a lower one is free
	upd := model.CoreNetwork{ID: "b", Name: "b", Kind: model.CoreKindFree5gc, Slot: 7}
	if d := assignFree5gcCore(&upd, &others[1], others, 6000); d != nil || upd.Slot != 3 || upd.WebconsolePort != 6003 || upd.N2Ip != "10.0.103.3" {
		t.Fatalf("update %+v %v", upd, d)
	}

	full := make([]model.CoreNetwork, 0, 9)
	for s := 1; s <= 9; s++ {
		full = append(full, model.CoreNetwork{ID: string(rune('a' + s)), Kind: model.CoreKindFree5gc, Slot: s})
	}
	if d := assignFree5gcCore(&model.CoreNetwork{Kind: model.CoreKindFree5gc}, nil, full, 5000); d == nil || d.HttpStatus != http.StatusConflict {
		t.Fatalf("a tenth free5gc core: %v", d)
	}
}

var testCores = map[string]*model.CoreNetwork{
	"f00000000001": {ID: "f00000000001", Name: "second", Kind: model.CoreKindFree5gc, Slot: 2, N2Ip: "10.0.102.3", N3Ip: "10.0.102.5"},
	"e00000000001": {ID: "e00000000001", Name: "campus", Kind: model.CoreKindExternal, Enabled: true, N2Ip: "192.168.1.10", N3Ip: "192.168.1.11", N2Interface: "eth0"},
	"e00000000002": {ID: "e00000000002", Name: "off", Kind: model.CoreKindExternal, N2Ip: "192.168.1.10", N3Ip: "192.168.1.11", N2Interface: "eth0"},
	"e00000000003": {ID: "e00000000003", Name: "noif", Kind: model.CoreKindExternal, Enabled: true, N2Ip: "192.168.1.10", N3Ip: "192.168.1.11"},
}

func lookupTestCore(id string) (*model.CoreNetwork, error) {
	if n, ok := testCores[id]; ok {
		cp := *n
		return &cp, nil
	}
	return nil, nil
}

func TestResolveGnbCoreLab(t *testing.T) {
	// an old lab profile still carrying an n2Ip: ignored, the lab cn-ran
	prof := validLabGnb()
	pl, d := resolveGnbCore(&prof, "", lookupTestCore)
	if d != nil {
		t.Fatal(d.Detail)
	}
	if pl.cnRan == nil || pl.cnRan.Network != "frulab-cn-ran" || pl.cnRan.AmfIp != "10.0.1.3" || pl.coreTarget != "free5gc" || prof.N2Ip != "" || prof.CoreNetwork != "" {
		t.Fatalf("lab: %+v %+v", pl, prof)
	}

	// an external profile deployed on the lab core
	ext := model.GnbProfile{Name: "g", GnbID: "000314", Mcc: "208", Mnc: "93", Tac: "000001", Sst: "1", Core: model.GnbCoreExternal, AmfIp: "1.2.3.4", AmfPort: 38412, UpfIp: "1.2.3.5", UpfPort: 2152, HostInterface: "eth0", HostIp: "1.2.3.6"}
	if pl, d := resolveGnbCore(&ext, "lab", lookupTestCore); d != nil || pl.cnRan == nil || ext.Core != model.GnbCoreLab || ext.AmfIp != "" || ext.HostIp != "" {
		t.Fatalf("lab override: %v %+v", d, ext)
	}

	// an external profile given by address stays as it is
	ext2 := ext
	ext2.Core, ext2.AmfIp, ext2.UpfIp, ext2.HostInterface, ext2.HostIp = model.GnbCoreExternal, "1.2.3.4", "1.2.3.5", "eth0", "1.2.3.6"
	if pl, d := resolveGnbCore(&ext2, "", lookupTestCore); d != nil || pl.cnRan != nil || pl.coreNetwork != nil || ext2.AmfIp != "1.2.3.4" {
		t.Fatalf("raw external: %v %+v", d, pl)
	}
}

func TestResolveGnbCoreFree5gc(t *testing.T) {
	for _, tc := range []struct {
		name string
		prof func() model.GnbProfile
		core string
	}{
		{"deploy picks it", validLabGnb, "f00000000001"},
		{"profile's core network", func() model.GnbProfile {
			p := validLabGnb()
			p.CoreNetwork = "f00000000001"
			return p
		}, ""},
		{"stored as external by an old client", func() model.GnbProfile {
			p := validLabGnb()
			p.Core, p.CoreNetwork, p.HostInterface = model.GnbCoreExternal, "f00000000001", "eth0"
			return p
		}, ""},
	} {
		prof := tc.prof()
		pl, d := resolveGnbCore(&prof, tc.core, lookupTestCore)
		if d != nil {
			t.Fatalf("%s: %s", tc.name, d.Detail)
		}
		if pl.cnRan == nil || pl.cnRan.Network != "frulab-cn-ran-2" || pl.cnRan.AmfIp != "10.0.102.3" || pl.cnRan.UpfIp != "10.0.102.5" || pl.cnRan.CoreNetwork != "f00000000001" {
			t.Fatalf("%s: cn-ran %+v", tc.name, pl.cnRan)
		}
		if pl.coreTarget != "core-f00000000001" || pl.coreName != "second" {
			t.Fatalf("%s: core %s %s", tc.name, pl.coreTarget, pl.coreName)
		}
		if prof.Core != model.GnbCoreLab || prof.CoreNetwork != "f00000000001" || prof.HostInterface != "" || prof.N2Ip != "" {
			t.Fatalf("%s: profile %+v", tc.name, prof)
		}
	}
}

func TestResolveGnbCoreExternal(t *testing.T) {
	prof := validLabGnb()
	prof.Core, prof.CoreNetwork, prof.HostInterface, prof.HostIp = model.GnbCoreExternal, "e00000000001", "wlan9", "192.168.1.50"
	pl, d := resolveGnbCore(&prof, "", lookupTestCore)
	if d != nil {
		t.Fatal(d.Detail)
	}
	if pl.cnRan != nil || pl.coreNetwork == nil || prof.AmfIp != "192.168.1.10" || prof.UpfIp != "192.168.1.11" || prof.HostInterface != "eth0" || prof.HostIp != "192.168.1.50" || prof.AmfPort != 38412 {
		t.Fatalf("external core: %+v %+v", pl, prof)
	}

	// a lab profile deployed on an external core has no host IP typed in
	lab := validLabGnb()
	_, d = resolveGnbCore(&lab, "e00000000001", lookupTestCore)
	if d == nil || d.HttpStatus != http.StatusConflict || !strings.Contains(d.Detail, "type a host IP of eth0 into gNB profile g") {
		t.Fatalf("no host ip: %+v", d)
	}

	for core, status := range map[string]int{"e00000000002": http.StatusConflict, "e00000000003": http.StatusConflict, "abcabcabcabc": http.StatusNotFound} {
		p := validLabGnb()
		p.HostIp = "192.168.1.50"
		if _, d := resolveGnbCore(&p, core, lookupTestCore); d == nil || d.HttpStatus != status {
			t.Errorf("%s: %+v; want %d", core, d, status)
		}
	}
}

func TestNormalizeGnbOnSavedCore(t *testing.T) {
	p := model.GnbProfile{Name: "g", GnbID: "000314", Mcc: "208", Mnc: "93", Tac: "000001", Sst: "1", Core: model.GnbCoreExternal, CoreNetwork: "abcdefabcdef", AmfIp: "9.9.9.9"}
	if d := normalizeGnbProfile(&p); d != nil {
		t.Fatalf("a gNB on a saved core needs no AMF / UPF / host IP yet: %v", d.Detail)
	}
	if p.AmfIp != "" {
		t.Error("the AMF was kept; it comes from the core on deploy")
	}
	p.HostIp = "nope"
	if normalizeGnbProfile(&p) == nil {
		t.Error("a bad host IP was accepted")
	}
	lab := model.GnbProfile{Name: "g", GnbID: "000314", Mcc: "208", Mnc: "93", Tac: "000001", Sst: "1", Core: model.GnbCoreLab, N2Ip: "10.0.1.20", CoreNetwork: "abcdefabcdef"}
	if d := normalizeGnbProfile(&lab); d != nil || lab.CoreNetwork != "abcdefabcdef" || lab.N2Ip != "" {
		t.Errorf("a lab gNB on a free5gc core network: %v %+v", d, lab)
	}
}
