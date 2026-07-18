package sqlite

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

func TestDependencyGraphQueriesUseCoveringIndexes(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	tests := []struct {
		name      string
		query     string
		args      []any
		wantIndex string
	}{
		{
			name: "project to resources",
			query: `SELECT id FROM dependencies
				WHERE source_type = ? AND source_id = ? AND target_type = ?
				ORDER BY relation, target_id`,
			args:      []any{domain.NodeProject, "project-1", domain.NodeResource},
			wantIndex: "idx_dependencies_project_resources",
		},
		{
			name: "resource to projects",
			query: `SELECT id FROM dependencies
				WHERE source_type = ? AND target_type = ? AND target_id = ?
				ORDER BY relation, source_id`,
			args:      []any{domain.NodeProject, domain.NodeResource, "resource-1"},
			wantIndex: "idx_dependencies_resource_projects",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows, err := db.Query("EXPLAIN QUERY PLAN "+tt.query, tt.args...)
			if err != nil {
				t.Fatalf("EXPLAIN QUERY PLAN error = %v", err)
			}
			defer rows.Close()

			var plan strings.Builder
			for rows.Next() {
				var id, parent, unused int
				var detail string
				if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
					t.Fatalf("Scan(query plan) error = %v", err)
				}
				plan.WriteString(detail)
				plan.WriteByte('\n')
			}
			if !strings.Contains(plan.String(), tt.wantIndex) {
				t.Fatalf("query plan = %q, want index %q", plan.String(), tt.wantIndex)
			}
		})
	}
}

func BenchmarkDependencyRepositoryReverseLookup(b *testing.B) {
	db, err := Open(":memory:")
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()
	if err := Migrate(db); err != nil {
		b.Fatal(err)
	}
	tx, err := db.Begin()
	if err != nil {
		b.Fatal(err)
	}
	statement, err := tx.Prepare(`
		INSERT INTO dependencies (id, source_type, source_id, target_type, target_id, relation, confidence)
		VALUES (?, 'PROJECT', ?, 'RESOURCE', ?, 'REQUIRES', 75)
	`)
	if err != nil {
		b.Fatal(err)
	}
	for i := range 10_000 {
		resourceID := fmt.Sprintf("resource-%04d", i%100)
		if _, err := statement.Exec(fmt.Sprintf("dependency-%05d", i), fmt.Sprintf("project-%05d", i), resourceID); err != nil {
			b.Fatal(err)
		}
	}
	statement.Close()
	if err := tx.Commit(); err != nil {
		b.Fatal(err)
	}

	repository := NewDependencyRepository(db)
	b.ResetTimer()
	for range b.N {
		if _, err := repository.FindProjectsByResource(context.Background(), "resource-0042"); err != nil {
			b.Fatal(err)
		}
	}
}
