//go:build windows

// A multi-item, partially-failing clean run against real files, exercising
// the same CleanupService.Execute path `libra clean --execute` uses in
// production. quarantine_windows_edgecases_test.go (internal/safety) proves
// QuarantineEngine handles one locked file gracefully in isolation; this
// proves the service layer correctly rolls that up into a transaction
// status of PARTIALLY_QUARANTINED rather than losing the failure or
// crashing the whole batch, per docs/libra_cli_commands_and_schedule.md
// Day 6 "격리 도중 일부만 성공한 경우".
package app

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/pathutil"
	"github.com/madcamp-official/26s-w3-c2-01/internal/safety"
	"github.com/madcamp-official/26s-w3-c2-01/internal/scanner"
)

type multiResourceRepositoryFake struct{ byID map[string]domain.Resource }

func (*multiResourceRepositoryFake) Upsert(context.Context, domain.Resource) error { return nil }
func (r *multiResourceRepositoryFake) FindByID(_ context.Context, id string) (domain.Resource, error) {
	res, ok := r.byID[id]
	if !ok {
		return domain.Resource{}, os.ErrNotExist
	}
	return res, nil
}
func (*multiResourceRepositoryFake) ListByType(context.Context, domain.ResourceType) ([]domain.Resource, error) {
	return nil, nil
}
func (*multiResourceRepositoryFake) List(context.Context) ([]domain.Resource, error) { return nil, nil }

func TestCleanupServicePartiallyQuarantinesWhenOneItemIsLockedOpen(t *testing.T) {
	root := t.TempDir()
	projectRoot := filepath.Join(root, "project")

	healthyArtifact := filepath.Join(projectRoot, "dist")
	if err := os.MkdirAll(healthyArtifact, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(healthyArtifact, "bundle.js"), []byte("generated"), 0o644); err != nil {
		t.Fatal(err)
	}

	lockedArtifact := filepath.Join(projectRoot, "build")
	if err := os.MkdirAll(lockedArtifact, 0o755); err != nil {
		t.Fatal(err)
	}
	lockedFile := filepath.Join(lockedArtifact, "output.dll")
	if err := os.WriteFile(lockedFile, []byte("binary"), 0o644); err != nil {
		t.Fatal(err)
	}
	normalizedHealthy, _ := pathutil.Normalize(healthyArtifact)
	normalizedLocked, _ := pathutil.Normalize(lockedArtifact)
	normalizedRoot, _ := pathutil.Normalize(projectRoot)

	// Measure real on-disk facts before locking anything: CleanupValidator
	// re-verifies size/mtime against the plan snapshot immediately before
	// Move, exactly like `libra clean --execute` does, so the plan items
	// must carry the true current values or validation would reject them
	// before the lock is ever exercised.
	measuredHealthy, err := scanner.MeasureResource(context.Background(), scanner.New(1), healthyArtifact)
	if err != nil || !measuredHealthy.SizeKnown {
		t.Fatalf("measure healthy artifact: %#v, %v", measuredHealthy, err)
	}
	measuredLocked, err := scanner.MeasureResource(context.Background(), scanner.New(1), lockedArtifact)
	if err != nil || !measuredLocked.SizeKnown {
		t.Fatalf("measure locked artifact: %#v, %v", measuredLocked, err)
	}

	pathPtr, err := syscall.UTF16PtrFromString(lockedFile)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := syscall.CreateFile(pathPtr, syscall.GENERIC_READ|syscall.GENERIC_WRITE, 0, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatalf("lock %q: %v", lockedFile, err)
	}
	t.Cleanup(func() { _ = syscall.CloseHandle(handle) })

	healthyResource := domain.Resource{ID: "resource-healthy", Type: domain.ResourceTypeBuildOutput, NormalizedPath: normalizedHealthy, Regenerable: true, Risk: domain.RiskSafe}
	lockedResource := domain.Resource{ID: "resource-locked", Type: domain.ResourceTypeBuildOutput, NormalizedPath: normalizedLocked, Regenerable: true, Risk: domain.RiskSafe}

	plan := domain.CleanupPlan{ID: "plan-partial", Items: []domain.CleanupPlanItem{
		{ID: "plan-item-healthy", ResourceID: healthyResource.ID, NormalizedPath: normalizedHealthy, ExpectedType: healthyResource.Type, ExpectedSize: measuredHealthy.LogicalSize, ExpectedModifiedTime: *measuredHealthy.LastModifiedAt, RiskAtPlanning: domain.RiskSafe, OwnerProjectID: "project-1"},
		{ID: "plan-item-locked", ResourceID: lockedResource.ID, NormalizedPath: normalizedLocked, ExpectedType: lockedResource.Type, ExpectedSize: measuredLocked.LogicalSize, ExpectedModifiedTime: *measuredLocked.LastModifiedAt, RiskAtPlanning: domain.RiskSafe, OwnerProjectID: "project-1"},
	}}

	plans := &cleanupPlanRepositoryFake{plan: plan}
	resources := &multiResourceRepositoryFake{byID: map[string]domain.Resource{
		healthyResource.ID: healthyResource,
		lockedResource.ID:  lockedResource,
	}}
	projects := &cleanupProjectRepositoryFake{project: domain.BuildProject{ID: "project-1", NormalizedRootPath: normalizedRoot}}
	transactions := &cleanupTransactionRepositoryFake{}
	classifier, err := safety.NewPathClassifier(nil)
	if err != nil {
		t.Fatal(err)
	}
	engine := safety.QuarantineEngine{RootForPath: func(_, transactionID string) string { return filepath.Join(root, "quarantine", transactionID) }}
	service := NewCleanupService(plans, resources, projects, transactions, safety.CleanupValidator{Paths: classifier}, engine)
	service.now = func() time.Time { return time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC) }

	transaction, err := service.Execute(context.Background(), plan.ID)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if transaction.Status != domain.TransactionPartiallyQuarantined {
		t.Fatalf("transaction.Status = %s, want PARTIALLY_QUARANTINED", transaction.Status)
	}
	var moved, failed int
	for _, item := range transaction.Items {
		switch item.Status {
		case domain.TransactionItemMoved:
			moved++
		case domain.TransactionItemFailed:
			failed++
			if item.Reason == "" {
				t.Fatal("failed item must carry a reason")
			}
		}
	}
	if moved != 1 || failed != 1 {
		t.Fatalf("moved=%d failed=%d, want exactly one of each", moved, failed)
	}

	// The healthy artifact must actually be gone from disk...
	if _, err := os.Stat(healthyArtifact); !os.IsNotExist(err) {
		t.Fatalf("healthy artifact should have been quarantined: %v", err)
	}
	// ...and the locked one must be completely untouched, not half-moved.
	if _, err := os.Stat(lockedFile); err != nil {
		t.Fatalf("locked artifact must remain exactly where it was: %v", err)
	}
}
