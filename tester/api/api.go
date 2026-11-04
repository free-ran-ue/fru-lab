// Package api is fru-tester's HTTP surface. fru-lab's backend is the only
// intended client: it authenticates users and forwards requests here with
// the shared API token.
package api

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"tester/bench"
	"tester/cpuset"
	"tester/hoststat"
	"tester/netcfg"
	"tester/profile"
	"tester/run"
)

// Controller is the slice of *run.Controller the handlers use.
type Controller interface {
	Validate(p profile.Profile) (*profile.Plan, error)
	Start(p profile.Profile) (run.Snapshot, error)
	Stop() (run.Snapshot, error)
	Snapshot() run.Snapshot
	SnapshotPage(q run.GnbPage) run.Snapshot
	Changed() <-chan struct{}
	Reports() []run.Report
	Ping(p profile.Profile, plane string) (run.PingResult, error)
	Interfaces() ([]netcfg.InterfaceInfo, error)
	System() (run.SystemInfo, error)
	SetCpus(a cpuset.Allocation) (cpuset.Allocation, error)
}

// CpusErrorResponse carries field errors when the CPU allocation is refused.
type CpusErrorResponse struct {
	Message string              `json:"message"`
	Errors  []cpuset.FieldError `json:"errors"`
}

// Bench is the slice of *bench.Runner the handlers use.
type Bench interface {
	Start(s bench.Settings) (bench.Result, error)
	Result() bench.Result
	Running() bool
}

type MessageResponse struct {
	Message string `json:"message"`
}

// ValidateResponse is returned for both valid and invalid profiles, so the
// setup page can render the plan or the field errors from one call.
type ValidateResponse struct {
	Valid  bool                 `json:"valid"`
	Errors []profile.FieldError `json:"errors"`
	Plan   *profile.Plan        `json:"plan"`
}

// PingRequest asks for a ping test of one network ("n2", "n3" or "n6") as
// profile configures it.
type PingRequest struct {
	Plane   string          `json:"plane"`
	Profile profile.Profile `json:"profile"`
}

// StartErrorResponse carries field errors when Start rejects the profile.
type StartErrorResponse struct {
	Message string               `json:"message"`
	Errors  []profile.FieldError `json:"errors"`
}

// streamInterval is how often the stream re-sends a snapshot even when no
// state changed, so totals and in-flight timers keep moving.
// minFrameGap bounds the frame rate: with thousands of UEs every step is a
// state change, and a full snapshot per change would flood the browser.
const (
	streamInterval = time.Second
	minFrameGap    = 200 * time.Millisecond
)

// NewRouter serves the API; name is this tester's name in /status.
func NewRouter(ctrl Controller, b Bench, apiToken, name string) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	g := r.Group("/api", requireToken(apiToken))
	g.POST("/profile/validate", handleValidate(ctrl))
	g.GET("/run", func(c *gin.Context) {
		q, err := gnbPageQuery(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, MessageResponse{Message: err.Error()})
			return
		}
		c.JSON(http.StatusOK, ctrl.SnapshotPage(q))
	})
	g.POST("/run", handleStart(ctrl, b))
	g.POST("/run/stop", handleStop(ctrl))
	g.GET("/run/stream", handleStream(ctrl))
	g.GET("/bench", func(c *gin.Context) { c.JSON(http.StatusOK, b.Result()) })
	g.POST("/bench", handleBenchStart(b))
	g.POST("/network/ping", handlePing(ctrl))
	g.GET("/network/interfaces", func(c *gin.Context) {
		ifaces, err := ctrl.Interfaces()
		if err != nil {
			c.JSON(http.StatusInternalServerError, MessageResponse{Message: err.Error()})
			return
		}
		c.JSON(http.StatusOK, ifaces)
	})
	g.GET("/system", func(c *gin.Context) {
		info, err := ctrl.System()
		if err != nil {
			c.JSON(http.StatusInternalServerError, MessageResponse{Message: err.Error()})
			return
		}
		c.JSON(http.StatusOK, info)
	})
	g.PUT("/system/cpus", handleSetCpus(ctrl))
	g.GET("/status", func(c *gin.Context) {
		s := hoststat.Read()
		s.Name = name
		c.JSON(http.StatusOK, s)
	})
	cpuLoad := cpuset.NewSampler()
	g.GET("/system/cpuload", func(c *gin.Context) {
		l, err := cpuLoad.Read()
		if err != nil {
			c.JSON(http.StatusInternalServerError, MessageResponse{Message: err.Error()})
			return
		}
		c.JSON(http.StatusOK, l)
	})
	g.GET("/run/reports", func(c *gin.Context) {
		reps := ctrl.Reports()
		if reps == nil {
			reps = []run.Report{} // always a list
		}
		c.JSON(http.StatusOK, reps)
	})
	return r
}

