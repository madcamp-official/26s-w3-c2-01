package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/pathutil"
	"github.com/madcamp-official/26s-w3-c2-01/internal/safety"
	"github.com/madcamp-official/26s-w3-c2-01/internal/scanner"
)

func TestCleanupServiceExecutesAndRestoresVerifiedPlan(t *testing.T) {
	root := t.TempDir()
	projectRoot := filepath.Join(root, "project")
	artifact := filepath.Join(projectRoot, "dist")
	if err := os.MkdirAll(artifact, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(artifact, "bundle.js"), []byte("generated"), 0o644); err != nil {
		t.Fatal(err)
	}
	normalizedArtifact, _ := pathutil.Normalize(artifact)
	normalizedRoot, _ := pathutil.Normalize(projectRoot)
	measured, err := scanner.MeasureResource(context.Background(), scanner.New(1), artifact)
	if err != nil || !measured.SizeKnown || measured.LastModifiedAt == nil {
		t.Fatalf("measure = %#v, %v", measured, err)
	}
	resource := domain.Resource{ID: "resource-1", Type: domain.ResourceTypeBuildOutput, NormalizedPath: normalizedArtifact, Regenerable: true, Risk: domain.RiskSafe}
	plan := domain.CleanupPlan{ID: "plan-1", Items: []domain.CleanupPlanItem{{ID: "plan-item-1", ResourceID: resource.ID, NormalizedPath: normalizedArtifact, ExpectedType: resource.Type, ExpectedSize: measured.LogicalSize, ExpectedModifiedTime: *measured.LastModifiedAt, RiskAtPlanning: domain.RiskSafe, OwnerProjectID: "project-1"}}}
	plans := &cleanupPlanRepositoryFake{plan: plan}
	resources := &cleanupResourceRepositoryFake{resource: resource}
	projects := &cleanupProjectRepositoryFake{project: domain.BuildProject{ID: "project-1", NormalizedRootPath: normalizedRoot}}
	transactions := &cleanupTransactionRepositoryFake{}
	classifier, err := safety.NewPathClassifier(nil)
	if err != nil {
		t.Fatal(err)
	}
	engine := safety.QuarantineEngine{RootForPath: func(_, transactionID string) string { return filepath.Join(root, "quarantine", transactionID) }}
	service := NewCleanupService(plans, resources, projects, transactions, safety.CleanupValidator{Paths: classifier}, engine)
	service.now = func() time.Time { return time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC) }

	transaction, err := service.Execute(context.Background(), plan.ID)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if transaction.Status != domain.TransactionQuarantined || transaction.Items[0].Status != domain.TransactionItemMoved {
		t.Fatalf("transaction = %#v", transaction)
	}
	if _, err := os.Stat(artifact); !os.IsNotExist(err) {
		t.Fatalf("artifact still exists: %v", err)
	}

	restored, err := service.Restore(context.Background(), transaction.ID)
	if err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	if restored.Status != domain.TransactionRestored || restored.Items[0].Status != domain.TransactionItemRestored {
		t.Fatalf("restored = %#v", restored)
	}
	if _, err := os.Stat(filepath.Join(artifact, "bundle.js")); err != nil {
		t.Fatalf("restored artifact: %v", err)
	}
}

