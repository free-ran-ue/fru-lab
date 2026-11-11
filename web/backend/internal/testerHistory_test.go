package internal

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	loggergo "github.com/Alonza0314/logger-go/v2"
	"github.com/gin-gonic/gin"

	flctx "backend/internal/context"
)

// memRuns is an in-memory testerRunStore.
type memRuns struct {
	mu      sync.Mutex
	runs    map[string][]byte
	ids     []string // oldest first
	deleted map[string]bool
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

func (m *memRuns) TesterRunDeleted(id string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.deleted[id], nil
}

func (m *memRuns) DeleteTesterRuns(ids []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.deleted == nil {
		m.deleted = map[string]bool{}
	}
	for _, id := range ids {
		m.deleted[id] = true
		delete(m.runs, id)
		m.ids = slices.DeleteFunc(m.ids, func(x string) bool { return x == id })
	}
	return nil
}

func (m *memRuns) UpdateTesterRun(id string, edit func([]byte) ([]byte, error)) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	raw, ok := m.runs[id]
	if !ok {
		return false, nil
	}
	out, err := edit(raw)
	if err != nil {
		return true, err
	}
	m.runs[id] = out
	return true, nil
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
		if r.URL.Path != "/api/run/reports" {
			http.NotFound(w, r)
			return
		}
		second := strings.Replace(sampleReport, "20261002-150405", "20261002-150410", 1)
		_, _ = w.Write([]byte("[" + sampleReport + "," + second + "]"))
	}))
	defer tester.Close()
	store := &memRuns{}
	w := testLog()
	w.testers, _ = newTesterRegistry(tester.URL, "engine-token", &memTesters{})
	w.client, w.store = tester.Client(), store

	for range 3 {
		if err := w.poll(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if auth != "Bearer engine-token" {
		t.Fatalf("auth %q", auth)
	}
	// both runs, each once: the second one followed the first before a poll
	if strings.Join(store.ids, ",") != "20261002-150405,20261002-150410" {
		t.Fatalf("stored %v; want each run once", store.ids)
	}
	// the report as fru-tester sent it, plus the tester that ran it (named
	// by its id here: this fake has no /api/status)
	var stored map[string]json.RawMessage
	if err := json.Unmarshal(store.runs["20261002-150405"], &stored); err != nil {
		t.Fatal(err)
	}
	if string(stored["tester"]) != `{"id":"default","name":"default"}` || stored["snapshot"] == nil || stored["profile"] == nil {
		t.Fatalf("stored %s", store.runs["20261002-150405"])
	}
}

func TestWatcherStoresNothingWhileNoRunFinished(t *testing.T) {
	tester := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	}))
	defer tester.Close()
	store := &memRuns{}
	w := testLog()
	w.testers, _ = newTesterRegistry(tester.URL, "t", &memTesters{})
	w.client, w.store = tester.Client(), store
	if err := w.poll(context.Background()); err != nil {
		t.Fatalf("no finished run is not an error: %v", err)
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

func TestHistoryJSONAndHTMLExports(t *testing.T) {
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

	rec = get(t, h, "/api/tester/history/20261002-150405/report.html")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "20261002-150405") {
		t.Fatalf("report: %d %.200q", rec.Code, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("report type %q", ct)
	}

	if rec := get(t, h, "/api/tester/history/nope"); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown run: %d", rec.Code)
	}
	if rec := get(t, h, "/api/tester/history/nope/report.html"); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown run report: %d", rec.Code)
	}
}

