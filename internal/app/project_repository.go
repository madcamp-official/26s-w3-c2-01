// [파일 역할] BuildProject/Workspace 저장 계약인 ProjectRepository와
// WorkspaceRepository 두 인터페이스를 선언하는 파일이다. 구현체는
// internal/store/sqlite에 있다. 하나로 합치지 않고 둘로 나눈 이유는
// domain/project.go의 주석대로 BuildProject가 Workspace 없이도 존재할 수
// 있기 때문(§3.1) — cmd/projects.go처럼 프로젝트만 다루는 호출자가
// WorkspaceRepository에 얹혀갈 필요가 없게 한다. analysis_orchestrator.go의
// AnalysisOrchestrator와 summary_service.go의 SummaryService가
// ProjectRepository를, AnalysisOrchestrator가 WorkspaceRepository를 함께 사용한다.
package app

import (
	"context"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

// ProjectRepository and WorkspaceRepository are the persistence contracts
// for BuildProject/Workspace (implemented by internal/store/sqlite). Kept
// as two separate interfaces, not one, because a BuildProject can exist
// with no Workspace at all (§3.1 of docs/libra_integration_contracts.md)
// and callers that only ever touch one side (e.g. cmd/projects.go never
// touches WorkspaceRepository) shouldn't have to depend on the other.
type ProjectRepository interface {
	UpsertObserved(context.Context, string, []domain.BuildProject) error
	FindByID(context.Context, string) (domain.BuildProject, error)
	FindByManifestPath(context.Context, domain.ProjectType, string) (domain.BuildProject, error)
	List(context.Context) ([]domain.BuildProject, error)
}

type WorkspaceRepository interface {
	Upsert(context.Context, string, domain.Workspace) error
	ReplaceMembers(context.Context, string, []string) error
}
