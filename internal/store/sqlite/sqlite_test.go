package sqlite

import (
	"path/filepath"
	"testing"
)

func TestOpenCreatesDatabaseAndEnablesForeignKeys(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "libra.db")

	db, err := Open(databasePath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })

	var foreignKeys int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		t.Fatalf("query foreign_keys: %v", err)
	}
	if foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d, want 1", foreignKeys)
	}

	if _, err := db.Exec("CREATE TABLE healthcheck (id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatalf("create table: %v", err)
	}
}
