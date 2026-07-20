package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

func TestCleanupTransactionRepositoryRoundTripAndUpdate(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	seedCleanupResources(t, db, "resource-1")
	now := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	plan := domain.CleanupPlan{ID: "plan-1", CreatedAt: now, TargetBytes: 1, SelectedBytes: 1, Status: domain.CleanupPlanReady, Items: []domain.CleanupPlanItem{{ID: "plan-item-1", ResourceID: "resource-1", NormalizedPath: `d:\repo\dist`, ExpectedType: domain.ResourceTypeBuildOutput, ExpectedSize: 1, ExpectedModifiedTime: now, RiskAtPlanning: domain.RiskSafe, ConfidenceAtPlanning: 90, ScanID: "scan-1"}}}
	if err := NewCleanupPlanRepository(db).Create(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	transaction := domain.CleanupTransaction{ID: "tx-1", PlanID: plan.ID, StartedAt: now, Status: domain.TransactionRunning, Items: []domain.CleanupTransactionItem{{ID: "tx-item-1", PlanItemID: "plan-item-1", ResourceID: "resource-1", OriginalPath: `d:\repo\dist`, QuarantinePath: `d:\.libra-quarantine\tx-1\items\dist`, ManifestPath: `d:\.libra-quarantine\tx-1\manifest.json`, ExpectedSize: 1, Status: domain.TransactionItemPending}}}
	repository := NewCleanupTransactionRepository(db)
	if err := repository.Create(context.Background(), transaction); err != nil {
		t.Fatal(err)
	}
	got, err := repository.FindByID(context.Background(), transaction.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.TransactionRunning || len(got.Items) != 1 || got.Items[0].Status != domain.TransactionItemPending {
		t.Fatalf("transaction = %#v", got)
	}
	finished := now.Add(time.Minute)
	got.Status = domain.TransactionQuarantined
	got.FinishedAt = &finished
	got.Items[0].Status = domain.TransactionItemMoved
	if err := repository.Update(context.Background(), got); err != nil {
		t.Fatal(err)
	}
	updated, err := repository.FindByID(context.Background(), got.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != domain.TransactionQuarantined || updated.Items[0].Status != domain.TransactionItemMoved || updated.FinishedAt == nil {
		t.Fatalf("updated = %#v", updated)
	}
}
