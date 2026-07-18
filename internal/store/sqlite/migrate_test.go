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
		"workspaces", "workspace_projects",
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
	if migrationCount != 5 {
		t.Fatalf("migration count = %d, want 5", migrationCount)
	}

	var sizeKnownColumn string
	if err := db.QueryRow(`
		SELECT name FROM pragma_table_info('resources') WHERE name = 'size_known'
	`).Scan(&sizeKnownColumn); err != nil {
		t.Fatalf("resources.size_known was not created: %v", err)
	}

	var scanIDColumn string
	if err := db.QueryRow(`
		SELECT name FROM pragma_table_info('evidence') WHERE name = 'scan_id'
	`).Scan(&scanIDColumn); err != nil {
		t.Fatalf("evidence.scan_id was not created: %v", err)
	}

	for _, index := range []string{"idx_dependencies_project_resources", "idx_dependencies_resource_projects"} {
		var found string
		if err := db.QueryRow("SELECT name FROM sqlite_master WHERE type = 'index' AND name = ?", index).Scan(&found); err != nil {
			t.Fatalf("dependency graph index %q was not created: %v", index, err)
		}
	}

	for _, column := range []string{"normalized_root_path", "manifest_path", "normalized_manifest_path", "last_observed_scan_id"} {
		var found string
		if err := db.QueryRow("SELECT name FROM pragma_table_info('projects') WHERE name = ?", column).Scan(&found); err != nil {
			t.Fatalf("projects.%s was not created: %v", column, err)
		}
	}
}

func TestEvidenceScanMigrationPreservesLegacyRows(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })

	for _, name := range []string{"migrations/001_initial.sql", "migrations/002_resource_size_known.sql"} {
		contents, err := migrationFiles.ReadFile(name)
		if err != nil {
			t.Fatalf("ReadFile(%q) error = %v", name, err)
		}
		if _, err := db.Exec(string(contents)); err != nil {
			t.Fatalf("apply %q error = %v", name, err)
		}
	}
	if _, err := db.Exec(`
		INSERT INTO dependencies (id, source_type, source_id, target_type, target_id, relation, confidence)
		VALUES ('dep-1', 'PROJECT', 'project-1', 'RESOURCE', 'resource-1', 'REQUIRES', 75);
		INSERT INTO evidence (id, dependency_id, evidence_type, source_path, collected_at)
		VALUES ('evidence-1', 'dep-1', 'DECLARED', 'project.vcxproj', '2026-07-18T00:00:00Z');
	`); err != nil {
		t.Fatalf("insert legacy evidence: %v", err)
	}

	contents, err := migrationFiles.ReadFile("migrations/003_evidence_scan.sql")
	if err != nil {
		t.Fatalf("ReadFile(003) error = %v", err)
	}
	if _, err := db.Exec(string(contents)); err != nil {
		t.Fatalf("apply evidence scan migration: %v", err)
	}

	var scanID string
	if err := db.QueryRow("SELECT scan_id FROM evidence WHERE id = 'evidence-1'").Scan(&scanID); err != nil {
		t.Fatalf("read migrated evidence: %v", err)
	}
	if scanID != "migration:003:legacy-evidence" {
		t.Fatalf("scan_id = %q, want legacy migration scan", scanID)
	}
}