func requireToken(token string) gin.HandlerFunc {
	want := []byte("Bearer " + token)
	return func(c *gin.Context) {
		got := []byte(c.GetHeader("Authorization"))
		if subtle.ConstantTimeCompare(got, want) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, MessageResponse{Message: "missing or wrong API token"})
			return
		}
		c.Next()
	}
}

func bindProfile(c *gin.Context) (profile.Profile, bool) {
	var p profile.Profile
	dec := json.NewDecoder(c.Request.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		c.JSON(http.StatusBadRequest, MessageResponse{Message: "invalid profile JSON: " + err.Error()})
		return p, false
	}
	return p, true
}

func handleValidate(ctrl Controller) gin.HandlerFunc {
	return func(c *gin.Context) {
		p, ok := bindProfile(c)
		if !ok {
			return
		}
		plan, err := ctrl.Validate(p)
		var verr *profile.ValidationError
		switch {
		case err == nil:
			c.JSON(http.StatusOK, ValidateResponse{Valid: true, Errors: []profile.FieldError{}, Plan: plan})
		case errors.As(err, &verr):
			c.JSON(http.StatusOK, ValidateResponse{Valid: false, Errors: verr.Errors})
		default:
			c.JSON(http.StatusInternalServerError, MessageResponse{Message: err.Error()})
		}
	}
}

func handleStart(ctrl Controller, b Bench) gin.HandlerFunc {
	return func(c *gin.Context) {
		if b.Running() { // a run and a bench would compete for the CPUs
			c.JSON(http.StatusConflict, MessageResponse{Message: "a bench is running; start the run when it has finished"})
			return
		}
		p, ok := bindProfile(c)
		if !ok {
			return
		}
		snap, err := ctrl.Start(p)
		var verr *profile.ValidationError
		var aerr *run.CpuAllocationError
		switch {
		case err == nil:
			c.JSON(http.StatusAccepted, snap)
		case errors.As(err, &verr):
			c.JSON(http.StatusBadRequest, StartErrorResponse{Message: "profile is invalid", Errors: verr.Errors})
		case errors.Is(err, run.ErrRunActive), errors.Is(err, run.ErrPingActive):
			c.JSON(http.StatusConflict, MessageResponse{Message: err.Error()})
		case errors.As(err, &aerr):
			c.JSON(http.StatusBadRequest, MessageResponse{Message: err.Error() + " (fix it on the Status page)"})
		default:
			c.JSON(http.StatusInternalServerError, MessageResponse{Message: err.Error()})
		}
	}
}

// handleSetCpus stores the CPU allocation the next run uses: 200 with it
// in canonical form, 400 with field errors, 409 while a run is active.
func handleSetCpus(ctrl Controller) gin.HandlerFunc {
	return func(c *gin.Context) {
		var a cpuset.Allocation
		dec := json.NewDecoder(c.Request.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&a); err != nil {
			c.JSON(http.StatusBadRequest, CpusErrorResponse{Message: "invalid CPU allocation JSON: " + err.Error(), Errors: []cpuset.FieldError{}})
			return
		}
		saved, err := ctrl.SetCpus(a)
		var aerr *run.CpuAllocationError
		switch {
		case err == nil:
			c.JSON(http.StatusOK, saved)
		case errors.As(err, &aerr):
			c.JSON(http.StatusBadRequest, CpusErrorResponse{Message: "CPU allocation is invalid", Errors: aerr.Errors})
		case errors.Is(err, run.ErrRunActive):
			c.JSON(http.StatusConflict, MessageResponse{Message: "the CPU allocation cannot change during a run; stop it first"})
		default:
			c.JSON(http.StatusInternalServerError, MessageResponse{Message: err.Error()})
		}
	}
}

// handlePing runs a ping test and answers 200 with its result, whether or
// not replies came back.
func handlePing(ctrl Controller) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req PingRequest
		dec := json.NewDecoder(c.Request.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil {
			c.JSON(http.StatusBadRequest, MessageResponse{Message: "invalid ping request JSON: " + err.Error()})
			return
		}
		res, err := ctrl.Ping(req.Profile, req.Plane)
		var verr *profile.ValidationError
		switch {
		case err == nil:
			c.JSON(http.StatusOK, res)
		case errors.As(err, &verr):
			c.JSON(http.StatusBadRequest, StartErrorResponse{Message: "fix this network's settings first", Errors: verr.Errors})
		case errors.Is(err, profile.ErrUnknownPlane):
			c.JSON(http.StatusBadRequest, StartErrorResponse{Message: err.Error(), Errors: []profile.FieldError{}})
		case errors.Is(err, run.ErrRunActive), errors.Is(err, run.ErrPingActive):
			c.JSON(http.StatusConflict, MessageResponse{Message: err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, MessageResponse{Message: err.Error()})
		}
	}
}

