package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/safety"
)

func TestPurgeServiceDryRunThenExecute(t *testing.T) {
	root := t.TempDir()
	itemPath := filepath.Join(root, "items", "item")
	if err := os.MkdirAll(itemPath, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(itemPath, "data"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(root, "manifest.json")
	finished := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	item := domain.CleanupTransactionItem{ID: "item-1", QuarantinePath: itemPath, ManifestPath: manifestPath, Status: domain.TransactionItemMoved}
	transaction := domain.CleanupTransaction{ID: "tx-1", PlanID: "plan-1", StartedAt: finished.Add(-time.Minute), FinishedAt: &finished, Status: domain.TransactionQuarantined, Items: []domain.CleanupTransactionItem{item}}
	data, _ := json.Marshal(safety.Manifest{SchemaVersion: safety.ManifestVersion, TransactionID: transaction.ID, PlanID: transaction.PlanID, Items: transaction.Items})
	if err := os.WriteFile(manifestPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	repository := &purgeTransactionRepositoryStub{transaction: transaction}
	service := NewPurgeService(repository)
	service.now = func() time.Time { return finished.Add(10 * 24 * time.Hour) }

	preview, err := service.Purge(context.Background(), transaction.ID, 7, false)
	if err != nil {
		t.Fatal(err)
	}
	if !preview.DryRun || len(preview.Candidates) != 1 {
		t.Fatalf("preview = %#v", preview)
	}
	if _, err := os.Stat(itemPath); err != nil {
		t.Fatalf("dry run changed item: %v", err)
	}

	result, err := service.Purge(context.Background(), transaction.ID, 7, true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Transaction.Status != domain.TransactionPurged || repository.updated.Status != domain.TransactionPurged {
		t.Fatalf("result = %#v", result)
	}
	if _, err := os.Stat(itemPath); !os.IsNotExist(err) {
		t.Fatalf("purged item still exists: %v", err)
	}
}

func TestPurgeServiceRejectsRetentionAndManifestEscape(t *testing.T) {
	now := time.Now().UTC()
	repository := &purgeTransactionRepositoryStub{transaction: domain.CleanupTransaction{ID: "tx", FinishedAt: &now, Status: domain.TransactionQuarantined}}
	service := NewPurgeService(repository)
	service.now = func() time.Time { return now }
	if _, err := service.Purge(context.Background(), "tx", 7, true); err == nil {
		t.Fatal("expected retention error")
	}
}

type purgeTransactionRepositoryStub struct{ transaction, updated domain.CleanupTransaction }

func (s *purgeTransactionRepositoryStub) Create(context.Context, domain.CleanupTransaction) error {
	return nil
}
func (s *purgeTransactionRepositoryStub) Update(_ context.Context, transaction domain.CleanupTransaction) error {
	s.updated = transaction
	s.transaction = transaction
	return nil
}
func (s *purgeTransactionRepositoryStub) FindByID(context.Context, string) (domain.CleanupTransaction, error) {
	return s.transaction, nil
}
func (s *purgeTransactionRepositoryStub) List(context.Context) ([]domain.CleanupTransaction, error) {
	return []domain.CleanupTransaction{s.transaction}, nil
}
