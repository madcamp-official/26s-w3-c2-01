package app

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
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
	// ReclassifyRequired re-classifies an already-persisted resource as
	// BLOCKED because dependency resolution (which runs after every
	// resource's first Observe) found a project that requires it.
	ReclassifyRequired(ctx context.Context, resourceID string) (ResourceObservation, error)
}

type AnalysisOptions struct {
	ScanID      string
	Scan        scanner.Options
	Environment Environment
}

// ScanProgress reports how much of the scan has been done so far. It is
// cumulative for the running scan, not a delta. FilesInspected/Directories
// are live during PhaseDiscoverFiles; Projects/Resources are zero until
// PhaseDiscoverProjects/PhaseDiscoverSystemResources finish counting them
// (see the WithPhaseHook callback, which reports them as soon as each is
// known rather than waiting for the whole scan to finish).
type ScanProgress struct {
	FilesInspected int64
	Directories    int64
	Projects       int64
	Resources      int64
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
	issues              ScanIssueRepository
	projectDetectors    []ProjectDetector
	resourceDetectors   []ResourceDetector
	dependencyAnalyzers []DependencyAnalyzer
	now                 func() time.Time
	onPhase             func(AnalysisPhase, ScanProgress)
	onProgress          func(ScanProgress)
}

func (o *AnalysisOrchestrator) WithIssueRepository(issues ScanIssueRepository) *AnalysisOrchestrator {
	o.issues = issues
	return o
}

// WithProgress registers a callback invoked after every filesystem entry
// PhaseDiscoverFiles visits, so a caller (e.g. a terminal progress bar) can
// show live progress without waiting for Run to return. The callback runs
// synchronously on the scanning goroutine, so it must be cheap and must not
// call back into the orchestrator.
func (o *AnalysisOrchestrator) WithProgress(fn func(ScanProgress)) *AnalysisOrchestrator {
	o.onProgress = fn
	return o
}

