package context

import (
	"errors"
	"testing"
)

func TestTestersAddListDelete(t *testing.T) {
	d := newProfileDb(t)
	if list, err := d.ListTesters(); err != nil || len(list) != 0 {
		t.Fatalf("empty db: %v %v", list, err)
	}
	a, err := d.AddTester("http://10.0.0.2:9100", "tok-a")
	if err != nil {
		t.Fatal(err)
	}
	b, err := d.AddTester("http://10.0.0.3:9100", "tok-b")
	if err != nil {
		t.Fatal(err)
	}
	list, err := d.ListTesters()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].ID != a.ID || list[1].ID != b.ID || list[0].ApiToken != "tok-a" {
		t.Fatalf("list %+v; want a then b, with their tokens", list)
	}
	if err := d.DeleteTester(a.ID); err != nil {
		t.Fatal(err)
	}
	if err := d.DeleteTester(a.ID); !errors.Is(err, ErrTesterNotFound) {
		t.Fatalf("second delete: %v", err)
	}
	if list, _ := d.ListTesters(); len(list) != 1 || list[0].ID != b.ID {
		t.Fatalf("after delete %+v", list)
	}
}
