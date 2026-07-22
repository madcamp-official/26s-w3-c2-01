package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	projectmarkeradapter "github.com/madcamp-official/26s-w3-c2-01/internal/adapter/projectmarker"
	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/safety"
	"github.com/madcamp-official/26s-w3-c2-01/internal/scanner"
)

func TestEcosystemCleanupPipelineScanPlanExecuteRestore(t *testing.T) {
	root := t.TempDir()
	manifest := filepath.Join(root, "pom.xml")
	target := filepath.Join(root, "target")
	if err := os.WriteFile(manifest, []byte("<project/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "app.jar"), []byte("generated"), 0o644); err != nil {
		t.Fatal(err)
	}

	// scan: the real ecosystem detector emits a project-owned target resource.
	detected := (EcosystemProjectDetector{Detector: projectmarkeradapter.Detector{}}).Observe(
		context.Background(), scanner.Entry{Path: manifest, Kind: scanner.EntryFile})
	if len(detected.Items) != 1 || len(detected.Items[0].ProjectResources) != 1 {
		t.Fatalf("detected = %#v", detected)
	}
	project, err := PrepareBuildProject(detected.Items[0].Projects[0], time.Now())
	if err != nil {
		t.Fatal(err)
	}
	candidate := detected.Items[0].ProjectResources[0]

	// resources: ResourceService measures, classifies, and stores the candidate.
	repository := &pipelineResourceRepository{}
	classifier, err := safety.NewPathClassifier(nil)
	if err != nil {
		t.Fatal(err)
	}
	observation, err := NewResourceService(scanner.New(1), repository, classifier, DefaultRiskPolicy{}).Observe(
		context.Background(), ResourceObservationInput{Resource: candidate.Resource, Cleanup: candidate.Cleanup, ProjectScoped: true})
	if err != nil {
		t.Fatal(err)
	}
	if observation.Resource.Risk != domain.RiskSafe {
		t.Fatalf("resource = %#v, want SAFE", observation.Resource)
	}

	// plan: the explicit OWNS relation makes the SAFE resource selectable.
	scan := ScanRecord{ID: "ecosystem-scan", StartedAt: time.Now(), Roots: []string{root}, Status: ScanStatusCompleted}
	dependencies := &planDependencyRepositoryStub{owners: map[string]string{observation.Resource.ID: project.ID}}
	planner := NewPlanService(repository, &planProjectRepositoryStub{projects: []domain.BuildProject{project}}, &planScanRepositoryStub{record: scan}, dependencies)
	planned, err := planner.Build(context.Background(), PlanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(planned.Plan.Items) != 1 {
		t.Fatalf("plan = %#v, want one target item", planned.Plan)
	}

	// clean --execute -> restore, using an isolated quarantine root.
	transactions := &cleanupTransactionRepositoryFake{}
	service := NewCleanupService(&cleanupPlanRepositoryFake{plan: planned.Plan}, &cleanupResourceRepositoryFake{resource: observation.Resource},
		&cleanupProjectRepositoryFake{project: project}, transactions, safety.CleanupValidator{Paths: classifier},
		safety.QuarantineEngine{RootForPath: func(_, transactionID string) string { return filepath.Join(t.TempDir(), transactionID) }})
	transaction, err := service.Execute(context.Background(), planned.Plan.ID)
	if err != nil || transaction.Status != domain.TransactionQuarantined {
		t.Fatalf("Execute() = %#v, %v", transaction, err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("target still exists: %v", err)
	}
	restored, err := service.Restore(context.Background(), transaction.ID)
	if err != nil || restored.Status != domain.TransactionRestored {
		t.Fatalf("Restore() = %#v, %v", restored, err)
	}
	if _, err := os.Stat(filepath.Join(target, "app.jar")); err != nil {
		t.Fatalf("restored target: %v", err)
	}
}

type pipelineResourceRepository struct{ resources []domain.Resource }

func (r *pipelineResourceRepository) Upsert(_ context.Context, resource domain.Resource) error {
	for index := range r.resources {
		if r.resources[index].ID == resource.ID {
			r.resources[index] = resource
			return nil
		}
	}
	r.resources = append(r.resources, resource)
	return nil
}
func (r *pipelineResourceRepository) FindByID(_ context.Context, id string) (domain.Resource, error) {
	for _, resource := range r.resources {
		if resource.ID == id {
			return resource, nil
		}
	}
	return domain.Resource{}, errNotImplemented
}
func (r *pipelineResourceRepository) ListByType(_ context.Context, resourceType domain.ResourceType) ([]domain.Resource, error) {
	var resources []domain.Resource
	for _, resource := range r.resources {
		if resource.Type == resourceType {
			resources = append(resources, resource)
		}
	}
	return resources, nil
}
func (r *pipelineResourceRepository) List(context.Context) ([]domain.Resource, error) {
	return r.resources, nil
}
