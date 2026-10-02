package context

import "backend/logger"

type dbContextIE struct {
	dbType string
	dbPath string

	*logger.BackendLogger
}

type dbContext struct {
	db DbIf

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

const (
	testerBucket     = "tester"
	testerProfileKey = "profile"
)

// GetTesterProfile returns the saved Throughput Tester profile JSON, or
// nil if none was saved yet. fru-lab stores it opaquely; fru-tester owns
// its schema and validation.
func (d *dbContext) GetTesterProfile() ([]byte, error) {
	return d.db.Get(testerBucket, testerProfileKey)
}

func (d *dbContext) PutTesterProfile(profile []byte) error {
	return d.db.Put(testerBucket, testerProfileKey, profile)
}

func (d *dbContext) release() {
	d.DbLog.Infoln("Release dbContext...")

	if err := d.db.Release(); err != nil {
		d.DbLog.Errorf("Failed to release dbContext: %v", err)
	}

	d.DbLog.Infoln("dbContext released")
}
