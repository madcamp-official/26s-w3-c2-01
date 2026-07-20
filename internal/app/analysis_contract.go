// [파일 역할] 이 파일은 AnalysisOrchestrator(analysis_orchestrator.go)와 모든
// 어댑터/디텍터가 공유하는 "공통 어휘(계약)"를 정의한다: 분석 단계(AnalysisPhase),
// 이슈 코드/심각도(IssueCode/IssueSeverity), 제네릭 결과 봉투(DetectionResult[T]),
// 그리고 ProjectDetector/ResourceDetector/DependencyAnalyzer 세 인터페이스가 그것이다.
// project_detector_adapters.go와 resource_detector_adapters.go는 각 internal/adapter/*
// 패키지의 구체 타입을 여기 정의된 인터페이스로 감싸는 역할만 하고, 실제 파이프라인
// 흐름 제어는 analysis_orchestrator.go의 AnalysisOrchestrator.Run이 담당한다.
// docs/libra_integration_contracts.md §18.2를 그대로 구현한 파일이므로, 여기를 바꾸는
// 것은 로컬 리팩터링이 아니라 3인 공동 소유 크로스팀 계약을 바꾸는 일이다.
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

type ProjectCandidate struct {
	Projects              []domain.BuildProject
	ProjectResources      []ProjectResourceCandidate
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
