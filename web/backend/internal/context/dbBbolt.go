package context

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.etcd.io/bbolt"
)

type bboltDb struct {
	db *bbolt.DB
}

func newBboltDb(dbPath string) (*bboltDb, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create DB directory: %v", err)
	}

	db, err := bbolt.Open(dbPath, 0600, &bbolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("failed to open DB: %v", err)
	}

	return &bboltDb{
		db: db,
	}, nil
}

func (b *bboltDb) Release() error {
	if b.db != nil {
		return b.db.Close()
	}
	return nil
}
