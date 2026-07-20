// [파일 역할] DependencyService.Persist는 어댑터가 이미 계산해 둔 의존성 그래프
// 간선(DependencyObservation = domain.Dependency + domain.Evidence)들을
// dependency_repository.go의 DependencyRepository를 통해 저장만 하는 서비스다.
// 의존성을 해석하는 로직은 전혀 없고 저장만 오케스트레이션한다.
//
// 주의: grep으로 직접 확인한 결과("NewDependencyService" 검색 시 이 파일의 정의부와
// 테스트 파일 외에는 매치가 없음) 이 서비스를 생성해서 쓰는 프로덕션 코드가 현재
// 저장소 어디에도 없다. internal/adapter/msbuild/resolve.go의 ResolveDependencies가
// 만들어내는 DependencyBundle을 저장하려고 만든 것으로 보이지만, 실제
// AnalysisOrchestrator.Run(analysis_orchestrator.go)은 DependencyService를 거치지
// 않고 DependencyRepository.UpsertGraph를 직접 호출한다. 즉 이 파일은 컴파일되고
// 자체 테스트도 통과하지만 실사용 경로에 배선되어 있지 않다 — cmd/scan.go가
// DependencyAnalyzer를 전혀 등록하지 않는 이슈(#22)와 같은 근본 원인이다. 다만 이
// 파일 자체가 orchestrator를 완전히 우회한다는 세부 사실은 이 주석을 쓰기 전까지
// docs/libra_review_findings_day4.md에는 아직 기록되어 있지 않았다.
package app

import (
	"context"
	"fmt"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

// DependencyObservation pairs a Dependency edge with the Evidence backing
// it, as produced by an adapter's dependency-resolution logic (e.g.
// msbuild.ResolveDependencies). It mirrors msbuild.ResolvedDependency's
// shape rather than importing it, keeping internal/app decoupled from any
// specific adapter package.
type DependencyObservation struct {
	Dependency domain.Dependency
	Evidence   []domain.Evidence
}

// DependencyService persists dependency graph edges an adapter has already
// resolved. It does not itself interpret project files or match versions --
// that is adapter-specific domain knowledge -- it only orchestrates storage.
type DependencyService struct {
	repository DependencyRepository
}

func NewDependencyService(repository DependencyRepository) *DependencyService {
	return &DependencyService{repository: repository}
}

// Persist upserts every observation into the dependency graph for scanID.
// It stops at the first failure; observations before it are already
// persisted (each UpsertGraph call is independently atomic per the
// repository contract, but the batch as a whole is not).
func (s *DependencyService) Persist(ctx context.Context, scanID string, observations []DependencyObservation) error {
	for _, observation := range observations {
		if err := s.repository.UpsertGraph(ctx, scanID, observation.Dependency, observation.Evidence); err != nil {
			return fmt.Errorf("persist dependency %q: %w", observation.Dependency.ID, err)
		}
	}
	return nil
}
