package internal

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	flctx "backend/internal/context"
	"backend/logger"

	"github.com/gin-gonic/gin"
)

// memTesters keeps added testers in memory.
type memTesters struct{ recs []flctx.TesterRecord }

func (m *memTesters) ListTesters() ([]flctx.TesterRecord, error) { return m.recs, nil }
func (m *memTesters) AddTester(url, token string) (*flctx.TesterRecord, error) {
	r := flctx.TesterRecord{ID: "id" + string(rune('a'+len(m.recs))), URL: url, ApiToken: token, AddedAt: time.Now()}
	m.recs = append(m.recs, r)
	return &r, nil
}
func (m *memTesters) DeleteTester(id string) error {
	for i, r := range m.recs {
		if r.ID == id {
			m.recs = append(m.recs[:i], m.recs[i+1:]...)
			return nil
		}
	}
	return flctx.ErrTesterNotFound
}

// fakeTester answers like fru-tester: name on /api/status, the path it
// was asked for elsewhere, 401 for any other token.
func fakeTester(t *testing.T, name, token string) *httptest.Server {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.Path == "/api/status" {
			_ = json.NewEncoder(w).Encode(map[string]string{"name": name, "hostname": "h-" + name})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"tester": name, "path": r.URL.Path})
	}))
	t.Cleanup(s.Close)
	return s
}

func TestAddingATesterChecksItAnswers(t *testing.T) {
	def := fakeTester(t, "default-tester", "d")
	lab2 := fakeTester(t, "lab-2", "secret")
	store := &memTesters{}
	r, err := newTesterRegistry(def.URL, "d", store)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	if _, err := r.add(ctx, lab2.URL, "wrong"); err == nil || !strings.Contains(err.Error(), "rejected the API token") {
		t.Fatalf("wrong token: %v", err)
	}
	if _, err := r.add(ctx, "http://127.0.0.1:1", "x"); err == nil || !strings.Contains(err.Error(), "no fru-tester answered") {
		t.Fatalf("nothing there: %v", err)
	}
	if _, err := r.add(ctx, "10.0.0.2:9100", "x"); err == nil || !strings.Contains(err.Error(), "must be like") {
		t.Fatalf("no scheme: %v", err)
	}
	if _, err := r.add(ctx, def.URL+"/", "d"); err == nil || !strings.Contains(err.Error(), "already on the list") {
		t.Fatalf("the default again: %v", err)
	}
	if len(store.recs) != 0 {
		t.Fatalf("a refused tester was stored: %+v", store.recs)
	}

	added, err := r.add(ctx, " "+lab2.URL+"/ ", " secret ")
	if err != nil {
		t.Fatal(err)
	}
	if added.Name != "lab-2" || added.URL != lab2.URL || added.BuiltIn {
		t.Fatalf("added %+v", added)
	}
	if list := r.list(); len(list) != 2 || list[0].ID != defaultTesterID || !list[0].BuiltIn || list[1].ID != added.ID {
		t.Fatalf("list %+v", list)
	}

	// a restarted fru-lab finds it again
	again, err := newTesterRegistry(def.URL, "d", store)
	if err != nil || len(again.list()) != 2 {
		t.Fatalf("reload: %v %+v", err, again.list())
	}
}

func TestRemovingTesters(t *testing.T) {
	def := fakeTester(t, "default-tester", "d")
	lab2 := fakeTester(t, "lab-2", "s")
	store := &memTesters{}
	r, _ := newTesterRegistry(def.URL, "d", store)
	added, err := r.add(context.Background(), lab2.URL, "s")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.remove(defaultTesterID); err != errTesterBuiltIn {
		t.Fatalf("removing the default: %v", err)
	}
	if err := r.remove("nope"); err != errTesterNotFound {
		t.Fatalf("unknown: %v", err)
	}
	if err := r.remove(added.ID); err != nil {
		t.Fatal(err)
	}
	if len(r.list()) != 1 || len(store.recs) != 0 {
		t.Fatalf("after remove: %+v %+v", r.list(), store.recs)
	}
}

// /api/testers/<id>/<rest> reaches that tester's /api/<rest> with its token.
func TestTesterRoutesForwardToTheRightTester(t *testing.T) {
	gin.SetMode(gin.TestMode)
	def := fakeTester(t, "default-tester", "d")
	lab2 := fakeTester(t, "lab-2", "s")
	r, _ := newTesterRegistry(def.URL, "d", &memTesters{})
	added, err := r.add(context.Background(), lab2.URL, "s")
	if err != nil {
		t.Fatal(err)
	}
	b := &backend{testers: r, BackendLogger: logger.NewBackendLogger("error", "", true)}
	router := gin.New()
	addRoutes(router.Group("/api"), b.getTestersRoutes())
	front := httptest.NewServer(router)
	t.Cleanup(front.Close)
	do := func(method, path string) (int, string) {
		req, _ := http.NewRequest(method, front.URL+path, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		body, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(body)
	}

	for _, tc := range []struct{ id, want string }{{defaultTesterID, "default-tester"}, {added.ID, "lab-2"}} {
		code, body := do(http.MethodGet, "/api/testers/"+tc.id+"/system/cpuload")
		var got map[string]string
		_ = json.Unmarshal([]byte(body), &got)
		if code != http.StatusOK || got["tester"] != tc.want || got["path"] != "/api/system/cpuload" {
			t.Fatalf("%s: %d %s", tc.id, code, body)
		}
	}
	if code, _ := do(http.MethodGet, "/api/testers/nope/status"); code != http.StatusNotFound {
		t.Fatalf("unknown tester: %d", code)
	}
	if code, _ := do(http.MethodDelete, "/api/testers/default"); code != http.StatusConflict {
		t.Fatalf("delete default: %d", code)
	}
	if _, body := do(http.MethodGet, "/api/testers"); !strings.Contains(body, `"builtIn":true`) || strings.Contains(body, "apiToken") {
		t.Fatalf("list must not carry tokens: %s", body)
	}
}
