package context

import (
	"backend/model"
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compose-spec/compose-go/v2/types"
)

// A lab-core gNB listens on its profile's N2 IP and the picked ran-ue IP.
func TestRenderGnbConfigLab(t *testing.T) {
	p := DefaultGnbProfile()
	p.N2Ip = "10.0.1.21"
	got, err := RenderGnbConfig(&p, "10.0.2.201")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`amfN2Ip: "10.0.1.3"`, `upfN3Ip: "10.0.1.5"`, `ranN2Ip: "10.0.1.21"`, `ranN3Ip: "10.0.1.21"`, `ranControlPlaneIp: "10.0.2.201"`, `ip: "10.0.1.21"`, `gnbId: "000314"`, `sd: "010203"`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
}

// An external core dials the given AMF / UPF and listens on the host IP.
func TestRenderGnbConfigExternal(t *testing.T) {
	p := DefaultGnbProfile()
	p.Core, p.AmfIp, p.AmfPort, p.UpfIp, p.UpfPort, p.HostIp = model.GnbCoreExternal, "192.168.1.10", 38412, "192.168.1.11", 2152, "192.168.1.50"
	got, err := RenderGnbConfig(&p, "ignored")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`amfN2Ip: "192.168.1.10"`, `upfN3Ip: "192.168.1.11"`, `ranN2Ip: "192.168.1.50"`, `ranControlPlaneIp: "192.168.1.50"`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
}

func TestPickFreeAddr(t *testing.T) {
	first, last := netip.MustParseAddr("10.0.2.200"), netip.MustParseAddr("10.0.2.202")
	used := map[netip.Addr]bool{}
	pick := func() string {
		a, ok := PickFreeAddr(first, last, func(a netip.Addr) bool { return used[a] })
		if !ok {
			return "none"
		}
		return a.String()
	}
	if got := pick(); got != "10.0.2.200" {
		t.Fatalf("empty network: %s", got)
	}
	used[netip.MustParseAddr("10.0.2.200")] = true
	used[netip.MustParseAddr("10.0.2.3")] = true // a container outside the range
	if got := pick(); got != "10.0.2.201" {
		t.Fatalf("got %s; want the lowest free one", got)
	}
	used[netip.MustParseAddr("10.0.2.202")] = true
	if got := pick(); got != "10.0.2.201" {
		t.Fatalf("got %s; want the hole", got)
	}
	used[netip.MustParseAddr("10.0.2.201")] = true
	if got := pick(); got != "none" {
		t.Fatalf("full range: %s", got)
	}
}

func TestIsGnbInstance(t *testing.T) {
	for target, want := range map[string]bool{
		"gnb-3f2a9c1e5b7d": true,
		"gnb-slice1":       false,
		"gnb":              false,
		"gnb-3F2A9C1E5B7D": false,
		"gnb-../../etc":    false,
	} {
		if IsGnbInstance(target) != want {
			t.Errorf("%s: want %v", target, want)
		}
	}
}

// A lab-core profile gNB is materialized into its own dir with its own
// container name and fixed addresses on both lab networks.
func TestMaterializeLabGnbInstance(t *testing.T) {
	c := newTestCompose(t, &fakeCompose{})
	p := DefaultGnbProfile()
	p.ID, p.N2Ip = "aaaaaaaaaaaa", "10.0.1.30"
	target := GnbInstanceTarget(p.ID)
	d := &model.GnbDeployment{ProfileID: p.ID, ProfileName: p.Name, Core: p.Core, GnbID: p.GnbID, Sd: p.Sd, N2Ip: p.N2Ip, RanIp: "10.0.2.200"}
	if err := c.materializeGnbInstance(target, &p, d, CnRanNetwork); err != nil {
		t.Fatal(err)
	}
	project, err := c.loadGnbInstanceProject(context.Background(), target)
	if err != nil {
		t.Fatal(err)
	}
	if project.Name != "frulab-gnb-aaaaaaaaaaaa" {
		t.Fatalf("project %s", project.Name)
	}
	svc := project.Services["gnb"]
	if svc.ContainerName != target {
		t.Fatalf("container %s", svc.ContainerName)
	}
	if n := svc.Networks["cn-ran"]; n == nil || n.Ipv4Address != "10.0.1.30" {
		t.Fatalf("cn-ran %+v", svc.Networks["cn-ran"])
	}
	if n := svc.Networks["ran-ue"]; n == nil || n.Ipv4Address != "10.0.2.200" {
		t.Fatalf("ran-ue %+v", svc.Networks["ran-ue"])
	}
	if project.Networks["cn-ran"].Name != CnRanNetwork || project.Networks["ran-ue"].Name != RanUeNetwork {
		t.Fatalf("networks %+v", project.Networks)
	}
	cfg, _ := os.ReadFile(filepath.Join(c.workDir, target, "config", "gnbcfg.yaml"))
	if !strings.Contains(string(cfg), `ranControlPlaneIp: "10.0.2.200"`) {
		t.Fatalf("gnbcfg.yaml\n%s", cfg)
	}
	if got := c.GnbInstanceDeployment(p.ID); got == nil || got.RanIp != "10.0.2.200" {
		t.Fatalf("deployment %+v", got)
	}
}

