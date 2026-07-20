package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

func TestCleanupPlanRepositoryCreateAndFindSnapshot(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	seedCleanupResources(t, db, "resource-1")

	now := time.Date(2026, 7, 20, 12, 0, 0, 123, time.UTC)
	plan := domain.CleanupPlan{
		ID: "plan-1", CreatedAt: now, TargetBytes: 100, SelectedBytes: 80,
		Status: domain.CleanupPlanInsufficientCandidates,
		Items: []domain.CleanupPlanItem{{
			ID: "item-1", ResourceID: "resource-1", NormalizedPath: `d:\repo\bin`,
			ExpectedType: domain.ResourceTypeBuildOutput, ExpectedSize: 80,
			ExpectedModifiedTime: now, RiskAtPlanning: domain.RiskSafe,
			ConfidenceAtPlanning: 90, OwnerProjectID: "project-1", ScanID: "scan-1",
			RegenerationCommand: "dotnet build",
		}},
	}
	repository := NewCleanupPlanRepository(db)
	if err := repository.Create(context.Background(), plan); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	got, err := repository.FindByID(context.Background(), plan.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if got.ID != plan.ID || got.Status != plan.Status || got.SelectedBytes != 80 || len(got.Items) != 1 {
		t.Fatalf("FindByID() = %#v", got)
	}
	if got.Items[0].OwnerProjectID != "project-1" || got.Items[0].RegenerationCommand != "dotnet build" || !got.Items[0].ExpectedModifiedTime.Equal(now) {
		t.Fatalf("item snapshot = %#v", got.Items[0])
	}
}

func TestCleanupPlanRepositoryRollsBackInvalidItemInsert(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	seedCleanupResources(t, db, "resource-1", "resource-2")
	now := time.Now().UTC()
	plan := domain.CleanupPlan{
		ID: "plan-rollback", CreatedAt: now, TargetBytes: 1, SelectedBytes: 2,
		Status: domain.CleanupPlanReady,
		Items: []domain.CleanupPlanItem{
			{ID: "same", ResourceID: "resource-1", NormalizedPath: "a", ExpectedType: domain.ResourceTypeBuildOutput, ExpectedSize: 1, ExpectedModifiedTime: now, RiskAtPlanning: domain.RiskSafe, ConfidenceAtPlanning: 90, ScanID: "scan-1"},
			{ID: "same", ResourceID: "resource-2", NormalizedPath: "b", ExpectedType: domain.ResourceTypeBuildOutput, ExpectedSize: 1, ExpectedModifiedTime: now, RiskAtPlanning: domain.RiskSafe, ConfidenceAtPlanning: 90, ScanID: "scan-1"},
		},
	}
	repository := NewCleanupPlanRepository(db)
	if err := repository.Create(context.Background(), plan); err == nil {
		t.Fatal("Create() error = nil, want duplicate item failure")
	}
	if _, err := repository.FindByID(context.Background(), plan.ID); !errors.Is(err, ErrCleanupPlanNotFound) {
		t.Fatalf("FindByID() error = %v, want ErrCleanupPlanNotFound", err)
	}
}

func seedCleanupResources(t *testing.T, db interface {
	Exec(string, ...any) (sql.Result, error)
}, ids ...string) {
	t.Helper()
	for _, id := range ids {
		_, err := db.Exec(`
			INSERT INTO resources (
				id, resource_type, name, path, normalized_path,
				last_observed_at, risk, confidence
			) VALUES (?, 'build-output', ?, ?, ?, '2026-07-20T00:00:00Z', 'SAFE', 90)
		`, id, id, id, id)
		if err != nil {
			t.Fatalf("seed resource %q: %v", id, err)
		}
	}
}
