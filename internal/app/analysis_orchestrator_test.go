package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/pathutil"
	"github.com/madcamp-official/26s-w3-c2-01/internal/scanner"
)

func TestAnalysisOrchestratorRunsPipelineInContractOrder(t *testing.T) {
	root := t.TempDir()
	manifest := filepath.Join(root, "package.json")
	if err := os.WriteFile(manifest, []byte(`{"name":"app"}`), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	resourceRoot := filepath.Join(root, "node_modules")
	if err := os.Mkdir(resourceRoot, 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}

	scans := &scanRepositoryCapture{}
	projects := &projectRepositoryCapture{}
	workspaces := &workspaceRepositoryCapture{}
	dependencies := &dependencyRepositoryCapture{}
	resourceObserver := resourceObserverFake{observedAt: time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC)}
	projectDetector := projectDetectorFake{root: root}
	resourceDetector := resourceDetectorFake{path: resourceRoot}
	analyzer := dependencyAnalyzerFake{}
	orchestrator := NewAnalysisOrchestrator(scanner.New(2), scans, projects, workspaces, resourceObserver, dependencies).
		WithDetectors([]ProjectDetector{projectDetector}, []ResourceDetector{resourceDetector}, []DependencyAnalyzer{analyzer})
	orchestrator.now = func() time.Time { return time.Date(2026, 7, 18, 11, 0, 0, 0, time.UTC) }
	var phases []AnalysisPhase
	orchestrator.onPhase = func(phase AnalysisPhase) { phases = append(phases, phase) }

	result, err := orchestrator.Run(context.Background(), AnalysisOptions{
		ScanID: "scan-orchestrated", Scan: scanner.Options{Roots: []string{root}, MaxDepth: 4},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != ScanStatusCompleted || result.Phase != PhaseCompleted {
		t.Fatalf("Run() status/phase = %q/%q", result.Status, result.Phase)
	}
	if len(result.Projects) != 1 || len(result.Resources) != 1 || len(result.Dependencies) != 1 || len(result.Evidence) != 1 {
		t.Fatalf("Run() result = %#v", result)
	}
	if len(projects.saved) != 1 || len(dependencies.saved) != 1 {
		t.Fatalf("persisted projects/dependencies = %d/%d", len(projects.saved), len(dependencies.saved))
	}
	wantPhases := []AnalysisPhase{
		PhaseDiscoverFiles, PhaseDiscoverProjects, PhaseDiscoverSystemResources,
		PhaseAnalyzeProjectSettings, PhaseResolveDependencies, PhaseClassifyArtifacts,
		PhaseCalculateRisk, PhasePersistResults, PhaseCompleted,
	}
	if len(phases) != len(wantPhases) {
		t.Fatalf("phases = %v, want %v", phases, wantPhases)
	}
	for i := range wantPhases {
		if phases[i] != wantPhases[i] {
			t.Fatalf("phases = %v, want %v", phases, wantPhases)
		}
	}
	if scans.records[len(scans.records)-1].Status != ScanStatusCompleted {
		t.Fatalf("final scan = %#v", scans.records[len(scans.records)-1])
	}
}

func TestAnalysisOrchestratorMarksPersistenceFailure(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	scans := &scanRepositoryCapture{}
	projects := &projectRepositoryCapture{err: errors.New("database unavailable")}
	orchestrator := NewAnalysisOrchestrator(scanner.New(1), scans, projects,
		&workspaceRepositoryCapture{}, resourceObserverFake{}, &dependencyRepositoryCapture{}).
		WithDetectors([]ProjectDetector{projectDetectorFake{root: root}}, nil, nil)

	result, err := orchestrator.Run(context.Background(), AnalysisOptions{
		ScanID: "scan-failed", Scan: scanner.Options{Roots: []string{root}, MaxDepth: 2},
	})
	if err == nil || result.Status != ScanStatusFailed {
		t.Fatalf("Run() result/error = %#v/%v, want FAILED", result, err)
	}
	if scans.records[len(scans.records)-1].Status != ScanStatusFailed {
		t.Fatalf("final scan = %#v", scans.records[len(scans.records)-1])
	}
}

func TestAnalysisOrchestratorObservesProjectOwnedResources(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	resourcePath := filepath.Join(root, "node_modules")
	if err := os.Mkdir(resourcePath, 0o755); err != nil {
		t.Fatal(err)
	}

	scans := &scanRepositoryCapture{}
	orchestrator := NewAnalysisOrchestrator(scanner.New(1), scans, &projectRepositoryCapture{},
		&workspaceRepositoryCapture{}, resourceObserverFake{observedAt: time.Now()}, &dependencyRepositoryCapture{}).
		WithDetectors([]ProjectDetector{projectResourceDetectorFake{root: root, resourcePath: resourcePath}}, nil, nil)

	result, err := orchestrator.Run(context.Background(), AnalysisOptions{
		ScanID: "scan-project-resources", Scan: scanner.Options{Roots: []string{root}, MaxDepth: 2},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	// The project's own node_modules is observed and persisted as a Resource,
	// even with no system resource detectors and no dependency edge.
	if len(result.Projects) != 1 || len(result.Resources) != 1 {
		t.Fatalf("Run() projects/resources = %d/%d, want 1/1", len(result.Projects), len(result.Resources))
	}
	if len(result.Dependencies) != 0 {
		t.Fatalf("Run() dependencies = %d, want 0 (edges deferred to Day 4)", len(result.Dependencies))
	}
	if result.Resources[0].Type != domain.ResourceTypeNodeModules {
		t.Fatalf("observed resource type = %q, want node-modules", result.Resources[0].Type)
	}
}

// projectResourceDetectorFake reports one project plus its own node_modules
// as a ProjectResourceCandidate, exercising the project-owned-resource path.
type projectResourceDetectorFake struct{ root, resourcePath string }

func (d projectResourceDetectorFake) Observe(_ context.Context, entry scanner.Entry) DetectionResult[ProjectCandidate] {
	if filepath.Base(entry.Path) != "package.json" {
		return DetectionResult[ProjectCandidate]{}
	}
	return DetectionResult[ProjectCandidate]{Items: []ProjectCandidate{{
		Projects: []domain.BuildProject{{
			Name: "app", Type: domain.ProjectTypeNode, RootPath: d.root,
			ManifestPath: entry.Path, LastModifiedAt: entry.ModifiedAt,
		}},
		ProjectResources: []ProjectResourceCandidate{{
			OwnerManifestPath: entry.Path,
			Resource: domain.Resource{
				Name: "node_modules", Type: domain.ResourceTypeNodeModules, DisplayPath: d.resourcePath, Confidence: 60,
			},
		}},
	}}}
}

type projectDetectorFake struct{ root string }

func (d projectDetectorFake) Observe(_ context.Context, entry scanner.Entry) DetectionResult[ProjectCandidate] {
	if filepath.Base(entry.Path) != "package.json" {
		return DetectionResult[ProjectCandidate]{}
	}
	return DetectionResult[ProjectCandidate]{Items: []ProjectCandidate{{Projects: []domain.BuildProject{{
		Name: "app", Type: domain.ProjectTypeNode, RootPath: d.root,
		ManifestPath: entry.Path, LastModifiedAt: entry.ModifiedAt,
	}}}}}
}

type resourceDetectorFake struct{ path string }

func (d resourceDetectorFake) Detect(context.Context, Environment) DetectionResult[domain.Resource] {
	return DetectionResult[domain.Resource]{Items: []domain.Resource{{
		Name: "node_modules", Type: domain.ResourceTypeNodeModules, DisplayPath: d.path, Confidence: 60,
	}}}
}

type resourceObserverFake struct{ observedAt time.Time }

func (f resourceObserverFake) Observe(_ context.Context, resource domain.Resource) (ResourceObservation, error) {
	display, err := pathutil.Absolute(resource.DisplayPath)
	if err != nil {
		return ResourceObservation{}, err
	}
	normalized, err := pathutil.Normalize(display)
	if err != nil {
		return ResourceObservation{}, err
	}
	resource.DisplayPath = display
	resource.NormalizedPath = normalized
	resource.ID = domain.ResourceID(resource.Type, resource.Version, normalized)
	resource.SizeKnown = true
	resource.Risk = domain.RiskReview
	resource.LastObservedAt = f.observedAt
	return ResourceObservation{Resource: resource}, nil
}

type dependencyAnalyzerFake struct{}

func (dependencyAnalyzerFake) Analyze(_ context.Context, input ProjectAnalysisInput, resources ResourceIndex) DetectionResult[DependencyBundle] {
	project := input.Project
	resource := resources.Find(domain.ResourceTypeNodeModules, "")[0]
	dependency := domain.Dependency{SourceType: domain.NodeProject, SourceID: project.ID,
		TargetType: domain.NodeResource, TargetID: resource.ID, Relation: domain.RelationRequires, Confidence: 60}
	dependency.ID = domain.DependencyID(dependency.SourceType, dependency.SourceID, dependency.Relation, dependency.TargetType, dependency.TargetID)
	evidence := domain.Evidence{DependencyID: dependency.ID, Kind: domain.EvidenceDeclared,
		SourcePath: project.ManifestPath, CollectedAt: project.LastObservedAt}
	evidence.ID = domain.EvidenceID(evidence.DependencyID, evidence.Kind, evidence.SourcePath, "", "", "")
	return DetectionResult[DependencyBundle]{Items: []DependencyBundle{{Dependency: dependency, Evidence: []domain.Evidence{evidence}}}}
}

func TestMemoryResourceIndexListsEveryVersionOfType(t *testing.T) {
	windowsA := domain.Resource{ID: "win-a", Type: domain.ResourceTypeWindowsSDK, Version: "10.0.1"}
	windowsB := domain.Resource{ID: "win-b", Type: domain.ResourceTypeWindowsSDK, Version: "10.0.2"}
	dotnet := domain.Resource{ID: "dotnet", Type: domain.ResourceTypeDotNetSDK, Version: "8.0.1"}
	index := newMemoryResourceIndex([]domain.Resource{windowsA, dotnet, windowsB})

	got := index.ListByType(domain.ResourceTypeWindowsSDK)
	if len(got) != 2 {
		t.Fatalf("ListByType() = %#v, want two Windows SDK resources", got)
	}
	seen := map[string]bool{}
	for _, resource := range got {
		seen[resource.ID] = true
	}
	if !seen[windowsA.ID] || !seen[windowsB.ID] || seen[dotnet.ID] {
		t.Fatalf("ListByType() IDs = %#v", seen)
	}

	for i := range got {
		got[i].ID = "mutated"
	}
	for _, resource := range index.ListByType(domain.ResourceTypeWindowsSDK) {
		if resource.ID == "mutated" {
			t.Fatal("ListByType() returned storage owned by the index")
		}
	}
}

type scanRepositoryCapture struct{ records []ScanRecord }

func (r *scanRepositoryCapture) Save(_ context.Context, record ScanRecord) error {
	r.records = append(r.records, record)
	return nil
}

type projectRepositoryCapture struct {
	saved []domain.BuildProject
	err   error
}

func (r *projectRepositoryCapture) UpsertObserved(_ context.Context, _ string, projects []domain.BuildProject) error {
	if r.err != nil {
		return r.err
	}
	r.saved = append(r.saved, projects...)
	return nil
}
func (*projectRepositoryCapture) FindByID(context.Context, string) (domain.BuildProject, error) {
	return domain.BuildProject{}, errors.New("not implemented")
}
func (*projectRepositoryCapture) FindByManifestPath(context.Context, domain.ProjectType, string) (domain.BuildProject, error) {
	return domain.BuildProject{}, errors.New("not implemented")
}
func (*projectRepositoryCapture) List(context.Context) ([]domain.BuildProject, error) {
	return nil, errors.New("not implemented")
}

type workspaceRepositoryCapture struct{}

func (*workspaceRepositoryCapture) Upsert(context.Context, string, domain.Workspace) error {
	return nil
}
func (*workspaceRepositoryCapture) ReplaceMembers(context.Context, string, []string) error {
	return nil
}

type dependencyRepositoryCapture struct{ saved []domain.Dependency }

func (r *dependencyRepositoryCapture) UpsertGraph(_ context.Context, _ string, dependency domain.Dependency, _ []domain.Evidence) error {
	r.saved = append(r.saved, dependency)
	return nil
}
func (*dependencyRepositoryCapture) FindResourcesByProject(context.Context, string) ([]domain.Dependency, error) {
	return nil, errors.New("not implemented")
}
func (*dependencyRepositoryCapture) FindProjectsByResource(context.Context, string) ([]domain.Dependency, error) {
	return nil, errors.New("not implemented")
}
func (*dependencyRepositoryCapture) FindEvidence(context.Context, string) ([]domain.Evidence, error) {
	return nil, errors.New("not implemented")
}
