package index

import (
	"errors"
	bolt "go.etcd.io/bbolt"
	"os"
	"path/filepath"
	"time"
)

type Store struct {
	db *bolt.DB
}

type OpenOptions struct {
	Path string // e.g. "./data/index.db"
}

func Open(opt OpenOptions) (*Store, error) {
	if opt.Path == "" {
		return nil, errors.New("index: missing path")
	}
	if err := os.MkdirAll(filepath.Dir(opt.Path), 0o755); err != nil {
		return nil, err
	}
	db, err := bolt.Open(opt.Path, 0o600, &bolt.Options{
		Timeout: 1 * time.Second,
	})
	if err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}
