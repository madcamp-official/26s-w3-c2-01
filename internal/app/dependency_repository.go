// [파일 역할] PROJECT -> RESOURCE 의존성 그래프의 저장 계약인
// DependencyRepository 인터페이스 하나만 선언하는 파일이다. 실제 구현체는
// internal/store/sqlite.DependencyRepository이고, internal/app 안에서는
// dependency_service.go의 DependencyService, impact_service.go의
// ImpactService, analysis_orchestrator.go의 AnalysisOrchestrator, 그리고
// (아직 스텁인) explain_service.go가 이 인터페이스에만 의존한다. 구체적인
// sqlite 패키지를 직접 참조하지 않기 때문에 인메모리 가짜 구현으로도 단위
// 테스트가 가능하다.
package app

import (
	"context"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

// DependencyRepository is the persistence contract for the PROJECT ->
// RESOURCE dependency graph (implemented by
// internal/store/sqlite.DependencyRepository). app.ImpactService and
// app.ExplainService both depend only on this interface, not the sqlite
// package, so they can be unit-tested against an in-memory stub.
type DependencyRepository interface {
	UpsertGraph(context.Context, string, domain.Dependency, []domain.Evidence) error
	FindResourcesByProject(context.Context, string) ([]domain.Dependency, error)
	FindProjectsByResource(context.Context, string) ([]domain.Dependency, error)
	FindEvidence(context.Context, string) ([]domain.Evidence, error)
}
