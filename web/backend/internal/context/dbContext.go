package context

import (
	"slices"
	"sync"

	"backend/logger"
)

type dbContextIE struct {
	dbType string
	dbPath string

	*logger.BackendLogger
}

type dbContext struct {
	db DbIf

	// profileMu makes a profile's name check and its write one step.
	profileMu sync.Mutex
	// historyMu makes each change to the run history one step, so a
	// rename cannot bring back a run being deleted.
	historyMu sync.Mutex

	*logger.BackendLogger
}

func newDbContext(dbContextIE *dbContextIE) (*dbContext, error) {
	db, err := newDb(dbContextIE.dbType, dbContextIE.dbPath)
	if err != nil {
		return nil, err
	}

	return &dbContext{
		db: db,

		BackendLogger: dbContextIE.BackendLogger,
	}, nil
}

// testerHistoryBucket holds finished Throughput Tester runs (design N7),
// keyed by run ID; fru-tester's run IDs (20261002-150405) sort by time.
const testerHistoryBucket = "tester-history"

// testerHistoryDeletedBucket remembers the runs a user deleted: a tester
// keeps its last few runs, and fru-lab must not copy them back.
const testerHistoryDeletedBucket = "tester-history-deleted"

// testerHistoryDeletedKeep bounds the remembered deletions; the oldest go
// first, long after their tester stopped keeping them.
const testerHistoryDeletedKeep = 1000

// SaveTesterRun stores a finished run's report once: saved is false if
// runID is already stored. Only the newest keep runs are kept.
func (d *dbContext) SaveTesterRun(runID string, report []byte, keep int) (saved bool, err error) {
	d.historyMu.Lock()
	defer d.historyMu.Unlock()
	existing, err := d.db.Get(testerHistoryBucket, runID)
	if err != nil || existing != nil {
		return false, err
	}
	if deleted, err := d.db.Get(testerHistoryDeletedBucket, runID); err != nil || deleted != nil {
		return false, err
	}
	if err := d.db.Put(testerHistoryBucket, runID, report); err != nil {
		return false, err
	}
	runs, err := d.db.List(testerHistoryBucket)
	if err != nil {
		return true, err
	}
	for i := 0; i < len(runs)-keep; i++ {
		if err := d.db.Delete(testerHistoryBucket, runs[i].Key); err != nil {
			return true, err
		}
	}
	return true, nil
}

// ListTesterRuns returns the stored runs, newest first.
func (d *dbContext) ListTesterRuns() ([]KV, error) {
	runs, err := d.db.List(testerHistoryBucket)
	slices.Reverse(runs)
	return runs, err
}

// GetTesterRun returns one stored report, nil if there is none.
func (d *dbContext) GetTesterRun(runID string) ([]byte, error) {
	return d.db.Get(testerHistoryBucket, runID)
}

// TesterRunDeleted reports whether the user deleted runID.
func (d *dbContext) TesterRunDeleted(runID string) (bool, error) {
	v, err := d.db.Get(testerHistoryDeletedBucket, runID)
	return v != nil, err
}

// DeleteTesterRuns removes runs from the history and remembers them as
// deleted; deleting a run that is not stored is not an error.
func (d *dbContext) DeleteTesterRuns(runIDs []string) error {
	d.historyMu.Lock()
	defer d.historyMu.Unlock()
	for _, id := range runIDs {
		if err := d.db.Put(testerHistoryDeletedBucket, id, []byte{1}); err != nil {
			return err
		}
		if err := d.db.Delete(testerHistoryBucket, id); err != nil {
			return err
		}
	}
	gone, err := d.db.List(testerHistoryDeletedBucket)
	if err != nil {
		return err
	}
	for i := 0; i < len(gone)-testerHistoryDeletedKeep; i++ {
		if err := d.db.Delete(testerHistoryDeletedBucket, gone[i].Key); err != nil {
			return err
		}
	}
	return nil
}

// UpdateTesterRun replaces a stored run's report with what edit returns;
// found is false if runID is not stored.
func (d *dbContext) UpdateTesterRun(runID string, edit func([]byte) ([]byte, error)) (found bool, err error) {
	d.historyMu.Lock()
	defer d.historyMu.Unlock()
	raw, err := d.db.Get(testerHistoryBucket, runID)
	if err != nil || raw == nil {
		return false, err
	}
	out, err := edit(raw)
	if err != nil {
		return true, err
	}
	return true, d.db.Put(testerHistoryBucket, runID, out)
}

func (d *dbContext) release() {
	d.DbLog.Infoln("Release dbContext...")

	if err := d.db.Release(); err != nil {
		d.DbLog.Errorf("Failed to release dbContext: %v", err)
	}

	d.DbLog.Infoln("dbContext released")
}
