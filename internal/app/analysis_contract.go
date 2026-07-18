package app

import (
	"context"
	"fmt"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/scanner"
)

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

type ProjectCandidate struct {
	Projects              []domain.BuildProject
	Workspace             *domain.Workspace
	WorkspaceProjectPaths []string
}

type DependencyBundle struct {
	Dependency domain.Dependency
	Evidence   []domain.Evidence
}

type Environment struct{}

type ResourceIndex interface {
	Find(domain.ResourceType, string) []domain.Resource
}

type ProjectDetector interface {
	Observe(context.Context, scanner.Entry) DetectionResult[ProjectCandidate]
}

type ResourceDetector interface {
	Detect(context.Context, Environment) DetectionResult[domain.Resource]
}

type DependencyAnalyzer interface {
	Analyze(context.Context, domain.BuildProject, ResourceIndex) DetectionResult[DependencyBundle]
}
