package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/pathutil"
	"github.com/madcamp-official/26s-w3-c2-01/internal/scanner"
)

// AnalysisOrchestrator.Run (below) is the entire `libra scan` pipeline: it
// walks the filesystem, hands each entry to every registered
// ProjectDetector/ResourceDetector/DependencyAnalyzer (see
// analysis_contract.go for those interfaces), and persists whatever they
// find through the *Repository interfaces this file's siblings declare.
// cmd/scan.go is a thin wrapper that only supplies concrete detectors and
// prints the result -- none of the actual DISCOVER_FILES ->
// ... -> PERSIST_RESULTS phase sequence (§18.3 of
// docs/libra_integration_contracts.md) lives in cmd.
type ResourceObserver interface {
	Observe(context.Context, ResourceObservationInput) (ResourceObservation, error)
}

type AnalysisOptions struct {
	ScanID      string
	Scan        scanner.Options
	Environment Environment
}

type ScanResult struct {
	ScanID       string
	Filesystem   scanner.Result
	Projects     []domain.BuildProject
	Workspaces   []domain.Workspace
	Resources    []domain.Resource
	Dependencies []domain.Dependency
	Evidence     []domain.Evidence
	Issues       []Issue
	Unverified   []UnverifiedScope
	Phase        AnalysisPhase
	Status       string
}

type AnalysisOrchestrator struct {
	filesystem          scanner.Scanner
	scans               ScanRepository
	projects            ProjectRepository
	workspaces          WorkspaceRepository
	resources           ResourceObserver
	dependencies        DependencyRepository
	projectDetectors    []ProjectDetector
	resourceDetectors   []ResourceDetector
	dependencyAnalyzers []DependencyAnalyzer
	now                 func() time.Time
	onPhase             func(AnalysisPhase)
}

func NewAnalysisOrchestrator(
	filesystem scanner.Scanner,
	scans ScanRepository,
	projects ProjectRepository,
	workspaces WorkspaceRepository,
	resources ResourceObserver,
	dependencies DependencyRepository,
) *AnalysisOrchestrator {
	return &AnalysisOrchestrator{
		filesystem: filesystem, scans: scans, projects: projects, workspaces: workspaces,
		resources: resources, dependencies: dependencies, now: time.Now,
	}
}

func (o *AnalysisOrchestrator) WithDetectors(projects []ProjectDetector, resources []ResourceDetector, dependencies []DependencyAnalyzer) *AnalysisOrchestrator {
	o.projectDetectors = append([]ProjectDetector(nil), projects...)
	o.resourceDetectors = append([]ResourceDetector(nil), resources...)
	o.dependencyAnalyzers = append([]DependencyAnalyzer(nil), dependencies...)
	return o
}

