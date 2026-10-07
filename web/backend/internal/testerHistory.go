package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	loggergoModel "github.com/Alonza0314/logger-go/v2/model"
	"github.com/gin-gonic/gin"

	flctx "backend/internal/context"
	"backend/model"
)

// testerHistoryKeep is how many finished runs fru-lab keeps (design N7).
const testerHistoryKeep = 50

// testerHistoryPoll is how often fru-lab asks fru-tester for a finished run.
const testerHistoryPoll = 3 * time.Second

// testerRunStore is the slice of fru-lab's DB the history uses.
type testerRunStore interface {
	SaveTesterRun(runID string, report []byte, keep int) (bool, error)
	ListTesterRuns() ([]flctx.KV, error)
	GetTesterRun(runID string) ([]byte, error)
}

// testerHistoryWatcher copies each finished run from fru-tester into
// fru-lab's DB. fru-tester keeps its last few finished runs; fru-lab polls
// them and stores every one it has not seen yet.
type testerHistoryWatcher struct {
	url, token string
	client     *http.Client
	store      testerRunStore
	log        loggergoModel.LoggerInterface
}

func (w *testerHistoryWatcher) run(ctx context.Context) {
	t := time.NewTicker(testerHistoryPoll)
	defer t.Stop()
	for {
		if err := w.poll(ctx); err != nil && ctx.Err() == nil {
			w.log.Debugf("tester history: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// poll stores fru-tester's finished runs that are new.
func (w *testerHistoryWatcher) poll(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(w.url, "/")+"/api/run/reports", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+w.token)
	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("reports: %s", resp.Status)
	}
	var reports []json.RawMessage
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<20)).Decode(&reports); err != nil {
		return fmt.Errorf("reports: %w", err)
	}
	for _, raw := range reports {
		var head struct {
			Snapshot struct {
				RunID string `json:"runId"`
			} `json:"snapshot"`
		}
		if err := json.Unmarshal(raw, &head); err != nil || head.Snapshot.RunID == "" {
			return errors.New("report without a run id")
		}
		saved, err := w.store.SaveTesterRun(head.Snapshot.RunID, raw, testerHistoryKeep)
		if err != nil {
			return err
		}
		if saved {
			w.log.Infof("Saved tester run %s to the history", head.Snapshot.RunID)
		}
	}
	return nil
}

// testerHistorySummary is one row of the History page, read from a
// stored report.
type testerHistorySummary struct {
	RunID             string     `json:"runId"`
	ProfileName       string     `json:"profileName"`
	State             string     `json:"state"`
	Error             string     `json:"error"`
	StopReason        string     `json:"stopReason"`
	StartedAt         *time.Time `json:"startedAt"`
	StoppedAt         *time.Time `json:"stoppedAt"`
	GnbCount          int        `json:"gnbCount"`
	UeCount           int        `json:"ueCount"`
	Registered        int64      `json:"registered"`
	RegistrationP95Ms float64    `json:"registrationP95Ms"`
	Established       int64      `json:"established"`
	Deregistered      int64      `json:"deregistered"`
	UlTxBytes         uint64     `json:"ulTxBytes"`
	UlRxBytes         uint64     `json:"ulRxBytes"`
	UlLossRate        float64    `json:"ulLossRate"`
	DlTxBytes         uint64     `json:"dlTxBytes"`
	DlRxBytes         uint64     `json:"dlRxBytes"`
	DlLossRate        float64    `json:"dlLossRate"`
}

type storedStage struct {
	Expected int     `json:"expected"`
	Accepted int64   `json:"accepted"`
	P95Ms    float64 `json:"p95Ms"`
}

type storedDirection struct {
	TxBytes  uint64  `json:"txBytes"`
	RxBytes  uint64  `json:"rxBytes"`
	LossRate float64 `json:"lossRate"`
}

// storedReport is the part of fru-tester's report the history reads.
type storedReport struct {
	Snapshot struct {
		RunID          string      `json:"runId"`
		ProfileName    string      `json:"profileName"`
		State          string      `json:"state"`
		Error          string      `json:"error"`
		StopReason     string      `json:"stopReason"`
		StartedAt      *time.Time  `json:"startedAt"`
		StoppedAt      *time.Time  `json:"stoppedAt"`
		N2             storedStage `json:"n2"`
		Registration   storedStage `json:"registration"`
		Pdu            storedStage `json:"pdu"`
		Deregistration storedStage `json:"deregistration"`
		Dataplane      struct {
			Ul storedDirection `json:"ul"`
			Dl storedDirection `json:"dl"`
		} `json:"dataplane"`
	} `json:"snapshot"`
}

func summarize(raw []byte) (testerHistorySummary, error) {
	var r storedReport
	if err := json.Unmarshal(raw, &r); err != nil {
		return testerHistorySummary{}, err
	}
	s, dp := r.Snapshot, r.Snapshot.Dataplane
	return testerHistorySummary{
		RunID: s.RunID, ProfileName: s.ProfileName, State: s.State, Error: s.Error, StopReason: s.StopReason,
		StartedAt: s.StartedAt, StoppedAt: s.StoppedAt, GnbCount: s.N2.Expected, UeCount: s.Registration.Expected,
		Registered: s.Registration.Accepted, RegistrationP95Ms: s.Registration.P95Ms,
		Established: s.Pdu.Accepted, Deregistered: s.Deregistration.Accepted,
		UlTxBytes: dp.Ul.TxBytes, UlRxBytes: dp.Ul.RxBytes, UlLossRate: dp.Ul.LossRate,
		DlTxBytes: dp.Dl.TxBytes, DlRxBytes: dp.Dl.RxBytes, DlLossRate: dp.Dl.LossRate,
	}, nil
}

// addTesterHistoryRoutes serves the History page under group (which is
// behind the JWT middleware): the list, and each stored run's JSON report
// and HTML report.
func addTesterHistoryRoutes(group *gin.RouterGroup, store testerRunStore) {
	group.GET("/tester/history", func(c *gin.Context) {
		runs, err := store.ListTesterRuns()
		if err != nil {
			c.JSON(http.StatusInternalServerError, model.ResponseTesterAction{Message: "Failed to read the run history"})
			return
		}
		out := []testerHistorySummary{}
		for _, kv := range runs {
			if s, err := summarize(kv.Value); err == nil {
				out = append(out, s)
			}
		}
		c.JSON(http.StatusOK, out)
	})
	group.GET("/tester/history/:runId", func(c *gin.Context) {
		raw, ok := storedRun(c, store)
		if !ok {
			return
		}
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="tester-%s.json"`, c.Param("runId")))
		c.Data(http.StatusOK, "application/json", raw)
	})
	group.GET("/tester/history/:runId/report.html", func(c *gin.Context) {
		raw, ok := storedRun(c, store)
		if !ok {
			return
		}
		page, err := renderTesterReport(raw, time.Now())
		if err != nil {
			c.JSON(http.StatusInternalServerError, model.ResponseTesterAction{Message: "Stored run is not readable"})
			return
		}
		c.Header("Content-Disposition", fmt.Sprintf(`inline; filename="tester-%s.html"`, c.Param("runId")))
		c.Data(http.StatusOK, "text/html; charset=utf-8", page)
	})
}

func storedRun(c *gin.Context, store testerRunStore) ([]byte, bool) {
	raw, err := store.GetTesterRun(c.Param("runId"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.ResponseTesterAction{Message: "Failed to read the run history"})
		return nil, false
	}
	if raw == nil {
		c.JSON(http.StatusNotFound, model.ResponseTesterAction{Message: "No such run in the history"})
		return nil, false
	}
	return raw, true
}
