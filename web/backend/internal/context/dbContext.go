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

// SaveTesterRun stores a finished run's report once: saved is false if
// runID is already stored. Only the newest keep runs are kept.
func (d *dbContext) SaveTesterRun(runID string, report []byte, keep int) (saved bool, err error) {
	existing, err := d.db.Get(testerHistoryBucket, runID)
	if err != nil || existing != nil {
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

func (d *dbContext) release() {
	d.DbLog.Infoln("Release dbContext...")

	if err := d.db.Release(); err != nil {
		d.DbLog.Errorf("Failed to release dbContext: %v", err)
	}

	d.DbLog.Infoln("dbContext released")
}
