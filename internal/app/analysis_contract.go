package app

import (
	"context"
	"fmt"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/scanner"
)

// analysis_contract.go is the shared vocabulary every adapter, detector,
// and AnalysisOrchestrator itself is written against: phases, issue
// codes/severity, the generic DetectionResult envelope, and the
// ProjectDetector/ResourceDetector/DependencyAnalyzer interfaces. See §18.2
// of docs/libra_integration_contracts.md, which this file implements
// directly -- changing anything here is a common-owned/cross-team contract
// change, not a local refactor.
type AnalysisPhase string

const (
	PhaseDiscoverFiles           AnalysisPhase = "DISCOVER_FILES"
	PhaseDiscoverProjects        AnalysisPhase = "DISCOVER_PROJECTS"
	PhaseDiscoverSystemResources AnalysisPhase = "DISCOVER_SYSTEM_RESOURCES"
	PhaseAnalyzeProjectSettings  AnalysisPhase = "ANALYZE_PROJECT_SETTINGS"
	PhaseResolveDependencies     AnalysisPhase = "RESOLVE_DEPENDENCIES"
	PhaseClassifyArtifacts       AnalysisPhase = "CLASSIFY_ARTIFACTS"
	PhaseCalculateRisk           AnalysisPhase = "CALCULATE_RISK"
	PhasePersistResults          AnalysisPhase = "PERSIST_RESULTS"
	PhaseCompleted               AnalysisPhase = "COMPLETED"
)

type IssueSeverity string

const (
	IssueWarning IssueSeverity = "WARNING"
	IssueError   IssueSeverity = "ERROR"
)

type IssueCode string

const (
	IssueAccessDenied        IssueCode = "ACCESS_DENIED"
	IssueMalformedManifest   IssueCode = "MALFORMED_MANIFEST"
	IssueUnsupportedPlatform IssueCode = "UNSUPPORTED_PLATFORM"
	IssueAdapterFailed       IssueCode = "ADAPTER_FAILED"
	IssueDatabaseWriteFailed IssueCode = "DB_WRITE_FAILED"
	IssueCancelled           IssueCode = "CANCELLED"
)

type Issue struct {
	Code      IssueCode
	Phase     AnalysisPhase
	Adapter   string
	Path      string
	Operation string
	Severity  IssueSeverity
	Message   string
	Cause     error
}

func (i Issue) Error() string {
	if i.Path != "" {
		return fmt.Sprintf("%s %s: %s", i.Operation, i.Path, i.Message)
	}
	return fmt.Sprintf("%s: %s", i.Operation, i.Message)
}

func (i Issue) Unwrap() error { return i.Cause }

type UnverifiedScope struct {
	Path   string
	Phase  AnalysisPhase
	Reason string
}

type DetectionResult[T any] struct {
	Items      []T
	Issues     []Issue
	Unverified []UnverifiedScope
}

// ProjectResourceCandidate is a resource discovered alongside a project
// during the same file-walk pass (e.g. Node's node_modules/dist/build next
// to package.json), before its owning project's stable ID is known.
//
// OwnerManifestPath records which project the resource belongs to so a
// future PROJECT -> RESOURCE dependency edge can be built once every
// candidate's project has been prepared (resolving the manifest path to a
// project ID the same way WorkspaceProjectPaths is resolved). That edge is
// not built yet: the current pipeline only observes and persists the
// Resource so it shows up in `summary`; linking it to its project is
// deferred to the Day 4 dependency graph.
type ProjectResourceCandidate struct {
	OwnerManifestPath string
	Resource          domain.Resource
}

// ProjectProperty is an adapter-neutral declared build property associated
// with the project identified by OwnerManifestPath.
type ProjectProperty struct {
	OwnerManifestPath string
	SourcePath        string
	Name              string
	Value             string
	Condition         string
}

type ProjectCandidate struct {
	Projects              []domain.BuildProject
	ProjectResources      []ProjectResourceCandidate
	ProjectProperties     []ProjectProperty
	Workspace             *domain.Workspace
	WorkspaceProjectPaths []string
}

// ProjectAnalysisInput contains a prepared project and the declared build
// properties that belong to it. Adapter-specific property types must be
// converted at the ProjectDetector boundary.
type ProjectAnalysisInput struct {
	Project    domain.BuildProject
	Properties []ProjectProperty
}

type DependencyBundle struct {
	Dependency domain.Dependency
	Evidence   []domain.Evidence
}

type Environment struct{}

type ResourceIndex interface {
	Find(domain.ResourceType, string) []domain.Resource
	ListByType(domain.ResourceType) []domain.Resource
}

type ProjectDetector interface {
	Observe(context.Context, scanner.Entry) DetectionResult[ProjectCandidate]
}

type ResourceDetector interface {
	Detect(context.Context, Environment) DetectionResult[domain.Resource]
}

type DependencyAnalyzer interface {
	Analyze(context.Context, ProjectAnalysisInput, ResourceIndex) DetectionResult[DependencyBundle]
}
