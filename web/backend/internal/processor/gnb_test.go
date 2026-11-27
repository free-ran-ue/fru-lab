package processor

import (
	"backend/model"
	"net"
	"net/netip"
	"strings"
	"testing"
)

var testGnbs = []runningGnb{
	{Target: "gnb-aaaaaaaaaaaa", ProfileName: "A", Core: model.GnbCoreLab, Sd: "112233", GnbID: "000001", N2Ip: "10.0.1.20", RanIp: "10.0.2.200"},
	{Target: "gnb-bbbbbbbbbbbb", ProfileName: "B", Core: model.GnbCoreLab, Sd: "445566", GnbID: "000002", N2Ip: "10.0.1.21", RanIp: "10.0.2.201"},
	{Target: "gnb-cccccccccccc", ProfileName: "C", Core: model.GnbCoreExternal, Sd: "445566", GnbID: "000003", N2Ip: "192.168.1.5", HostIp: "192.168.1.5", RanIp: "192.168.1.5"},
}

func TestPickUeGnb(t *testing.T) {
	for _, tc := range []struct {
		sd, requested string
		gnbs          []runningGnb
		want, problem string
	}{
		{"112233", "", testGnbs, "gnb-aaaaaaaaaaaa", ""},
		{"445566", "", testGnbs, "gnb-bbbbbbbbbbbb", ""},
		{"ffffff", "", testGnbs, "gnb-aaaaaaaaaaaa", ""},
		{"112233", "gnb-cccccccccccc", testGnbs, "gnb-cccccccccccc", ""},
		{"112233", "gnb-dddddddddddd", testGnbs, "", "not running"},
		{"112233", "gnb-slice1", testGnbs, "", "Unknown gNB"},
		{"112233", "../x", testGnbs, "", "Unknown gNB"},
		{"112233", "", nil, "", "Deploy a gNB first"},
	} {
		g, problem := pickUeGnb(tc.sd, tc.requested, tc.gnbs)
		if g.Target != tc.want || (tc.problem == "") != (problem == "") || !strings.Contains(problem, tc.problem) {
			t.Errorf("sd %s gnb %q: got %q %q; want %q %q", tc.sd, tc.requested, g.Target, problem, tc.want, tc.problem)
		}
	}
	if g, _ := pickUeGnb("112233", "", testGnbs); g.RanIp != "10.0.2.200" {
		t.Errorf("ranIp %s", g.RanIp)
	}
}

func TestGnbDeployConflicts(t *testing.T) {
	cnRan := map[netip.Addr]string{
		netip.MustParseAddr("10.0.1.3"):  "amf",
		netip.MustParseAddr("10.0.1.20"): "gnb-aaaaaaaaaaaa",
	}
	p := validLabGnb()
	p.ID, p.GnbID, p.N2Ip = "eeeeeeeeeeee", "000002", "10.0.1.20"
	got := gnbDeployConflicts(&p, testGnbs, cnRan)
	if len(got) != 2 || !strings.Contains(got[0], "gnb-bbbbbbbbbbbb") || !strings.Contains(got[1], "gnb-aaaaaaaaaaaa") {
		t.Fatalf("got %v", got)
	}

	// redeploying A itself: its own address and ID do not count
	p.ID, p.GnbID = "aaaaaaaaaaaa", "000001"
	if got := gnbDeployConflicts(&p, testGnbs, cnRan); len(got) != 0 {
		t.Fatalf("self: %v", got)
	}

	ext := validLabGnb()
	ext.ID, ext.Core, ext.N2Ip, ext.GnbID, ext.HostIp = "ffffffffffff", model.GnbCoreExternal, "", "00ffff", "192.168.1.5"
	if got := gnbDeployConflicts(&ext, testGnbs, nil); len(got) != 1 || !strings.Contains(got[0], "192.168.1.5") {
		t.Fatalf("external: %v", got)
	}
}

func TestIsGnbTarget(t *testing.T) {
	for target, want := range map[string]bool{
		"gnb-aaaaaaaaaaaa": true,
		"gnb-slice1":       false, "gnb-slice2": false, "gnb": false, "free5gc": false, "ue": false,
	} {
		if isGnbTarget(target) != want {
			t.Errorf("%s: want %v", target, want)
		}
	}
}

func TestUsableHostInterface(t *testing.T) {
	up := net.FlagUp | net.FlagBroadcast
	for _, tc := range []struct {
		name  string
		flags net.Flags
		want  bool
	}{
		{"eth0", up, true},
		{"enp3s0", up, true},
		{"eth1", net.FlagBroadcast, false},
		{"lo", net.FlagUp | net.FlagLoopback, false},
		{"docker0", up, false},
		{"docker-cn-ran", up, false},
		{"br-1234", up, false},
		{"veth99", up, false},
	} {
		if usableHostInterface(tc.name, tc.flags) != tc.want {
			t.Errorf("%s: want %v", tc.name, tc.want)
		}
	}
}

func TestUeInstanceID(t *testing.T) {
	p := validUe()
	if got := UeInstanceID(&p); got != "imsi-208930000000001" {
		t.Fatal(got)
	}
}
