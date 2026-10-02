package context

import "fmt"

var (
	DefaultDbPath = "/tmp/frulab_default.db"
)

var (
	DbTypeList = []string{
		"bbolt",
	}
)

type DbIf interface {
	// Get returns nil, nil when bucket or key does not exist yet.
	Get(bucket, key string) ([]byte, error)
	Put(bucket, key string, value []byte) error
	Release() error
}

func newDb(dbType, dbPath string) (DbIf, error) {
	switch dbType {
	case "bbolt":
		return newBboltDb(dbPath)
	}
	return nil, fmt.Errorf("unsupported db type: %s", dbType)
}