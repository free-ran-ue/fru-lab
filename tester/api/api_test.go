package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"

	"tester/bench"
	"tester/profile"
	"tester/run"
)

type fakeCtrl struct {
	validateErr error
	startErr    error
	stopErr     error
	snap        run.Snapshot
	changed     chan struct{}
	reports     []run.Report
}

func (f *fakeCtrl) Validate(profile.Profile) (*profile.Plan, error) {
	if f.validateErr != nil {
		return nil, f.validateErr
	}
	return &profile.Plan{UesPerGnb: 4}, nil
}
func (f *fakeCtrl) Start(profile.Profile) (run.Snapshot, error) { return f.snap, f.startErr }
func (f *fakeCtrl) Stop() (run.Snapshot, error)                 { return f.snap, f.stopErr }
func (f *fakeCtrl) Snapshot() run.Snapshot                      { return f.snap }
func (f *fakeCtrl) Changed() <-chan struct{}                    { return f.changed }
func (f *fakeCtrl) Reports() []run.Report                       { return f.reports }

// fakeBench stands in for *bench.Runner.
type fakeBench struct {
	running  bool
	startErr error
	result   bench.Result
}

func (f *fakeBench) Start(bench.Settings) (bench.Result, error) { return f.result, f.startErr }
func (f *fakeBench) Result() bench.Result                       { return f.result }
func (f *fakeBench) Running() bool                              { return f.running }

func newTestRouter(ctrl Controller, token string) http.Handler {
	return NewRouter(ctrl, &fakeBench{}, token)
}

func do(t *testing.T, h http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestTokenRequired(t *testing.T) {
	h := newTestRouter(&fakeCtrl{}, "secret")
	require.Equal(t, http.StatusUnauthorized, do(t, h, http.MethodGet, "/api/run", "", "").Code)
	require.Equal(t, http.StatusUnauthorized, do(t, h, http.MethodGet, "/api/run", "", "wrong").Code)
	require.Equal(t, http.StatusOK, do(t, h, http.MethodGet, "/api/run", "", "secret").Code)
}

func TestValidateReturnsPlanOrFieldErrors(t *testing.T) {
	ctrl := &fakeCtrl{}
	h := newTestRouter(ctrl, "t")

	rec := do(t, h, http.MethodPost, "/api/profile/validate", `{"name":"x"}`, "t")
	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"valid":true,"errors":[],"plan":{"gnbs":null,"uesPerGnb":4,"n2Prefix":0,"n3Prefix":0}}`, rec.Body.String())

	ctrl.validateErr = &profile.ValidationError{Errors: []profile.FieldError{{Field: "name", Message: "must not be empty"}}}
	rec = do(t, h, http.MethodPost, "/api/profile/validate", `{}`, "t")
	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"valid":false,"errors":[{"field":"name","message":"must not be empty"}],"plan":null}`, rec.Body.String())

	rec = do(t, h, http.MethodPost, "/api/profile/validate", `{"bogus":1}`, "t")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), `unknown field \"bogus\"`)
}

func TestStartAndStopStatusCodes(t *testing.T) {
	ctrl := &fakeCtrl{snap: run.Snapshot{RunID: "r1", State: run.StateConfiguring}}
	h := newTestRouter(ctrl, "t")

	require.Equal(t, http.StatusAccepted, do(t, h, http.MethodPost, "/api/run", `{}`, "t").Code)

	ctrl.startErr = run.ErrRunActive
	require.Equal(t, http.StatusConflict, do(t, h, http.MethodPost, "/api/run", `{}`, "t").Code)

	ctrl.startErr = &profile.ValidationError{Errors: []profile.FieldError{{Field: "scale.gnbCount", Message: "must be at least 1"}}}
	rec := do(t, h, http.MethodPost, "/api/run", `{}`, "t")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.JSONEq(t, `{"message":"profile is invalid","errors":[{"field":"scale.gnbCount","message":"must be at least 1"}]}`, rec.Body.String())

	require.Equal(t, http.StatusAccepted, do(t, h, http.MethodPost, "/api/run/stop", "", "t").Code)
	ctrl.stopErr = run.ErrNotRunning
	require.Equal(t, http.StatusConflict, do(t, h, http.MethodPost, "/api/run/stop", "", "t").Code)
}

