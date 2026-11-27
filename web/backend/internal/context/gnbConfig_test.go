package context

import (
	"backend/model"
	"path/filepath"
	"strings"
	"testing"
)

// The lab default renders the same addresses as the built-in template.
func TestRenderGnbConfigLabDefault(t *testing.T) {
	p := DefaultGnbProfile()
	got, err := RenderGnbConfig(&p)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`amfN2Ip: "10.0.1.3"`, `ranN2Ip: "10.0.1.2"`, `ranControlPlaneIp: "10.0.2.3"`, `gnbId: "000314"`, `sd: "010203"`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
}

// An external core dials the given AMF / UPF and listens on the host IP.
func TestRenderGnbConfigExternal(t *testing.T) {
	p := DefaultGnbProfile()
	p.Core, p.AmfIp, p.AmfPort, p.UpfIp, p.UpfPort, p.HostIp = model.GnbCoreExternal, "192.168.1.10", 38412, "192.168.1.11", 2152, "192.168.1.50"
	got, err := RenderGnbConfig(&p)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`amfN2Ip: "192.168.1.10"`, `upfN3Ip: "192.168.1.11"`, `ranN2Ip: "192.168.1.50"`, `ranControlPlaneIp: "192.168.1.50"`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
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
	if err != nil || got == nil || got.GnbID != "000314" {
		t.Fatalf("get: %+v %v", got, err)
	}
	u := model.UeProfile{Name: "u", Count: 3}
	if err := d.SaveUeProfile(&u); err != nil {
		t.Fatal(err)
	}
	if list, _ := d.ListUeProfiles(); len(list) != 1 || list[0].Count != 3 {
		t.Fatalf("list: %+v", list)
	}
	if err := d.DeleteGnbProfile(p.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := d.GetGnbProfile(p.ID); got != nil {
		t.Fatal("still there after delete")
	}
}
