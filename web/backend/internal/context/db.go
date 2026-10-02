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
	// List returns every key and value of bucket in key order (empty if
	// the bucket does not exist yet).
	List(bucket string) ([]KV, error)
	// Delete removes key; a missing bucket or key is not an error.
	Delete(bucket, key string) error
	Release() error
}

type KV struct {
	Key   string
	Value []byte
}

func newDb(dbType, dbPath string) (DbIf, error) {
	switch dbType {
	case "bbolt":
		return newBboltDb(dbPath)
	}
	return nil, fmt.Errorf("unsupported db type: %s", dbType)
}