func (o *AnalysisOrchestrator) Run(ctx context.Context, options AnalysisOptions) (ScanResult, error) {
	result := ScanResult{ScanID: options.ScanID, Status: ScanStatusRunning}
	startedAt := o.now()
	record := ScanRecord{ID: options.ScanID, StartedAt: startedAt, Roots: append([]string(nil), options.Scan.Roots...), Status: ScanStatusRunning}
	if err := o.scans.Save(ctx, record); err != nil {
		return result, err
	}

	var candidates []ProjectCandidate
	o.phase(&result, PhaseDiscoverFiles)
	filesystemResult, err := o.filesystem.Scan(ctx, options.Scan, func(ctx context.Context, entry scanner.Entry) error {
		for _, detector := range o.projectDetectors {
			detected := detector.Observe(ctx, entry)
			candidates = append(candidates, detected.Items...)
			result.Issues = append(result.Issues, detected.Issues...)
			result.Unverified = append(result.Unverified, detected.Unverified...)
		}
		return nil
	})
	result.Filesystem = filesystemResult
	for _, issue := range filesystemResult.Issues {
		result.Issues = append(result.Issues, Issue{
			Code: IssueAccessDenied, Phase: PhaseDiscoverFiles, Path: issue.Path,
			Operation: issue.Operation, Severity: IssueWarning, Message: issue.Err.Error(), Cause: issue.Err,
		})
	}
	if err != nil {
		return o.fail(ctx, result, record, err)
	}

	o.phase(&result, PhaseDiscoverProjects)
	observedAt := o.now()
	workspaceCandidates := make([]workspaceCandidate, 0)
	var projectResourceFacts []ResourceObservationInput
	for _, candidate := range candidates {
		for _, projectFact := range candidate.Projects {
			project, err := PrepareBuildProject(projectFact, observedAt)
			if err != nil {
				result.Issues = append(result.Issues, structuredCandidateIssue(projectFact.ManifestPath, "prepare project", err))
				continue
			}
			result.Projects = append(result.Projects, project)
		}
		for _, projectResource := range candidate.ProjectResources {
			projectResourceFacts = append(projectResourceFacts, ResourceObservationInput{
				Resource: projectResource.Resource,
				Cleanup:  projectResource.Cleanup,
			})
		}
		if candidate.Workspace != nil {
			workspace, err := PrepareWorkspace(*candidate.Workspace, observedAt)
			if err != nil {
				result.Issues = append(result.Issues, structuredCandidateIssue(candidate.Workspace.ManifestPath, "prepare workspace", err))
				continue
			}
			result.Workspaces = append(result.Workspaces, workspace)
			workspaceCandidates = append(workspaceCandidates, workspaceCandidate{workspace: workspace, projectPaths: candidate.WorkspaceProjectPaths})
		}
	}

	o.phase(&result, PhaseDiscoverSystemResources)
	for _, detector := range o.resourceDetectors {
		detected := detector.Detect(ctx, options.Environment)
		result.Issues = append(result.Issues, detected.Issues...)
		result.Unverified = append(result.Unverified, detected.Unverified...)
		facts := make([]ResourceObservationInput, 0, len(detected.Items))
		for _, resource := range detected.Items {
			facts = append(facts, ResourceObservationInput{Resource: resource})
		}
		if err := o.observeResourceFacts(ctx, &result, facts); err != nil {
			return o.fail(ctx, result, record, fmt.Errorf("observe resource: %w", err))
		}
	}
	// Project-scoped resources (Node's node_modules/dist/build) were collected
	// during project discovery; observe them the same way as system resources
	// so they are sized, risk-classified, and persisted. Linking each back to
	// its owning project (a dependency edge) is deferred to the Day 4 graph.
	if err := o.observeResourceFacts(ctx, &result, projectResourceFacts); err != nil {
		return o.fail(ctx, result, record, fmt.Errorf("observe project resource: %w", err))
	}

	o.phase(&result, PhaseAnalyzeProjectSettings)
	o.phase(&result, PhaseResolveDependencies)
	index := newMemoryResourceIndex(result.Resources)
	for _, project := range result.Projects {
		for _, analyzer := range o.dependencyAnalyzers {
			analyzed := analyzer.Analyze(ctx, project, index)
			result.Issues = append(result.Issues, analyzed.Issues...)
			result.Unverified = append(result.Unverified, analyzed.Unverified...)
			for _, bundle := range analyzed.Items {
				result.Dependencies = append(result.Dependencies, bundle.Dependency)
				result.Evidence = append(result.Evidence, bundle.Evidence...)
			}
		}
	}

	o.phase(&result, PhaseClassifyArtifacts)
	o.phase(&result, PhaseCalculateRisk)
	o.phase(&result, PhasePersistResults)
	if err := o.projects.UpsertObserved(ctx, options.ScanID, result.Projects); err != nil {
		return o.fail(ctx, result, record, fmt.Errorf("persist projects: %w", err))
	}
	for _, candidate := range workspaceCandidates {
		if err := o.workspaces.Upsert(ctx, options.ScanID, candidate.workspace); err != nil {
			return o.fail(ctx, result, record, fmt.Errorf("persist workspace: %w", err))
		}
		memberIDs, issues := resolveWorkspaceMembers(candidate.projectPaths, result.Projects)
		result.Issues = append(result.Issues, issues...)
		if err := o.workspaces.ReplaceMembers(ctx, candidate.workspace.ID, memberIDs); err != nil {
			return o.fail(ctx, result, record, fmt.Errorf("persist workspace members: %w", err))
		}
	}
	for _, dependency := range result.Dependencies {
		var evidence []domain.Evidence
		for _, item := range result.Evidence {
			if item.DependencyID == dependency.ID {
				evidence = append(evidence, item)
			}
		}
		if err := o.dependencies.UpsertGraph(ctx, options.ScanID, dependency, evidence); err != nil {
			return o.fail(ctx, result, record, fmt.Errorf("persist dependency graph: %w", err))
		}
	}

	o.phase(&result, PhaseCompleted)
	result.Status = ScanStatusCompleted
	if len(result.Issues) > 0 {
		result.Status = ScanStatusCompletedWithErrors
	}
	finishedAt := o.now()
	record.FinishedAt = &finishedAt
	record.FileCount = filesystemResult.FilesInspected
	record.ErrorCount = int64(len(result.Issues))
	record.Status = result.Status
	if err := o.scans.Save(context.WithoutCancel(ctx), record); err != nil {
		return result, err
	}
	return result, nil
}