// WithPhaseHook registers a callback invoked at the start of every pipeline
// phase (see AnalysisPhase and §18.3 of docs/libra_integration_contracts.md),
// so a caller can show which phase a running scan is currently in. Unlike
// WithProgress this fires only once per phase -- a handful of times per
// scan -- so the callback does not need to throttle itself. The ScanProgress
// passed alongside is a snapshot as of that phase starting: Projects is
// final starting with PhaseDiscoverSystemResources, Resources starting with
// PhaseAnalyzeProjectSettings; both are zero before that.
func (o *AnalysisOrchestrator) WithPhaseHook(fn func(AnalysisPhase, ScanProgress)) *AnalysisOrchestrator {
	o.onPhase = fn
	return o
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
	var progress ScanProgress
	o.phase(&result, PhaseDiscoverFiles, progress)
	filesystemResult, err := o.filesystem.Scan(ctx, options.Scan, func(ctx context.Context, entry scanner.Entry) error {
		switch entry.Kind {
		case scanner.EntryFile, scanner.EntryOther:
			progress.FilesInspected++
		case scanner.EntryDirectory:
			progress.Directories++
		}
		if o.onProgress != nil {
			o.onProgress(progress)
		}
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

	o.phase(&result, PhaseDiscoverProjects, progress)
	observedAt := o.now()
	workspaceCandidates := make([]workspaceCandidate, 0)
	var projectResourceCandidates []ProjectResourceCandidate
	propertiesByManifest := make(map[string][]ProjectProperty)
	for _, candidate := range candidates {
		for _, projectFact := range candidate.Projects {
			project, err := PrepareBuildProject(projectFact, observedAt)
			if err != nil {
				result.Issues = append(result.Issues, structuredCandidateIssue(projectFact.ManifestPath, "prepare project", err))
				continue
			}
			measured, err := scanner.MeasureResource(ctx, o.filesystem, project.RootPath)
			if err != nil {
				result.Issues = append(result.Issues, structuredCandidateIssue(project.RootPath, "measure project size", err))
			} else {
				project.LogicalSize = measured.LogicalSize
				project.SizeKnown = measured.SizeKnown
			}
			result.Projects = append(result.Projects, project)
		}
		projectResourceCandidates = append(projectResourceCandidates, candidate.ProjectResources...)
		for _, property := range candidate.ProjectProperties {
			normalizedManifest, err := pathutil.Normalize(property.OwnerManifestPath)
			if err != nil {
				result.Issues = append(result.Issues, structuredCandidateIssue(
					property.OwnerManifestPath, "resolve project property owner", err))
				continue
			}
			propertiesByManifest[normalizedManifest] = append(propertiesByManifest[normalizedManifest], property)
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
	workspaceCandidates = topLevelWorkspaceCandidates(workspaceCandidates)
	result.Workspaces = workspacesFromCandidates(workspaceCandidates)
	result.Projects = dedupeProjects(result.Projects)
	result.Projects = filterNodeProjectsByWorkspace(result.Projects, workspaceCandidates)
	projectResourceCandidates = filterProjectResourcesByOwner(projectResourceCandidates, result.Projects)
	projectResourceCandidates = dedupeProjectResources(projectResourceCandidates)
	propertiesByManifest = filterProjectPropertiesByOwner(propertiesByManifest, result.Projects)

	// result.Projects is final now, so the projects count a phase hook
	// observer sees from here on is the real one, not a work-in-progress tally.
	progress.Projects = int64(len(result.Projects))
	o.phase(&result, PhaseDiscoverSystemResources, progress)
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
	// Project-scoped resources are observed, then linked to their owner with
	// an explicit OWNS edge below.
	projectResourceFacts := make([]ResourceObservationInput, 0, len(projectResourceCandidates))
	for _, candidate := range projectResourceCandidates {
		projectResourceFacts = append(projectResourceFacts, ResourceObservationInput{Resource: candidate.Resource, Cleanup: candidate.Cleanup})
	}
	if err := o.observeResourceFacts(ctx, &result, projectResourceFacts); err != nil {
		return o.fail(ctx, result, record, fmt.Errorf("observe project resource: %w", err))
	}
	for _, candidate := range projectResourceCandidates {
		owner, found := projectByManifest(result.Projects, candidate.OwnerManifestPath)
		if !found {
			result.Issues = append(result.Issues, structuredCandidateIssue(candidate.OwnerManifestPath, "resolve resource owner", errors.New("owner project not found")))
			continue
		}
		normalizedResource, err := pathutil.Normalize(candidate.Resource.DisplayPath)
		if err != nil {
			result.Issues = append(result.Issues, structuredCandidateIssue(candidate.Resource.DisplayPath, "normalize owned resource", err))
			continue
		}
		var resourceID string
		for _, resource := range result.Resources {
			if resource.NormalizedPath == normalizedResource {
				resourceID = resource.ID
				break
			}
		}
		if resourceID == "" {
			continue
		}
		dependency := domain.Dependency{SourceType: domain.NodeProject, SourceID: owner.ID, TargetType: domain.NodeResource, TargetID: resourceID, Relation: domain.RelationOwns, Confidence: domain.DefaultConfidence[domain.EvidenceObserved]}
		dependency.ID = domain.DependencyID(dependency.SourceType, dependency.SourceID, dependency.Relation, dependency.TargetType, dependency.TargetID)
		evidence := domain.Evidence{DependencyID: dependency.ID, Kind: domain.EvidenceObserved, SourcePath: candidate.OwnerManifestPath, Property: "project-output", ResolvedValue: normalizedResource, CollectedAt: observedAt}
		evidence.ID = domain.EvidenceID(evidence.DependencyID, evidence.Kind, evidence.SourcePath, evidence.Property, evidence.RawValue, evidence.ResolvedValue)
		result.Dependencies = append(result.Dependencies, dependency)
		result.Evidence = append(result.Evidence, evidence)
	}

	// result.Resources is final now (system resources plus project-owned
	// ones observed above), so this is the real resources count.
	progress.Resources = int64(len(result.Resources))
	o.phase(&result, PhaseAnalyzeProjectSettings, progress)
	o.phase(&result, PhaseResolveDependencies, progress)
	index := newMemoryResourceIndex(result.Resources)
	for _, project := range result.Projects {
		input := ProjectAnalysisInput{
			Project: project,
			Properties: append([]ProjectProperty(nil),
				propertiesByManifest[project.NormalizedManifestPath]...),
		}
		for _, analyzer := range o.dependencyAnalyzers {
			analyzed := analyzer.Analyze(ctx, input, index)
			result.Issues = append(result.Issues, analyzed.Issues...)
			result.Unverified = append(result.Unverified, analyzed.Unverified...)
			for _, bundle := range analyzed.Items {
				result.Dependencies = append(result.Dependencies, bundle.Dependency)
				result.Evidence = append(result.Evidence, bundle.Evidence...)
			}
		}
	}

	o.phase(&result, PhaseClassifyArtifacts, progress)
	o.phase(&result, PhaseCalculateRisk, progress)
	// A resource's first Classify pass (inside Observe, back during
	// PhaseDiscoverSystemResources) can't know yet whether any project
	// requires it -- that's only known now that PhaseResolveDependencies has
	// run. Re-classify every resource a REQUIRES edge targets so §20.3's
	// "current project depends on this SDK -> BLOCKED" rule actually applies.
	requiredResourceIDs := make(map[string]bool, len(result.Dependencies))
	for _, dependency := range result.Dependencies {
		if dependency.TargetType == domain.NodeResource && dependency.Relation == domain.RelationRequires {
			requiredResourceIDs[dependency.TargetID] = true
		}
	}
	for i, resource := range result.Resources {
		if !requiredResourceIDs[resource.ID] {
			continue
		}
		reclassified, err := o.resources.ReclassifyRequired(ctx, resource.ID)
		if err != nil {
			return o.fail(ctx, result, record, fmt.Errorf("reclassify required resource: %w", err))
		}
		result.Resources[i].Risk = reclassified.Resource.Risk
		result.Resources[i].ReclaimableSize = reclassified.Resource.ReclaimableSize
	}
	o.phase(&result, PhasePersistResults, progress)
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

	o.phase(&result, PhaseCompleted, progress)
	result.Status = ScanStatusCompleted
	if len(result.Issues) > 0 {
		result.Status = ScanStatusCompletedWithErrors
	}
	finishedAt := o.now()
	record.FinishedAt = &finishedAt
	record.FileCount = filesystemResult.FilesInspected
	record.ErrorCount = int64(len(result.Issues))
	record.Status = result.Status
	if o.issues != nil {
		if err := o.issues.Replace(context.WithoutCancel(ctx), options.ScanID, result.Issues); err != nil {
			return result, fmt.Errorf("persist scan issues: %w", err)
		}
	}
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

func (o *AnalysisOrchestrator) phase(result *ScanResult, phase AnalysisPhase, progress ScanProgress) {
	result.Phase = phase
	if o.onPhase != nil {
		o.onPhase(phase, progress)
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
	withoutCancel := context.WithoutCancel(ctx)
	var issueErr error
	if o.issues != nil {
		issueErr = o.issues.Replace(withoutCancel, record.ID, result.Issues)
	}
	return result, errors.Join(cause, issueErr, o.scans.Save(withoutCancel, record))
}

func structuredCandidateIssue(path, operation string, err error) Issue {
	return Issue{Code: IssueMalformedManifest, Phase: PhaseDiscoverProjects, Path: path,
		Operation: operation, Severity: IssueWarning, Message: err.Error(), Cause: err}
}

type workspaceCandidate struct {
	workspace    domain.Workspace
	projectPaths []string
}

// topLevelWorkspaceCandidates preserves every non-Node workspace and only
// the outermost Node workspace. Nested workspace expansion is intentionally
// outside the MVP contract (§19.2), even when a member also declares its own
// workspaces field.
func topLevelWorkspaceCandidates(candidates []workspaceCandidate) []workspaceCandidate {
	filtered := make([]workspaceCandidate, 0, len(candidates))
	for i, candidate := range candidates {
		if candidate.workspace.Type != domain.WorkspaceTypeNode {
			filtered = append(filtered, candidate)
			continue
		}
		root := filepath.Dir(candidate.workspace.ManifestPath)
		nested := false
		for j, other := range candidates {
			if i == j || other.workspace.Type != domain.WorkspaceTypeNode {
				continue
			}
			otherRoot := filepath.Dir(other.workspace.ManifestPath)
			inside, err := pathutil.IsSameOrChild(root, otherRoot)
			if err == nil && inside {
				same, _ := pathutil.Equal(root, otherRoot)
				if !same {
					nested = true
					break
				}
			}
		}
		if !nested {
			filtered = append(filtered, candidate)
		}
	}
	return filtered
}

func workspacesFromCandidates(candidates []workspaceCandidate) []domain.Workspace {
	workspaces := make([]domain.Workspace, 0, len(candidates))
	for _, candidate := range candidates {
		workspaces = append(workspaces, candidate.workspace)
	}
	return workspaces
}

func filterNodeProjectsByWorkspace(projects []domain.BuildProject, workspaces []workspaceCandidate) []domain.BuildProject {
	filtered := make([]domain.BuildProject, 0, len(projects))
	for _, project := range projects {
		if project.Type != domain.ProjectTypeNode || nodeProjectAllowedByWorkspaces(project, workspaces) {
			filtered = append(filtered, project)
		}
	}
	return filtered
}

func dedupeProjects(projects []domain.BuildProject) []domain.BuildProject {
	seen := make(map[string]struct{}, len(projects))
	unique := make([]domain.BuildProject, 0, len(projects))
	for _, project := range projects {
		if _, exists := seen[project.ID]; exists {
			continue
		}
		seen[project.ID] = struct{}{}
		unique = append(unique, project)
	}
	return unique
}

func nodeProjectAllowedByWorkspaces(project domain.BuildProject, workspaces []workspaceCandidate) bool {
	for _, candidate := range workspaces {
		if candidate.workspace.Type != domain.WorkspaceTypeNode {
			continue
		}
		root := filepath.Dir(candidate.workspace.ManifestPath)
		inside, err := pathutil.IsSameOrChild(project.RootPath, root)
		if err != nil || !inside {
			continue
		}
		sameRoot, err := pathutil.Equal(project.RootPath, root)
		if err == nil && sameRoot {
			return true
		}
		for _, memberManifest := range candidate.projectPaths {
			sameManifest, err := pathutil.Equal(project.ManifestPath, memberManifest)
			if err == nil && sameManifest {
				return true
			}
		}
		return false
	}
	return true
}

func filterProjectResourcesByOwner(candidates []ProjectResourceCandidate, projects []domain.BuildProject) []ProjectResourceCandidate {
	filtered := make([]ProjectResourceCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if _, found := projectByManifest(projects, candidate.OwnerManifestPath); found {
			filtered = append(filtered, candidate)
		}
	}
	return filtered
}

func dedupeProjectResources(candidates []ProjectResourceCandidate) []ProjectResourceCandidate {
	seen := make(map[string]struct{}, len(candidates))
	unique := make([]ProjectResourceCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		owner, ownerErr := pathutil.Normalize(candidate.OwnerManifestPath)
		resource, resourceErr := pathutil.Normalize(candidate.Resource.DisplayPath)
		if ownerErr != nil || resourceErr != nil {
			unique = append(unique, candidate)
			continue
		}
		key := owner + "\x00" + resource
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, candidate)
	}
	return unique
}

func filterProjectPropertiesByOwner(properties map[string][]ProjectProperty, projects []domain.BuildProject) map[string][]ProjectProperty {
	filtered := make(map[string][]ProjectProperty, len(properties))
	for _, project := range projects {
		if owned := properties[project.NormalizedManifestPath]; len(owned) > 0 {
			filtered[project.NormalizedManifestPath] = owned
		}
	}
	return filtered
}

func projectByManifest(projects []domain.BuildProject, manifestPath string) (domain.BuildProject, bool) {
	normalized, err := pathutil.Normalize(manifestPath)
	if err != nil {
		return domain.BuildProject{}, false
	}
	for _, project := range projects {
		if project.NormalizedManifestPath == normalized {
			return project, true
		}
	}
	return domain.BuildProject{}, false
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

func (i memoryResourceIndex) ListByType(resourceType domain.ResourceType) []domain.Resource {
	byVersion := i[resourceType]
	resources := make([]domain.Resource, 0)
	for _, matches := range byVersion {
		resources = append(resources, matches...)
	}
	return resources
}