func handleBenchStart(b Bench) gin.HandlerFunc {
	return func(c *gin.Context) {
		var s bench.Settings
		if err := c.ShouldBindJSON(&s); err != nil {
			c.JSON(http.StatusBadRequest, MessageResponse{Message: "invalid bench settings: " + err.Error()})
			return
		}
		res, err := b.Start(s)
		switch {
		case err == nil:
			c.JSON(http.StatusAccepted, res)
		case errors.Is(err, bench.ErrInvalidSettings):
			c.JSON(http.StatusBadRequest, MessageResponse{Message: err.Error()})
		case errors.Is(err, bench.ErrBenchRunning), errors.Is(err, bench.ErrRunActive):
			c.JSON(http.StatusConflict, MessageResponse{Message: err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, MessageResponse{Message: err.Error()})
		}
	}
}

func handleStop(ctrl Controller) gin.HandlerFunc {
	return func(c *gin.Context) {
		snap, err := ctrl.Stop()
		if errors.Is(err, run.ErrNotRunning) {
			c.JSON(http.StatusConflict, MessageResponse{Message: err.Error()})
			return
		}
		c.JSON(http.StatusAccepted, snap)
	}
}

var upgrader = websocket.Upgrader{
	// Only fru-lab's backend reaches this port (token-guarded above).
	CheckOrigin: func(*http.Request) bool { return true },
}

// GnbPageRequest picks the gNBs a snapshot lists: query parameters on
// GET /run and /run/stream, or a JSON text message the stream's client
// sends to change page. No gnbLimit lists every gNB.
type GnbPageRequest struct {
	GnbOffset int    `json:"gnbOffset"`
	GnbLimit  *int   `json:"gnbLimit"`
	GnbState  string `json:"gnbState"` // comma-separated states; empty is all
}

func (r GnbPageRequest) page() (run.GnbPage, error) {
	q := run.AllGnbs
	if r.GnbLimit != nil {
		if *r.GnbLimit < 0 || *r.GnbLimit > run.MaxGnbPage {
			return q, fmt.Errorf("gnbLimit must be between 0 and %d", run.MaxGnbPage)
		}
		q.Limit = *r.GnbLimit
	}
	if r.GnbOffset < 0 {
		return q, errors.New("gnbOffset must not be negative")
	}
	q.Offset = r.GnbOffset
	for _, s := range strings.Split(r.GnbState, ",") {
		switch st := run.GnbState(strings.TrimSpace(s)); st {
		case "":
		case run.GnbPending, run.GnbConnecting, run.GnbUp, run.GnbFailed, run.GnbLost, run.GnbClosed:
			q.States = append(q.States, st)
		default:
			return q, fmt.Errorf("gnbState: unknown state %q", st)
		}
	}
	return q, nil
}

func gnbPageQuery(c *gin.Context) (run.GnbPage, error) {
	var r GnbPageRequest
	var err error
	if v := c.Query("gnbOffset"); v != "" {
		if r.GnbOffset, err = strconv.Atoi(v); err != nil {
			return run.AllGnbs, errors.New("gnbOffset must be a number")
		}
	}
	if v := c.Query("gnbLimit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return run.AllGnbs, errors.New("gnbLimit must be a number")
		}
		r.GnbLimit = &n
	}
	r.GnbState = c.Query("gnbState")
	return r.page()
}

// handleStream pushes a run.Snapshot JSON text frame immediately, on every
// state change, and at least once per streamInterval, until the client
// goes away. The query picks the gNBs listed (GnbPageRequest); the client
// changes page by sending a GnbPageRequest, answered with a frame at once.
// A message that is not a valid request is ignored.
func handleStream(ctrl Controller) gin.HandlerFunc {
	return func(c *gin.Context) {
		first, err := gnbPageQuery(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, MessageResponse{Message: err.Error()})
			return
		}
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()

		var mu sync.Mutex
		page := first
		paged := make(chan struct{}, 1)
		gone := make(chan struct{})
		go func() { // reading also detects the client's close
			defer close(gone)
			for {
				_, msg, err := conn.ReadMessage()
				if err != nil {
					return
				}
				var r GnbPageRequest
				if json.Unmarshal(msg, &r) != nil {
					continue
				}
				q, err := r.page()
				if err != nil {
					continue
				}
				mu.Lock()
				page = q
				mu.Unlock()
				select {
				case paged <- struct{}{}:
				default:
				}
			}
		}()

		ticker := time.NewTicker(streamInterval)
		defer ticker.Stop()
		for {
			changed := ctrl.Changed()
			sent := time.Now()
			mu.Lock()
			q := page
			mu.Unlock()
			if err := conn.WriteJSON(ctrl.SnapshotPage(q)); err != nil {
				return
			}
			select {
			case <-gone:
				return
			case <-paged:
				continue // a new page goes out at once
			case <-changed:
			case <-ticker.C:
			}
			// coalesce bursts of changes into one frame per minFrameGap
			if wait := minFrameGap - time.Since(sent); wait > 0 {
				t := time.NewTimer(wait)
				select {
				case <-gone:
					t.Stop()
					return
				case <-t.C:
				}
			}
		}
	}
}
