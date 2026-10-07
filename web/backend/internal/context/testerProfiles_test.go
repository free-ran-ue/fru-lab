package context

import (
	"errors"
	"path/filepath"
	"testing"
)

func newProfileDb(t *testing.T) *dbContext {
	t.Helper()
	db, err := newBboltDb(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Release() })
	return &dbContext{db: db}
}

func names(t *testing.T, d *dbContext) []string {
	t.Helper()
	list, err := d.ListTesterProfiles()
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, p := range list {
		out = append(out, ProfileName(p.Profile))
	}
	return out
}

func TestProfilesCreateUpdateDelete(t *testing.T) {
	d := newProfileDb(t)
	b, err := d.CreateTesterProfile([]byte(`{"name":"b"}`))
	if err != nil {
		t.Fatal(err)
	}
	a, err := d.CreateTesterProfile([]byte(`{"name":" a "}`))
	if err != nil {
		t.Fatal(err)
	}
	if a.ID == b.ID {
		t.Fatal("two profiles got one ID")
	}
	if got := names(t, d); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("list = %q, want [a b] by name", got)
	}

	if _, err := d.UpdateTesterProfile(b.ID, []byte(`{"name":"c","x":1}`)); err != nil {
		t.Fatal(err)
	}
	got, err := d.GetTesterProfile(b.ID)
	if err != nil || string(got.Profile) != `{"name":"c","x":1}` {
		t.Fatalf("get = %+v, %v; want the update", got, err)
	}
	// keeping its own name is not a clash
	if _, err := d.UpdateTesterProfile(b.ID, []byte(`{"name":"C"}`)); err != nil {
		t.Fatal(err)
	}

	if err := d.DeleteTesterProfile(a.ID); err != nil {
		t.Fatal(err)
	}
	if got := names(t, d); len(got) != 1 || got[0] != "C" {
		t.Fatalf("after delete: %q", got)
	}
	for name, err := range map[string]error{
		"get":    func() error { _, err := d.GetTesterProfile(a.ID); return err }(),
		"update": func() error { _, err := d.UpdateTesterProfile(a.ID, []byte(`{"name":"z"}`)); return err }(),
		"delete": d.DeleteTesterProfile(a.ID),
	} {
		if !errors.Is(err, ErrProfileNotFound) {
			t.Errorf("%s of a deleted profile: %v, want not found", name, err)
		}
	}
}

func TestProfileNamesAreUniqueAndRequired(t *testing.T) {
	d := newProfileDb(t)
	a, err := d.CreateTesterProfile([]byte(`{"name":"Basic"}`))
	if err != nil {
		t.Fatal(err)
	}
	other, err := d.CreateTesterProfile([]byte(`{"name":"other"}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.CreateTesterProfile([]byte(`{"name":" basic "}`)); !errors.Is(err, ErrProfileNameTaken) {
		t.Errorf("create with a taken name (other case, spaces): %v", err)
	}
	if _, err := d.UpdateTesterProfile(other.ID, []byte(`{"name":"BASIC"}`)); !errors.Is(err, ErrProfileNameTaken) {
		t.Errorf("rename onto a taken name: %v", err)
	}
	for _, body := range []string{`{}`, `{"name":"  "}`} {
		if _, err := d.CreateTesterProfile([]byte(body)); !errors.Is(err, ErrProfileNameMissing) {
			t.Errorf("create %s: %v, want name missing", body, err)
		}
		if _, err := d.UpdateTesterProfile(a.ID, []byte(body)); !errors.Is(err, ErrProfileNameMissing) {
			t.Errorf("update %s: %v, want name missing", body, err)
		}
	}
}

// The one profile older versions kept moves into the list, renamed if
// its name is already taken, and only once.
func TestLegacyProfileMovesIntoTheList(t *testing.T) {
	d := newProfileDb(t)
	if _, err := d.CreateTesterProfile([]byte(`{"name":"basic"}`)); err != nil {
		t.Fatal(err)
	}
	// written after the first profile, as an upgrade would find it
	if err := d.db.Put(legacyTesterBucket, legacyTesterProfileKey, []byte(`{"name":"basic","scale":{"ueCount":3}}`)); err != nil {
		t.Fatal(err)
	}
	if got := names(t, d); len(got) != 2 || got[0] != "basic" || got[1] != "basic (2)" {
		t.Fatalf("list = %q, want the legacy profile as \"basic (2)\"", got)
	}
	if raw, _ := d.db.Get(legacyTesterBucket, legacyTesterProfileKey); raw != nil {
		t.Fatal("legacy profile still there")
	}
	if got := names(t, d); len(got) != 2 {
		t.Fatalf("moved twice: %q", got)
	}
}
