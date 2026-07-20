// [파일 역할] PrepareBuildProject / PrepareWorkspace 두 함수만 담고 있는
// 파일이다. project_detector_adapters.go의 어댑터들이 만들어낸 "관찰 사실
// (domain.BuildProject / domain.Workspace 원본 candidate)"을 절대경로화·경로
// 정규화하고 domain.ProjectID / domain.WorkspaceID로 안정적 ID를 부여해
// "저장 가능한 상태"로 바꾸는 순수 변환 로직만 모아 둔다. 실제 저장 자체는
// project_repository.go의 ProjectRepository/WorkspaceRepository가 담당하므로,
// 이 파일은 "저장 전 정규화" 책임만 별도로 분리한 것이다.
// analysis_orchestrator.go의 AnalysisOrchestrator.Run이 DISCOVER_PROJECTS
// 단계에서 각 ProjectCandidate.Projects / Workspace마다 이 두 함수를 호출한다.
package app

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/pathutil"
)

// PrepareBuildProject turns an adapter fact into a storage-ready project.
func PrepareBuildProject(candidate domain.BuildProject, observedAt time.Time) (domain.BuildProject, error) {
	if candidate.Name == "" || candidate.Type == "" {
		return domain.BuildProject{}, errors.New("project name and type are required")
	}
	if observedAt.IsZero() {
		return domain.BuildProject{}, errors.New("project observation time is required")
	}
	rootPath, err := pathutil.Absolute(candidate.RootPath)
	if err != nil {
		return domain.BuildProject{}, fmt.Errorf("resolve project root: %w", err)
	}
	manifestPath, err := pathutil.Absolute(candidate.ManifestPath)
	if err != nil {
		return domain.BuildProject{}, fmt.Errorf("resolve project manifest: %w", err)
	}
	inside, err := pathutil.IsSameOrChild(manifestPath, rootPath)
	if err != nil {
		return domain.BuildProject{}, fmt.Errorf("compare project root and manifest: %w", err)
	}
	if !inside {
		return domain.BuildProject{}, errors.New("project manifest must be inside project root")
	}
	normalizedRoot, err := pathutil.Normalize(rootPath)
	if err != nil {
		return domain.BuildProject{}, fmt.Errorf("normalize project root: %w", err)
	}
	normalizedManifest, err := pathutil.Normalize(manifestPath)
	if err != nil {
		return domain.BuildProject{}, fmt.Errorf("normalize project manifest: %w", err)
	}

	candidate.RootPath = rootPath
	candidate.NormalizedRootPath = normalizedRoot
	candidate.ManifestPath = manifestPath
	candidate.NormalizedManifestPath = normalizedManifest
	candidate.ID = domain.ProjectID(candidate.Type, normalizedManifest)
	candidate.Drive = filepath.VolumeName(rootPath)
	candidate.LastObservedAt = observedAt
	candidate.Status = domain.ProjectStatusActive
	return candidate, nil
}

// PrepareWorkspace turns a parser fact into a storage-ready workspace.
func PrepareWorkspace(candidate domain.Workspace, observedAt time.Time) (domain.Workspace, error) {
	if candidate.Name == "" || candidate.Type == "" {
		return domain.Workspace{}, errors.New("workspace name and type are required")
	}
	if observedAt.IsZero() {
		return domain.Workspace{}, errors.New("workspace observation time is required")
	}
	manifestPath, err := pathutil.Absolute(candidate.ManifestPath)
	if err != nil {
		return domain.Workspace{}, fmt.Errorf("resolve workspace manifest: %w", err)
	}
	normalizedManifest, err := pathutil.Normalize(manifestPath)
	if err != nil {
		return domain.Workspace{}, fmt.Errorf("normalize workspace manifest: %w", err)
	}
	candidate.ManifestPath = manifestPath
	candidate.NormalizedManifestPath = normalizedManifest
	candidate.ID = domain.WorkspaceID(candidate.Type, normalizedManifest)
	candidate.LastObservedAt = observedAt
	return candidate, nil
}