// An external-core profile gNB runs on the host network; it is listed
// while deployed and gone (working dir included) once stopped.
func TestExternalGnbInstanceLifecycle(t *testing.T) {
	fake := &fakeCompose{}
	c := newTestCompose(t, fake)
	p := DefaultGnbProfile()
	p.ID, p.Core, p.N2Ip = "bbbbbbbbbbbb", model.GnbCoreExternal, ""
	p.AmfIp, p.AmfPort, p.UpfIp, p.UpfPort, p.HostInterface, p.HostIp = "192.168.1.10", 38412, "192.168.1.11", 2152, "eth0", "192.168.1.50"

	d, err := c.UpGnbInstance(context.Background(), &p, nil)
	if err != nil {
		t.Fatal(err)
	}
	if d.RanIp != "192.168.1.50" || d.N2Ip != "192.168.1.50" || d.HostInterface != "eth0" {
		t.Fatalf("deployment %+v", d)
	}
	if fake.upped.Services["gnb"].NetworkMode != "host" {
		t.Fatalf("network mode %q", fake.upped.Services["gnb"].NetworkMode)
	}
	if ids, _ := c.ListGnbInstances(); len(ids) != 1 || ids[0] != p.ID {
		t.Fatalf("list %v", ids)
	}
	raw, _ := os.ReadFile(filepath.Join(c.workDir, "gnb-bbbbbbbbbbbb", gnbDeploymentFile))
	var saved model.GnbDeployment
	if json.Unmarshal(raw, &saved) != nil || saved.ProfileID != p.ID {
		t.Fatalf("recorded %s", raw)
	}

	if err := c.DownGnbInstance(context.Background(), p.ID); err != nil {
		t.Fatal(err)
	}
	if len(fake.downs) != 1 || fake.downs[0] != "frulab-gnb-bbbbbbbbbbbb" {
		t.Fatalf("downs %v", fake.downs)
	}
	if ids, _ := c.ListGnbInstances(); len(ids) != 0 {
		t.Fatalf("still listed after stop: %v", ids)
	}
	if _, err := os.Stat(filepath.Join(c.workDir, "gnb-bbbbbbbbbbbb")); !os.IsNotExist(err) {
		t.Fatalf("working dir left behind: %v", err)
	}
}

// A failed deploy leaves no working dir, so the gNB is not listed.
func TestGnbInstanceFailedUpLeavesNothing(t *testing.T) {
	fake := &fakeCompose{upFn: func(context.Context, *types.Project) error { return errors.New("image pull failed") }}
	c := newTestCompose(t, fake)
	p := DefaultGnbProfile()
	p.ID, p.Core, p.N2Ip, p.AmfIp, p.UpfIp, p.HostInterface, p.HostIp = "cccccccccccc", model.GnbCoreExternal, "", "1.1.1.1", "1.1.1.2", "eth0", "1.1.1.3"
	if _, err := c.UpGnbInstance(context.Background(), &p, nil); err == nil {
		t.Fatal("want an error")
	}
	if ids, _ := c.ListGnbInstances(); len(ids) != 0 {
		t.Fatalf("listed after a failed deploy: %v", ids)
	}
}

func TestProfilesRoundTrip(t *testing.T) {
	db, err := newBboltDb(filepath.Join(t.TempDir(), "p.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Release() }()
	d := &dbContext{db: db}

	p := DefaultGnbProfile()
	if err := d.SaveGnbProfile(&p); err != nil || p.ID == "" {
		t.Fatalf("save: %v, id %q", err, p.ID)
	}
	got, err := d.GetGnbProfile(p.ID)
	if err != nil || got == nil || got.GnbID != "000314" || got.N2Ip != "" {
		t.Fatalf("get: %+v %v", got, err)
	}
	u := model.UeProfile{Name: "u", Msin: "0000000001"}
	if err := d.SaveUeProfile(&u); err != nil {
		t.Fatal(err)
	}
	if list, _ := d.ListUeProfiles(); len(list) != 1 || list[0].Msin != "0000000001" {
		t.Fatalf("list: %+v", list)
	}
	if err := d.DeleteGnbProfile(p.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := d.GetGnbProfile(p.ID); got != nil {
		t.Fatal("still there after delete")
	}
}

// A UE profile saved when profiles had msinStart + count reads as one UE
// with msin = msinStart.
func TestLegacyUeProfileReadsMsinStart(t *testing.T) {
	db, err := newBboltDb(filepath.Join(t.TempDir(), "p.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Release() }()
	d := &dbContext{db: db}
	if err := db.Put(ueProfileBucket, "aaaaaaaaaaaa", []byte(`{"id":"aaaaaaaaaaaa","name":"old","mcc":"208","mnc":"93","msinStart":"0000000007","count":5}`)); err != nil {
		t.Fatal(err)
	}
	got, err := d.GetUeProfile("aaaaaaaaaaaa")
	if err != nil || got == nil || got.Msin != "0000000007" || got.Name != "old" {
		t.Fatalf("got %+v %v", got, err)
	}
	out, _ := json.Marshal(got)
	if strings.Contains(string(out), "msinStart") || strings.Contains(string(out), "count") {
		t.Fatalf("legacy fields written back: %s", out)
	}
}
