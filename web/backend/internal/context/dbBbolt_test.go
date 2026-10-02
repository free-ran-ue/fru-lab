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
