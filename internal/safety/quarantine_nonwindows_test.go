//go:build !windows

package safety

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

// TestQuarantineEngineDefaultRootStaysUnderOriginalParent locks in the
// placement guarantee documented in docs/libra_integration_contracts.md §11:
// on non-Windows, the default (no RootForPath override) quarantine root
// lives directly under the original item's own parent directory, never at
// a shared volume root. filepath.VolumeName is always "" on non-Windows, so
// without this guarantee a naive "one root per volume" layout would still
// technically be same-filesystem, but only by accident; this test exercises
// the actual production root() logic (both prior tests in this package
// override RootForPath and never touch it) and confirms os.Rename never has
// to cross a filesystem boundary regardless of which volume the original
// path lives on -- verified for real against a separate mounted APFS volume
// during manual testing (see docs/libra_integration_contracts.md §11).
func TestQuarantineEngineDefaultRootStaysUnderOriginalParent(t *testing.T) {
	root := t.TempDir()
	original := filepath.Join(root, "project", "node_modules")
	if err := os.MkdirAll(original, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(original, "pkg.js"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	engine := QuarantineEngine{}
	transaction := domain.CleanupTransaction{ID: "tx-parent", PlanID: "plan-parent", Items: []domain.CleanupTransactionItem{{ID: "item-parent", OriginalPath: original, Status: domain.TransactionItemPending}}}

	if err := engine.Prepare(&transaction); err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	wantRoot := filepath.Join(filepath.Dir(original), ".libra-quarantine", transaction.ID)
	if got := filepath.Dir(filepath.Dir(transaction.Items[0].QuarantinePath)); got != wantRoot {
		t.Fatalf("quarantine root = %q, want %q (under original's own parent, not a shared volume root)", got, wantRoot)
	}

	engine.Move(context.Background(), &transaction)
	if transaction.Items[0].Status != domain.TransactionItemMoved {
		t.Fatalf("move status = %s, reason = %s", transaction.Items[0].Status, transaction.Items[0].Reason)
	}
	engine.Restore(context.Background(), &transaction)
	if transaction.Items[0].Status != domain.TransactionItemRestored {
		t.Fatalf("restore status = %s, reason = %s", transaction.Items[0].Status, transaction.Items[0].Reason)
	}
	if _, err := os.Stat(filepath.Join(original, "pkg.js")); err != nil {
		t.Fatalf("restored file: %v", err)
	}
}
