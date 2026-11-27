package context

import (
	"path/filepath"
	"testing"
)

func TestBboltGetPut(t *testing.T) {
	db, err := newBboltDb(filepath.Join(t.TempDir(), "sub", "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Release() }()

	got, err := db.Get("bucket", "key")
	if err != nil || got != nil {
		t.Fatalf("missing bucket: got %q, %v; want nil, nil", got, err)
	}
	if err := db.Put("bucket", "key", []byte(`{"name":"a"}`)); err != nil {
		t.Fatal(err)
	}
	if err := db.Put("bucket", "key", []byte(`{"name":"b"}`)); err != nil {
		t.Fatal(err)
	}
	got, err = db.Get("bucket", "key")
	if err != nil || string(got) != `{"name":"b"}` {
		t.Fatalf("got %q, %v; want the second write", got, err)
	}
	got, err = db.Get("bucket", "other")
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
