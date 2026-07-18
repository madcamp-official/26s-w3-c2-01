package sqlite

import "testing"

func TestMigrateCreatesContractTablesAndIsIdempotent(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("second Migrate() error = %v", err)
	}

	wantTables := []string{
		"cleanup_items", "cleanup_plans", "dependencies", "evidence",
		"projects", "resources", "scans", "schema_migrations", "transactions",
	}
	for _, table := range wantTables {
		var found string
		err := db.QueryRow(
			"SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?", table,
		).Scan(&found)
		if err != nil {
			t.Errorf("table %q was not created: %v", table, err)
		}
	}

	var migrationCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&migrationCount); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if migrationCount != 2 {
		t.Fatalf("migration count = %d, want 2", migrationCount)
	}

	var sizeKnownColumn string
	if err := db.QueryRow(`
		SELECT name FROM pragma_table_info('resources') WHERE name = 'size_known'
	`).Scan(&sizeKnownColumn); err != nil {
		t.Fatalf("resources.size_known was not created: %v", err)
	}
}
