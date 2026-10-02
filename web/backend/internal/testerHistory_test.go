package internal

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	loggergo "github.com/Alonza0314/logger-go/v2"
	"github.com/gin-gonic/gin"

	flctx "backend/internal/context"
)

// memRuns is an in-memory testerRunStore.
type memRuns struct {
	mu   sync.Mutex
	runs map[string][]byte
	ids  []string // oldest first
}

func (m *memRuns) SaveTesterRun(id string, report []byte, keep int) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.runs == nil {
		m.runs = map[string][]byte{}
	}
	if _, ok := m.runs[id]; ok {
		return false, nil
	}
	m.runs[id] = report
	m.ids = append(m.ids, id)
	return true, nil
}

func (m *memRuns) ListTesterRuns() ([]flctx.KV, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []flctx.KV
	for i := len(m.ids) - 1; i >= 0; i-- {
		out = append(out, flctx.KV{Key: m.ids[i], Value: m.runs[m.ids[i]]})
	}
	return out, nil
}

func (m *memRuns) GetTesterRun(id string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.runs[id], nil
}

const sampleReport = `{"profile":{"name":"basic"},"snapshot":{"runId":"20261002-150405","profileName":"basic","state":"stopped","error":"","stopReason":"user",
"startedAt":"2026-10-02T15:04:05Z","stoppedAt":"2026-10-02T15:05:05Z",
"n2":{"expected":2,"accepted":2},"registration":{"expected":4,"accepted":4,"p95Ms":120.5},"pdu":{"expected":4,"accepted":3},
"deregistration":{"expected":4,"accepted":3},
"dataplane":{"ul":{"txBytes":1000,"rxBytes":990,"lossRate":0.01},"dl":{"txBytes":5000,"rxBytes":5000,"lossRate":0},
"series":[{"t":1,"ulTxBps":8,"ulRxBps":8,"dlTxBps":40,"dlRxBps":40},{"t":2,"ulTxBps":8,"ulRxBps":7.5,"dlTxBps":40,"dlRxBps":40}]}}}`

func testLog() *testerHistoryWatcher {
	lg := loggergo.NewLogger("", true)
	lg.SetLevel("error")
	return &testerHistoryWatcher{log: lg.WithTags("TEST")}
}

func TestWatcherStoresEachRunOnce(t *testing.T) {
	var auth string
	tester := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		if r.URL.Path != "/api/run/report" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(sampleReport))
	}))
	defer tester.Close()
	store := &memRuns{}
	w := testLog()
	w.url, w.token, w.client, w.store = tester.URL, "engine-token", tester.Client(), store

	for range 3 {
		if err := w.poll(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if auth != "Bearer engine-token" {
		t.Fatalf("auth %q", auth)
	}
	if len(store.ids) != 1 || store.ids[0] != "20261002-150405" {
		t.Fatalf("stored %v; want the run once", store.ids)
	}
}

func TestWatcherStoresNothingWhileNoRunFinished(t *testing.T) {
	tester := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"no finished run"}`))
	}))
	defer tester.Close()
	store := &memRuns{}
	w := testLog()
	w.url, w.token, w.client, w.store = tester.URL, "t", tester.Client(), store
	if err := w.poll(context.Background()); err != nil {
		t.Fatalf("404 is not an error: %v", err)
	}
	if len(store.ids) != 0 {
		t.Fatalf("stored %v", store.ids)
	}
}

func historyRouter(store testerRunStore) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	addTesterHistoryRoutes(r.Group("/api"), store)
	return r
}

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestHistoryListSummarizesRunsNewestFirst(t *testing.T) {
	store := &memRuns{}
	_, _ = store.SaveTesterRun("20261002-150405", []byte(sampleReport), 50)
	_, _ = store.SaveTesterRun("20261002-160000", []byte(strings.Replace(sampleReport, "20261002-150405", "20261002-160000", 1)), 50)
	rec := get(t, historyRouter(store), "/api/tester/history")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var list []testerHistorySummary
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].RunID != "20261002-160000" {
		t.Fatalf("got %+v", list)
	}
	s := list[1]
	if s.ProfileName != "basic" || s.State != "stopped" || s.StopReason != "user" || s.GnbCount != 2 || s.UeCount != 4 ||
		s.Registered != 4 || s.RegistrationP95Ms != 120.5 || s.Established != 3 || s.Deregistered != 3 ||
		s.UlRxBytes != 990 || s.DlRxBytes != 5000 || s.UlLossRate != 0.01 {
		t.Fatalf("summary %+v", s)
	}
}

func TestHistoryEmptyListIsAnArray(t *testing.T) {
	rec := get(t, historyRouter(&memRuns{}), "/api/tester/history")
	if strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("got %q", rec.Body)
	}
}

func TestHistoryJSONAndCSVExports(t *testing.T) {
	store := &memRuns{}
	_, _ = store.SaveTesterRun("20261002-150405", []byte(sampleReport), 50)
	h := historyRouter(store)

	rec := get(t, h, "/api/tester/history/20261002-150405")
	if rec.Code != http.StatusOK || rec.Body.String() != sampleReport {
		t.Fatalf("json: %d %q", rec.Code, rec.Body)
	}
	if cd := rec.Header().Get("Content-Disposition"); cd != `attachment; filename="tester-20261002-150405.json"` {
		t.Fatalf("json disposition %q", cd)
	}

	rec = get(t, h, "/api/tester/history/20261002-150405/series.csv")
	want := "t,ulTxBps,ulRxBps,dlTxBps,dlRxBps\n1,8,8,40,40\n2,8,7.5,40,40\n"
	if rec.Code != http.StatusOK || rec.Body.String() != want {
		t.Fatalf("csv: %d %q", rec.Code, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
		t.Fatalf("csv type %q", ct)
	}

	if rec := get(t, h, "/api/tester/history/nope"); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown run: %d", rec.Code)
	}
	if rec := get(t, h, "/api/tester/history/nope/series.csv"); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown run csv: %d", rec.Code)
	}
}