// observeResourceFacts enriches, risk-classifies, and persists each detected
// resource fact through the ResourceObserver, appending the results and any
// recoverable measurement issues to result.
func (o *AnalysisOrchestrator) observeResourceFacts(ctx context.Context, result *ScanResult, facts []ResourceObservationInput) error {
	for _, resourceFact := range facts {
		observed, err := o.resources.Observe(ctx, resourceFact)
		if err != nil {
			return err
		}
		result.Resources = append(result.Resources, observed.Resource)
		for _, issue := range observed.Issues {
			result.Issues = append(result.Issues, Issue{
				Code: IssueAccessDenied, Phase: PhaseDiscoverSystemResources, Path: issue.Path,
				Operation: issue.Operation, Severity: IssueWarning, Message: issue.Err.Error(), Cause: issue.Err,
			})
		}
	}
	return nil
}

func (o *AnalysisOrchestrator) phase(result *ScanResult, phase AnalysisPhase) {
	result.Phase = phase
	if o.onPhase != nil {
		o.onPhase(phase)
	}
}

func (o *AnalysisOrchestrator) fail(ctx context.Context, result ScanResult, record ScanRecord, cause error) (ScanResult, error) {
	result.Status = ScanStatusFailed
	result.Issues = append(result.Issues, Issue{
		Code: IssueDatabaseWriteFailed, Phase: result.Phase, Operation: "run analysis",
		Severity: IssueError, Message: cause.Error(), Cause: cause,
	})
	finishedAt := o.now()
	record.FinishedAt = &finishedAt
	record.FileCount = result.Filesystem.FilesInspected
	record.ErrorCount = int64(len(result.Issues))
	record.Status = ScanStatusFailed
	return result, errors.Join(cause, o.scans.Save(context.WithoutCancel(ctx), record))
}

func structuredCandidateIssue(path, operation string, err error) Issue {
	return Issue{Code: IssueMalformedManifest, Phase: PhaseDiscoverProjects, Path: path,
		Operation: operation, Severity: IssueWarning, Message: err.Error(), Cause: err}
}

type workspaceCandidate struct {
	workspace    domain.Workspace
	projectPaths []string
}

func resolveWorkspaceMembers(paths []string, projects []domain.BuildProject) ([]string, []Issue) {
	byManifest := make(map[string]string, len(projects))
	for _, project := range projects {
		byManifest[project.NormalizedManifestPath] = project.ID
	}
	ids := make([]string, 0, len(paths))
	var issues []Issue
	for _, path := range paths {
		normalized, err := pathutil.Normalize(path)
		if err != nil {
			issues = append(issues, structuredCandidateIssue(path, "resolve workspace member", err))
			continue
		}
		id, exists := byManifest[normalized]
		if !exists {
			issues = append(issues, Issue{Code: IssueAdapterFailed, Phase: PhasePersistResults,
				Path: path, Operation: "resolve workspace member", Severity: IssueWarning,
				Message: "referenced project was not observed"})
			continue
		}
		ids = append(ids, id)
	}
	return ids, issues
}

type memoryResourceIndex map[domain.ResourceType]map[string][]domain.Resource

func newMemoryResourceIndex(resources []domain.Resource) memoryResourceIndex {
	index := make(memoryResourceIndex)
	for _, resource := range resources {
		byVersion := index[resource.Type]
		if byVersion == nil {
			byVersion = make(map[string][]domain.Resource)
			index[resource.Type] = byVersion
		}
		byVersion[resource.Version] = append(byVersion[resource.Version], resource)
	}
	return index
}

func (i memoryResourceIndex) Find(resourceType domain.ResourceType, version string) []domain.Resource {
	return append([]domain.Resource(nil), i[resourceType][version]...)
}
