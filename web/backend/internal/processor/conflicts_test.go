package processor

import (
	"backend/model"
	"net/http"
	"strings"
	"testing"
)

func byField(cs []model.ProfileConflict) map[string]model.ProfileConflict {
	out := map[string]model.ProfileConflict{}
	for _, c := range cs {
		out[c.Field] = c
	}
	return out
}

func TestGnbConflicts(t *testing.T) {
	a := validLabGnb()
	a.ID, a.Name, a.GnbID, a.N2Ip = "aaaaaaaaaaaa", "Lab gNB", "000314", "10.0.1.20"
	b := validLabGnb()
	b.ID, b.Name, b.GnbID, b.N2Ip = "bbbbbbbbbbbb", "Lab gNB-2", "000315", "10.0.1.21"
	ext := validLabGnb()
	ext.ID, ext.Name, ext.Core, ext.N2Ip, ext.GnbID, ext.HostIp = "cccccccccccc", "ext", model.GnbCoreExternal, "", "00aaaa", "192.168.1.5"
	saved := []model.GnbProfile{a, b, ext}

	// a new profile copying a: every unique field collides
	p := a
	p.ID = ""
	got := byField(gnbConflicts(&p, saved))
	if c := got["name"]; c.Suggestion != "Lab gNB-3" {
		t.Errorf("name: %+v", c)
	}
	if c := got["gnbId"]; c.Suggestion != "000316" || !strings.Contains(c.Message, "Lab gNB") {
		t.Errorf("gnbId: %+v", c)
	}
	if c := got["n2Ip"]; c.Suggestion != "10.0.1.22" || !strings.Contains(c.Message, "Lab gNB") {
		t.Errorf("n2Ip: %+v", c)
	}
	if len(got) != 3 {
		t.Errorf("conflicts %+v", got)
	}

	// a itself, saved again unchanged, conflicts with nothing
	if cs := gnbConflicts(&a, saved); len(cs) != 0 {
		t.Errorf("self conflicts: %+v", cs)
	}

	// gNB IDs compare case-insensitively
	p = validLabGnb()
	p.Name, p.GnbID, p.N2Ip = "new", "00AAAA", "10.0.1.50"
	if c := byField(gnbConflicts(&p, saved))["gnbId"]; c.Suggestion != "00aaab" {
		t.Errorf("gnbId case: %+v", c)
	}

	// an external profile's host IP collides only with external profiles,
	// and the lab N2 IP of another profile does not matter to it
	p = ext
	p.ID, p.Name, p.GnbID = "", "ext2", "00bbbb"
	got = byField(gnbConflicts(&p, saved))
	if _, ok := got["hostIp"]; !ok || len(got) != 1 {
		t.Errorf("external: %+v", got)
	}
}

func TestUeConflicts(t *testing.T) {
	a := validUe()
	a.ID, a.Name, a.Msin = "aaaaaaaaaaaa", "ue", "0000000001"
	b := validUe()
	b.ID, b.Name, b.Msin = "bbbbbbbbbbbb", "ue-2", "0000000002"
	saved := []model.UeProfile{a, b}

	p := a
	p.ID = ""
	got := byField(ueConflicts(&p, saved))
	if c := got["name"]; c.Suggestion != "ue-3" {
		t.Errorf("name: %+v", c)
	}
	if c := got["msin"]; c.Suggestion != "0000000003" || !strings.Contains(c.Message, "208930000000001") || !strings.Contains(c.Message, "ue") {
		t.Errorf("msin: %+v", c)
	}

	// the IMSI is what must be unique: the same MSIN on another PLMN is fine
	p = a
	p.ID, p.Name, p.Mnc = "", "other plmn", "94"
	if cs := ueConflicts(&p, saved); len(cs) != 0 {
		t.Errorf("other plmn: %+v", cs)
	}

	// the suggestion keeps the digit count and wraps around
	p = validUe()
	p.Name, p.Msin = "x", "999999999"
	if c := byField(ueConflicts(&p, []model.UeProfile{{ID: "z", Name: "z", Mcc: "208", Mnc: "93", Msin: "999999999"}}))["msin"]; c.Suggestion != "000000000" {
		t.Errorf("wrap: %+v", c)
	}
}

func TestConflictDetail(t *testing.T) {
	if conflictDetail(nil) != nil {
		t.Fatal("no conflicts must not fail")
	}
	d := conflictDetail([]model.ProfileConflict{{Field: "gnbId", Message: "gNB ID 000314 is used by profile A"}, {Field: "name", Message: "name x is used"}})
	if d.HttpStatus != http.StatusConflict || !strings.Contains(d.Detail, "profile A") || !strings.Contains(d.Detail, "name x") {
		t.Fatalf("%+v", d)
	}
}

func TestNextFreeN2IpWraps(t *testing.T) {
	taken := set{"10.0.1.254": true, "10.0.1.20": true}
	if got, _ := nextFreeN2Ip("10.0.1.253", taken.has); got != "10.0.1.21" {
		t.Fatalf("got %s", got)
	}
}

func TestPlanGnbBulk(t *testing.T) {
	src := validLabGnb()
	src.ID, src.Name, src.GnbID, src.N2Ip = "aaaaaaaaaaaa", "edge", "000001", "10.0.1.20"
	other := validLabGnb()
	other.ID, other.Name, other.GnbID, other.N2Ip = "bbbbbbbbbbbb", "edge-3", "000003", "10.0.1.22"
	plan, d := planGnbBulk(src, []model.GnbProfile{src, other}, 3)
	if d != nil {
		t.Fatal(d.Detail)
	}
	want := []struct{ name, id, n2 string }{
		{"edge-2", "000002", "10.0.1.21"},
		{"edge-4", "000004", "10.0.1.23"},
		{"edge-5", "000005", "10.0.1.24"},
	}
	for i, w := range want {
		g := plan[i]
		if g.Name != w.name || g.GnbID != w.id || g.N2Ip != w.n2 || g.ID != "" || g.Sd != src.Sd {
			t.Errorf("%d: %+v; want %+v", i, g, w)
		}
	}

	ext := src
	ext.Core = model.GnbCoreExternal
	if _, d := planGnbBulk(ext, nil, 2); d == nil || d.HttpStatus != http.StatusBadRequest {
		t.Error("external bulk accepted")
	}
	for _, n := range []int{0, 501} {
		if _, d := planGnbBulk(src, nil, n); d == nil {
			t.Errorf("count %d accepted", n)
		}
	}
	// the N2 range has 235 addresses; src holds one
	if _, d := planGnbBulk(src, []model.GnbProfile{src}, 235); d == nil {
		t.Error("ran past the N2 range")
	}
	if _, d := planGnbBulk(src, []model.GnbProfile{src}, 234); d != nil {
		t.Errorf("full N2 range refused: %s", d.Detail)
	}
}

func TestPlanUeBulk(t *testing.T) {
	src := validUe()
	src.ID, src.Name, src.Msin = "aaaaaaaaaaaa", "phone", "0000000001"
	other := validUe()
	other.ID, other.Name, other.Msin = "bbbbbbbbbbbb", "phone-2", "0000000002"
	plan, d := planUeBulk(src, []model.UeProfile{src, other}, 2)
	if d != nil {
		t.Fatal(d.Detail)
	}
	if plan[0].Name != "phone-3" || plan[0].Msin != "0000000003" || plan[1].Name != "phone-4" || plan[1].Msin != "0000000004" {
		t.Fatalf("plan %+v", plan)
	}
	if plan[0].PermanentKey != src.PermanentKey || plan[0].ID != "" {
		t.Fatalf("copy %+v", plan[0])
	}
}