func TestEcosystemTargetPlanExecutesAndRestores(t *testing.T) {
	root := t.TempDir()
	projectRoot := filepath.Join(root, "maven-project")
	artifact := filepath.Join(projectRoot, "target")
	if err := os.MkdirAll(artifact, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "pom.xml"), []byte("<project/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(artifact, "app.jar"), []byte("generated"), 0o644); err != nil {
		t.Fatal(err)
	}
	normalizedArtifact, _ := pathutil.Normalize(artifact)
	normalizedRoot, _ := pathutil.Normalize(projectRoot)
	measured, err := scanner.MeasureResource(context.Background(), scanner.New(1), artifact)
	if err != nil || !measured.SizeKnown || measured.LastModifiedAt == nil {
		t.Fatalf("measure = %#v, %v", measured, err)
	}
	resource := domain.Resource{ID: "maven-target", Type: domain.ResourceTypeBuildOutput,
		NormalizedPath: normalizedArtifact, Regenerable: true, Risk: domain.RiskSafe}
	plan := domain.CleanupPlan{ID: "maven-plan", Items: []domain.CleanupPlanItem{{
		ID: "maven-item", ResourceID: resource.ID, NormalizedPath: normalizedArtifact,
		ExpectedType: resource.Type, ExpectedSize: measured.LogicalSize,
		ExpectedModifiedTime: *measured.LastModifiedAt, RiskAtPlanning: domain.RiskSafe, OwnerProjectID: "maven-project",
	}}}
	classifier, err := safety.NewPathClassifier(nil)
	if err != nil {
		t.Fatal(err)
	}
	service := NewCleanupService(&cleanupPlanRepositoryFake{plan: plan}, &cleanupResourceRepositoryFake{resource: resource},
		&cleanupProjectRepositoryFake{project: domain.BuildProject{ID: "maven-project", NormalizedRootPath: normalizedRoot}},
		&cleanupTransactionRepositoryFake{}, safety.CleanupValidator{Paths: classifier},
		safety.QuarantineEngine{RootForPath: func(_, transactionID string) string { return filepath.Join(root, "quarantine", transactionID) }})
	transaction, err := service.Execute(context.Background(), plan.ID)
	if err != nil || transaction.Status != domain.TransactionQuarantined {
		t.Fatalf("Execute() = %#v, %v", transaction, err)
	}
	if _, err := os.Stat(artifact); !os.IsNotExist(err) {
		t.Fatalf("artifact still exists: %v", err)
	}
	restored, err := service.Restore(context.Background(), transaction.ID)
	if err != nil || restored.Status != domain.TransactionRestored {
		t.Fatalf("Restore() = %#v, %v", restored, err)
	}
	if _, err := os.Stat(filepath.Join(artifact, "app.jar")); err != nil {
		t.Fatalf("restored artifact: %v", err)
	}
}

type cleanupPlanRepositoryFake struct{ plan domain.CleanupPlan }

func (*cleanupPlanRepositoryFake) Create(context.Context, domain.CleanupPlan) error { return nil }
func (r *cleanupPlanRepositoryFake) FindByID(context.Context, string) (domain.CleanupPlan, error) {
	return r.plan, nil
}

type cleanupResourceRepositoryFake struct{ resource domain.Resource }

func (*cleanupResourceRepositoryFake) Upsert(context.Context, domain.Resource) error { return nil }
func (r *cleanupResourceRepositoryFake) FindByID(context.Context, string) (domain.Resource, error) {
	return r.resource, nil
}
func (*cleanupResourceRepositoryFake) ListByType(context.Context, domain.ResourceType) ([]domain.Resource, error) {
	return nil, nil
}
func (*cleanupResourceRepositoryFake) List(context.Context) ([]domain.Resource, error) {
	return nil, nil
}

type cleanupProjectRepositoryFake struct{ project domain.BuildProject }

func (*cleanupProjectRepositoryFake) UpsertObserved(context.Context, string, []domain.BuildProject) error {
	return nil
}
func (r *cleanupProjectRepositoryFake) FindByID(context.Context, string) (domain.BuildProject, error) {
	return r.project, nil
}
func (*cleanupProjectRepositoryFake) FindByManifestPath(context.Context, domain.ProjectType, string) (domain.BuildProject, error) {
	return domain.BuildProject{}, nil
}
func (*cleanupProjectRepositoryFake) List(context.Context) ([]domain.BuildProject, error) {
	return nil, nil
}

type cleanupTransactionRepositoryFake struct{ transaction domain.CleanupTransaction }

func (r *cleanupTransactionRepositoryFake) Create(_ context.Context, transaction domain.CleanupTransaction) error {
	r.transaction = transaction
	return nil
}
func (r *cleanupTransactionRepositoryFake) Update(_ context.Context, transaction domain.CleanupTransaction) error {
	r.transaction = transaction
	return nil
}
func (r *cleanupTransactionRepositoryFake) FindByID(context.Context, string) (domain.CleanupTransaction, error) {
	return r.transaction, nil
}
func (*cleanupTransactionRepositoryFake) List(context.Context) ([]domain.CleanupTransaction, error) {
	return nil, nil
}
