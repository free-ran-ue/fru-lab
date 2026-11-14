package netcfg

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// Claims are seen by every claimant on the host and go with their holder.
func TestClaimsAreSharedAndReleased(t *testing.T) {
	m := Netlink{}
	key := fmt.Sprintf("test/%d/10.0.3.2/24", os.Getpid())
	a, err := m.Claim(key, "10.0.3.2/32")
	if err != nil {
		t.Fatal(err)
	}
	b, err := m.Claim(key, "-")
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := m.Claims(key); len(got) != 2 || !strings.Contains(strings.Join(got, ","), "10.0.3.2/32") {
		t.Fatalf("claims %v; want both, with their notes", got)
	}
	a()
	if got, _ := m.Claims(key); len(got) != 1 || got[0] != "-" {
		t.Fatalf("after one release %v", got)
	}
	b()
	if got, _ := m.Claims(key); len(got) != 0 {
		t.Fatalf("after both %v", got)
	}
	if got, _ := m.Claims(key + "x"); len(got) != 0 {
		t.Fatal("another key must not match")
	}
}

// Lock lets one holder in at a time.
func TestLockIsExclusive(t *testing.T) {
	m := Netlink{}
	unlock, err := m.Lock()
	if err != nil {
		t.Fatal(err)
	}
	got := make(chan time.Time, 1)
	go func() {
		u, err := m.Lock()
		if err == nil {
			u()
		}
		got <- time.Now()
	}()
	time.Sleep(50 * time.Millisecond)
	released := time.Now()
	unlock()
	if at := <-got; at.Before(released) {
		t.Fatal("the second Lock went in while the first was held")
	}
}
