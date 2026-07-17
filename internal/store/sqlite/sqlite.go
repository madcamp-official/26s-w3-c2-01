// Package sqlite provides the SQLite persistence foundation for Libra.
package sqlite

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// Open opens a SQLite database and configures the connection invariants used
// by Libra. The caller owns the returned database and must close it.
func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	db.SetMaxOpenConns(1)
	if err := configure(db); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func configure(db *sql.DB) error {
	for _, statement := range []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = 5000",
	} {
		if _, err := db.Exec(statement); err != nil {
			return fmt.Errorf("configure sqlite database: %w", err)
		}
	}

	return nil
}
