package context

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestBboltGetPut(t *testing.T) {
	db, err := newBboltDb(filepath.Join(t.TempDir(), "sub", "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Release() }()

	got, err := db.Get("tester", "profile")
	if err != nil || got != nil {
		t.Fatalf("missing bucket: got %q, %v; want nil, nil", got, err)
	}
	if err := db.Put("tester", "profile", []byte(`{"name":"a"}`)); err != nil {
		t.Fatal(err)
	}
	if err := db.Put("tester", "profile", []byte(`{"name":"b"}`)); err != nil {
		t.Fatal(err)
	}
	got, err = db.Get("tester", "profile")
	if err != nil || string(got) != `{"name":"b"}` {
		t.Fatalf("got %q, %v; want the second write", got, err)
	}
	got, err = db.Get("tester", "other")
	if err != nil || got != nil {
		t.Fatalf("missing key: got %q, %v; want nil, nil", got, err)
	}
}

func TestBboltListAndDelete(t *testing.T) {
	db, err := newBboltDb(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Release() }()

	if kvs, err := db.List("none"); err != nil || len(kvs) != 0 {
		t.Fatalf("missing bucket: got %v, %v; want empty", kvs, err)
	}
	for _, k := range []string{"b", "a", "c"} {
		if err := db.Put("h", k, []byte("v"+k)); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Delete("h", "b"); err != nil {
		t.Fatal(err)
	}
	if err := db.Delete("none", "x"); err != nil {
		t.Fatalf("delete from a missing bucket: %v", err)
	}
	kvs, err := db.List("h")
	if err != nil {
		t.Fatal(err)
	}
	if len(kvs) != 2 || kvs[0].Key != "a" || string(kvs[0].Value) != "va" || kvs[1].Key != "c" {
		t.Fatalf("got %+v; want a, c in key order", kvs)
	}
}

func TestTesterRunsStoreOnceAndKeepNewest(t *testing.T) {
	db, err := newBboltDb(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Release() }()
	d := &dbContext{db: db}

	saved, err := d.SaveTesterRun("run-000", []byte(`{"n":0}`), 3)
	if err != nil || !saved {
		t.Fatalf("first save: %v, %v", saved, err)
	}
	if saved, _ := d.SaveTesterRun("run-000", []byte(`{"n":"again"}`), 3); saved {
		t.Fatal("the same run must be stored once")
	}
	for _, id := range []string{"run-001", "run-002", "run-003"} {
		if _, err := d.SaveTesterRun(id, []byte(`{}`), 3); err != nil {
			t.Fatal(err)
		}
	}
	runs, err := d.ListTesterRuns()
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, r := range runs {
		ids = append(ids, r.Key)
	}
	if strings.Join(ids, ",") != "run-003,run-002,run-001" {
		t.Fatalf("got %v; want the newest 3, newest first", ids)
	}
	if got, _ := d.GetTesterRun("run-000"); got != nil {
		t.Fatalf("evicted run still there: %s", got)
	}
	if got, _ := d.GetTesterRun("run-002"); string(got) != `{}` {
		t.Fatalf("got %q", got)
	}
}

// A deleted run is gone and is not stored again when its tester still
// reports it; renaming edits the stored report, and a missing run is
// not found.
func TestTesterRunsDeleteAndUpdate(t *testing.T) {
	db, err := newBboltDb(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Release() }()
	d := &dbContext{db: db}
	for _, id := range []string{"run-1", "run-2"} {
		if _, err := d.SaveTesterRun(id, []byte(`{}`), 50); err != nil {
			t.Fatal(err)
		}
	}

	if err := d.DeleteTesterRuns([]string{"run-1", "never-stored"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := d.GetTesterRun("run-1"); got != nil {
		t.Fatalf("deleted run still there: %s", got)
	}
	if deleted, _ := d.TesterRunDeleted("run-1"); !deleted {
		t.Fatal("run-1 must be remembered as deleted")
	}
	if saved, _ := d.SaveTesterRun("run-1", []byte(`{}`), 50); saved {
		t.Fatal("a deleted run must not be stored again")
	}

	found, err := d.UpdateTesterRun("run-2", func(raw []byte) ([]byte, error) { return []byte(`{"name":"x"}`), nil })
	if err != nil || !found {
		t.Fatalf("update: %v, %v", found, err)
	}
	if got, _ := d.GetTesterRun("run-2"); string(got) != `{"name":"x"}` {
		t.Fatalf("after update %s", got)
	}
	if found, err := d.UpdateTesterRun("run-1", func(raw []byte) ([]byte, error) { return raw, nil }); found || err != nil {
		t.Fatalf("update of a deleted run: %v, %v", found, err)
	}
}
