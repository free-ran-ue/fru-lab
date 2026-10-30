package internal

import (
	"bufio"
	"os"
	"regexp"
	"strings"
	"testing"

	"backend/logger"
)

// Every /api/tester/ operation in openapi.yaml needs a route here, or the
// request falls through to the frontend's index.html. History is served
// by its own router (testerHistory.go).
func TestEveryTesterOperationInTheSpecHasARoute(t *testing.T) {
	f, err := os.Open("../../openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	pathLine := regexp.MustCompile(`^  (/api/tester/[^:]+):\s*$`)
	opLine := regexp.MustCompile(`^    (get|post|put|delete|patch):\s*$`)
	want := map[string]bool{}
	path := ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if m := pathLine.FindStringSubmatch(sc.Text()); m != nil {
			path = m[1]
			if strings.HasPrefix(path, "/api/tester/history") {
				path = ""
			}
			continue
		}
		if strings.HasPrefix(sc.Text(), "  /") {
			path = ""
		}
		if m := opLine.FindStringSubmatch(sc.Text()); m != nil && path != "" {
			gin := regexp.MustCompile(`\{([^}]+)\}`).ReplaceAllString(strings.TrimPrefix(path, "/api"), ":$1")
			want[strings.ToUpper(m[1])+" "+gin] = true
		}
	}
	if len(want) == 0 {
		t.Fatal("found no /api/tester operations in openapi.yaml")
	}
	have := map[string]bool{}
	b := &backend{BackendLogger: logger.NewBackendLogger("error", "", true)}
	for _, r := range b.getTesterRoutes() {
		have[r.Method+" "+r.Pattern] = true
	}
	for op := range want {
		if !have[op] {
			t.Errorf("openapi.yaml has %s but getTesterRoutes does not", op)
		}
	}
}
