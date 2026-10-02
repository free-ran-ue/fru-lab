package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
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
// fru-lab's DB. fru-tester only keeps its latest run, so fru-lab polls it
// and stores every run it has not seen yet.
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

// poll stores fru-tester's finished run if it is new. No finished run
// (404) is not an error.
func (w *testerHistoryWatcher) poll(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(w.url, "/")+"/api/run/report", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+w.token)
	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("report: %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return err
	}
	var head struct {
		Snapshot struct {
			RunID string `json:"runId"`
		} `json:"snapshot"`
	}
	if err := json.Unmarshal(body, &head); err != nil || head.Snapshot.RunID == "" {
		return errors.New("report without a run id")
	}
	saved, err := w.store.SaveTesterRun(head.Snapshot.RunID, body, testerHistoryKeep)
	if err != nil {
		return err
	}
	if saved {
		w.log.Infof("Saved tester run %s to the history", head.Snapshot.RunID)
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
			Ul     storedDirection `json:"ul"`
			Dl     storedDirection `json:"dl"`
			Series []struct {
				T, UlTxBps, UlRxBps, DlTxBps, DlRxBps float64
			} `json:"series"`
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
// behind the JWT middleware): the list, the JSON report and the CSV time
// series of each stored run.
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
	group.GET("/tester/history/:runId/series.csv", func(c *gin.Context) {
		raw, ok := storedRun(c, store)
		if !ok {
			return
		}
		var r storedReport
		if err := json.Unmarshal(raw, &r); err != nil {
			c.JSON(http.StatusInternalServerError, model.ResponseTesterAction{Message: "Stored run is not readable"})
			return
		}
		var b strings.Builder
		b.WriteString("t,ulTxBps,ulRxBps,dlTxBps,dlRxBps\n")
		f := func(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
		for _, p := range r.Snapshot.Dataplane.Series {
			b.WriteString(strings.Join([]string{f(p.T), f(p.UlTxBps), f(p.UlRxBps), f(p.DlTxBps), f(p.DlRxBps)}, ",") + "\n")
		}
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="tester-%s.csv"`, c.Param("runId")))
		c.Data(http.StatusOK, "text/csv; charset=utf-8", []byte(b.String()))
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
