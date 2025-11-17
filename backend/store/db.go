package store

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	db *sql.DB
}

// NewSQLiteStore opens the database file
func NewSQLiteStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	s := &Store{db: db}

	// Run migrations immediately on startup
	if err := s.migrate(); err != nil {
		return nil, err
	}

	return s, nil
}

// Close allows main.go to close the connection
func (s *Store) Close() error {
	return s.db.Close()
}
