// Package api is fru-tester's HTTP surface. fru-lab's backend is the only
// intended client: it authenticates users and forwards requests here with
// the shared API token.
package api

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"tester/profile"
	"tester/run"
)

// Controller is the slice of *run.Controller the handlers use.
type Controller interface {
	Validate(p profile.Profile) (*profile.Plan, error)
	Start(p profile.Profile) (run.Snapshot, error)
	Stop() (run.Snapshot, error)
	Snapshot() run.Snapshot
	Changed() <-chan struct{}
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

func NewRouter(ctrl Controller, apiToken string) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	g := r.Group("/api", requireToken(apiToken))
	g.POST("/profile/validate", handleValidate(ctrl))
	g.GET("/run", func(c *gin.Context) { c.JSON(http.StatusOK, ctrl.Snapshot()) })
	g.POST("/run", handleStart(ctrl))
	g.POST("/run/stop", handleStop(ctrl))
	g.GET("/run/stream", handleStream(ctrl))
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

func handleStart(ctrl Controller) gin.HandlerFunc {
	return func(c *gin.Context) {
		p, ok := bindProfile(c)
		if !ok {
			return
		}
		snap, err := ctrl.Start(p)
		var verr *profile.ValidationError
		switch {
		case err == nil:
			c.JSON(http.StatusAccepted, snap)
		case errors.As(err, &verr):
			c.JSON(http.StatusBadRequest, StartErrorResponse{Message: "profile is invalid", Errors: verr.Errors})
		case errors.Is(err, run.ErrRunActive):
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

// handleStream pushes a run.Snapshot JSON text frame immediately, on every
// state change, and at least once per streamInterval, until the client
// goes away.
func handleStream(ctrl Controller) gin.HandlerFunc {
	return func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()

		gone := make(chan struct{})
		go func() { // the client never sends; reading detects its close
			defer close(gone)
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					return
				}
			}
		}()

		ticker := time.NewTicker(streamInterval)
		defer ticker.Stop()
		for {
			changed := ctrl.Changed()
			sent := time.Now()
			if err := conn.WriteJSON(ctrl.Snapshot()); err != nil {
				return
			}
			select {
			case <-gone:
				return
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