// Runs from an added tester are stored too, under a key of their own, so
// a run ID both testers used is kept twice; the list says which tester ran
// each, and a run stored before testers could be added counts as the
// default tester's.
func TestHistoryKeepsWhichTesterRanEachRun(t *testing.T) {
	reports := func(name string) *httptest.Server {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/status":
				_, _ = w.Write([]byte(`{"name":"` + name + `"}`))
			case "/api/run/reports":
				_, _ = w.Write([]byte("[" + sampleReport + "]"))
			default:
				http.NotFound(w, r)
			}
		}))
		t.Cleanup(s.Close)
		return s
	}
	def, lab2 := reports("default-tester"), reports("lab-2")
	reg, _ := newTesterRegistry(def.URL, "t", &memTesters{})
	added, err := reg.add(context.Background(), lab2.URL, "t")
	if err != nil {
		t.Fatal(err)
	}
	store := &memRuns{}
	_, _ = store.SaveTesterRun("20261002-140000", []byte(strings.Replace(sampleReport, "20261002-150405", "20261002-140000", 1)), 50)
	w := testLog()
	w.testers, w.client, w.store = reg, http.DefaultClient, store
	if err := w.poll(context.Background()); err != nil {
		t.Fatal(err)
	}

	rec := get(t, historyRouter(store), "/api/tester/history")
	var list []testerHistorySummary
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, s := range list {
		got[s.ID] = s.TesterID + "/" + s.TesterName
	}
	want := map[string]string{
		"20261002-150405":             "default/default-tester",
		"20261002-150405@" + added.ID: added.ID + "/lab-2",
		"20261002-140000":             "default/",
	}
	if len(got) != len(want) {
		t.Fatalf("history %v; want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("history %v; want %v", got, want)
		}
	}
	if rec := get(t, historyRouter(store), "/api/tester/history/20261002-150405@"+added.ID); rec.Code != http.StatusOK {
		t.Fatalf("an added tester's run by its id: %d", rec.Code)
	}
}

func send(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rec, req)
	return rec
}

// Deleted runs leave the list, and the watcher does not copy them back
// from a tester that still reports them.
func TestHistoryDeleteRunsStaysDeleted(t *testing.T) {
	tester := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		second := strings.Replace(sampleReport, "20261002-150405", "20261002-150410", 1)
		_, _ = w.Write([]byte("[" + sampleReport + "," + second + "]"))
	}))
	defer tester.Close()
	store := &memRuns{}
	w := testLog()
	w.testers, _ = newTesterRegistry(tester.URL, "t", &memTesters{})
	w.client, w.store = tester.Client(), store
	if err := w.poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	h := historyRouter(store)

	if rec := send(t, h, http.MethodDelete, "/api/tester/history", `{"ids":[]}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("empty delete: %d", rec.Code)
	}
	if rec := send(t, h, http.MethodDelete, "/api/tester/history", `{"ids":["20261002-150405"]}`); rec.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body)
	}
	if err := w.poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if strings.Join(store.ids, ",") != "20261002-150410" {
		t.Fatalf("stored %v; want only the run that was not deleted", store.ids)
	}
}

// A run's name shows in the list and the HTML report; an empty name
// goes back to the run ID, and a name too long or a missing run is refused.
func TestHistoryRenameRun(t *testing.T) {
	store := &memRuns{}
	_, _ = store.SaveTesterRun("20261002-150405", []byte(sampleReport), 50)
	h := historyRouter(store)

	rec := send(t, h, http.MethodPatch, "/api/tester/history/20261002-150405", `{"name":"  baseline 1000 UE  "}`)
	var s testerHistorySummary
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &s) != nil || s.Name != "baseline 1000 UE" || s.RunID != "20261002-150405" {
		t.Fatalf("rename: %d %s", rec.Code, rec.Body)
	}
	var list []testerHistorySummary
	_ = json.Unmarshal(get(t, h, "/api/tester/history").Body.Bytes(), &list)
	if len(list) != 1 || list[0].Name != "baseline 1000 UE" {
		t.Fatalf("list %+v", list)
	}
	if body := get(t, h, "/api/tester/history/20261002-150405/report.html").Body.String(); !strings.Contains(body, "baseline 1000 UE") {
		t.Fatal("the report must show the run's name")
	}

	rec = send(t, h, http.MethodPatch, "/api/tester/history/20261002-150405", `{"name":""}`)
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &s) != nil || s.Name != "" {
		t.Fatalf("clear name: %d %s", rec.Code, rec.Body)
	}
	if strings.Contains(string(store.runs["20261002-150405"]), `"name":""`) {
		t.Fatal("a cleared name must be removed, not stored empty")
	}

	if rec := send(t, h, http.MethodPatch, "/api/tester/history/20261002-150405", `{"name":"`+strings.Repeat("名", 81)+`"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("too long: %d", rec.Code)
	}
	if rec := send(t, h, http.MethodPatch, "/api/tester/history/20261002-150405", `{}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("no name: %d", rec.Code)
	}
	if rec := send(t, h, http.MethodPatch, "/api/tester/history/nope", `{"name":"x"}`); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown run: %d", rec.Code)
	}
}
