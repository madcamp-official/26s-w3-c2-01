package safety

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

func TestQuarantineEngineMovesAndRestoresDirectoryWithManifest(t *testing.T) {
	root := t.TempDir()
	original := filepath.Join(root, "project", "dist")
	if err := os.MkdirAll(original, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(original, "bundle.js"), []byte("output"), 0o644); err != nil {
		t.Fatal(err)
	}
	engine := QuarantineEngine{RootForPath: func(_, transactionID string) string { return filepath.Join(root, "quarantine", transactionID) }}
	transaction := domain.CleanupTransaction{ID: "tx-1", PlanID: "plan-1", Items: []domain.CleanupTransactionItem{{ID: "item-1", OriginalPath: original, Status: domain.TransactionItemPending}}}

	if err := engine.Prepare(&transaction); err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	manifestBytes, err := os.ReadFile(transaction.Items[0].ManifestPath)
	if err != nil {
		t.Fatalf("manifest before move: %v", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	if manifest.SchemaVersion != ManifestVersion || manifest.Items[0].Status != domain.TransactionItemPending {
		t.Fatalf("manifest = %#v", manifest)
	}

	engine.Move(context.Background(), &transaction)
	if transaction.Items[0].Status != domain.TransactionItemMoved {
		t.Fatalf("move status = %s", transaction.Items[0].Status)
	}
	if _, err := os.Stat(original); !os.IsNotExist(err) {
		t.Fatalf("original still exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(transaction.Items[0].QuarantinePath, "bundle.js")); err != nil {
		t.Fatalf("quarantined file: %v", err)
	}

	engine.Restore(context.Background(), &transaction)
	if transaction.Items[0].Status != domain.TransactionItemRestored {
		t.Fatalf("restore status = %s", transaction.Items[0].Status)
	}
	if _, err := os.Stat(filepath.Join(original, "bundle.js")); err != nil {
		t.Fatalf("restored file: %v", err)
	}
}

func TestQuarantineEngineRestoreNeverOverwritesExistingPath(t *testing.T) {
	root := t.TempDir()
	original := filepath.Join(root, "project", "bin")
	if err := os.MkdirAll(original, 0o755); err != nil {
		t.Fatal(err)
	}
	engine := QuarantineEngine{RootForPath: func(_, transactionID string) string { return filepath.Join(root, "quarantine", transactionID) }}
	transaction := domain.CleanupTransaction{ID: "tx-2", PlanID: "plan-2", Items: []domain.CleanupTransactionItem{{ID: "item-2", OriginalPath: original, Status: domain.TransactionItemPending}}}
	if err := engine.Prepare(&transaction); err != nil {
		t.Fatal(err)
	}
	engine.Move(context.Background(), &transaction)
	if err := os.MkdirAll(original, 0o755); err != nil {
		t.Fatal(err)
	}
	engine.Restore(context.Background(), &transaction)
	if transaction.Items[0].Status != domain.TransactionItemSkipped {
		t.Fatalf("status = %s, want SKIPPED", transaction.Items[0].Status)
	}
	if _, err := os.Stat(transaction.Items[0].QuarantinePath); err != nil {
		t.Fatalf("quarantine was lost: %v", err)
	}
}
