package boltz

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/timshannon/bolthold"
	bolt "go.etcd.io/bbolt"
)

// DB is an interface for a database. It is used for easier testing
type DB interface {
	Connect(dbPath string) error
	Get(key, result interface{}) error
	Insert(key, data interface{}) error
}

// BoltzDB is a wrapper around bolthold implementing DB interface
type BoltzDB struct {
	db *bolthold.Store
}

func (b *BoltzDB) Connect(dbPath string) error {
	if _, err := os.Stat(dbPath); errors.Is(err, os.ErrNotExist) {
		_, err := os.Create(dbPath)
		if err != nil {
			return err
		}
	}
	db, err := bolthold.Open(dbPath, 0666, &bolthold.Options{
		Options: &bolt.Options{Timeout: 3 * time.Second},
	})
	if err != nil {
		return fmt.Errorf("failed to open db. Check that `dbpath` path is valid and writable: %v", err)
	}
	b.db = db
	return nil
}

func (b *BoltzDB) Get(key, value interface{}) error {
	return b.db.Get(key, value)
}

func (b *BoltzDB) Insert(key, data interface{}) error {
	return b.db.Insert(key, data)
}
