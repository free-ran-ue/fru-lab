package internal

import (
	"strings"
	"testing"
)

func TestUeTestCommand(t *testing.T) {
	q := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	cmd, problem := ueTestCommand(q(map[string]string{"action": "ping", "target": "8.8.8.8", "count": "3"}))
	if problem != "" || strings.Join(cmd, " ") != "ping -I ueTun0 -c 3 -W 2 8.8.8.8" {
		t.Fatalf("ping: %v %q", cmd, problem)
	}
	cmd, _ = ueTestCommand(q(map[string]string{"action": "curl", "target": "http://example.com", "dev": "ueTun3"}))
	if cmd[2] != "ueTun3" {
		t.Fatalf("curl dev: %v", cmd)
	}
	for _, bad := range []map[string]string{
		{"action": "ping", "target": "8.8.8.8; rm -rf /"},
		{"action": "curl", "target": "file:///etc/passwd"},
		{"action": "sh", "target": "x"},
	} {
		if _, problem := ueTestCommand(q(bad)); problem == "" {
			t.Errorf("accepted %v", bad)
		}
	}
	cmd, _ = ueTestCommand(q(map[string]string{"action": "ping", "target": "x", "dev": "eth0"}))
	if cmd[2] != "ueTun0" {
		t.Errorf("dev eth0 not refused: %v", cmd)
	}
}