func TestStreamSendsSnapshotOnConnectAndOnChange(t *testing.T) {
	ctrl := &fakeCtrl{snap: run.Snapshot{RunID: "r1", State: run.StateN2}, changed: make(chan struct{})}
	srv := httptest.NewServer(newTestRouter(ctrl, "t"))
	defer srv.Close()

	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/run/stream"
	_, resp, err := websocket.DefaultDialer.Dial(url, nil)
	require.Error(t, err)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	conn, _, err := websocket.DefaultDialer.Dial(url, http.Header{"Authorization": {"Bearer t"}})
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	var got run.Snapshot
	require.NoError(t, conn.ReadJSON(&got))
	require.Equal(t, run.StateN2, got.State)

	ctrl.snap = run.Snapshot{RunID: "r1", State: run.StateRunning}
	close(ctrl.changed) // the fake never replaces it, so later frames come at once
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
	require.NoError(t, conn.ReadJSON(&got))
	require.Equal(t, run.StateRunning, got.State)
}

func TestSnapshotJSONShape(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, json.NewEncoder(&buf).Encode(run.Snapshot{State: run.StateIdle}))
	for _, key := range []string{`"runId"`, `"state":"idle"`, `"n2"`, `"gnbs"`, `"startedAt"`} {
		require.Contains(t, buf.String(), key)
	}
}

func TestStreamCoalescesBurstsOfChanges(t *testing.T) {
	ctrl := &fakeCtrl{snap: run.Snapshot{State: run.StateRunning}, changed: make(chan struct{})}
	close(ctrl.changed) // every frame sees "changed" at once, like a busy run
	srv := httptest.NewServer(newTestRouter(ctrl, "t"))
	defer srv.Close()
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/run/stream"
	conn, _, err := websocket.DefaultDialer.Dial(url, http.Header{"Authorization": {"Bearer t"}})
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	var got run.Snapshot
	start := time.Now()
	for range 4 {
		require.NoError(t, conn.ReadJSON(&got))
	}
	require.GreaterOrEqual(t, time.Since(start), 3*minFrameGap-20*time.Millisecond,
		"4 frames need at least 3 gaps")
}

func TestReportsIsAnEmptyListBeforeAnyRunFinished(t *testing.T) {
	h := newTestRouter(&fakeCtrl{}, "t")
	rec := do(t, h, http.MethodGet, "/api/run/reports", "", "t")
	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `[]`, rec.Body.String())
}

func TestReportsCarryProfileAndSnapshot(t *testing.T) {
	rep := run.Report{Profile: profile.Profile{Name: "basic"}, Snapshot: run.Snapshot{RunID: "r1", State: run.StateStopped}}
	h := newTestRouter(&fakeCtrl{reports: []run.Report{rep}}, "t")
	rec := do(t, h, http.MethodGet, "/api/run/reports", "", "t")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `[{"profile":{"name":"basic"`)
	require.Contains(t, rec.Body.String(), `"snapshot":{"runId":"r1"`)
}

func TestBenchStartAndResult(t *testing.T) {
	b := &fakeBench{result: bench.Result{State: bench.StateRunning, Cpus: 6, PlannedSenders: []int{1, 2, 4, 6}, Steps: []bench.Step{}}}
	h := NewRouter(&fakeCtrl{}, b, "t")
	rec := do(t, h, http.MethodPost, "/api/bench", `{"packetSize":1400,"stepSeconds":3}`, "t")
	require.Equal(t, http.StatusAccepted, rec.Code)
	require.Contains(t, rec.Body.String(), `"plannedSenders":[1,2,4,6]`)
	rec = do(t, h, http.MethodGet, "/api/bench", "", "t")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"state":"running"`)
}

func TestBenchRejectsBadSettingsAndConflicts(t *testing.T) {
	for err, code := range map[error]int{
		bench.ErrInvalidSettings: http.StatusBadRequest,
		bench.ErrBenchRunning:    http.StatusConflict,
		bench.ErrRunActive:       http.StatusConflict,
	} {
		h := NewRouter(&fakeCtrl{}, &fakeBench{startErr: err}, "t")
		rec := do(t, h, http.MethodPost, "/api/bench", `{"packetSize":1400,"stepSeconds":3}`, "t")
		require.Equal(t, code, rec.Code, err.Error())
	}
}

func TestARunCannotStartDuringABench(t *testing.T) {
	h := NewRouter(&fakeCtrl{}, &fakeBench{running: true}, "t")
	rec := do(t, h, http.MethodPost, "/api/run", `{}`, "t")
	require.Equal(t, http.StatusConflict, rec.Code)
	require.Contains(t, rec.Body.String(), "bench")
}
