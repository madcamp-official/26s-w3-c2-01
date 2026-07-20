// [파일 역할] ImpactService.Assess는 dependency_repository.go의
// DependencyRepository를 통해 이미 저장되어 있는 PROJECT -> RESOURCE 의존성
// 그래프를 조회해서, 특정 리소스를 제거했을 때 어떤 프로젝트의 어떤 활동이
// 영향을 받는지 domain/impact.go의 ImpactAssessment로 판정하는 파일이다.
// 의존성을 새로 분석/해석하지 않고 이미 만들어진 그래프만 읽는다는 점에서
// dependency_service.go(그래프를 "쓰는" 쪽)와 대비된다. 현재는 BUILD 스코프,
// 직접 의존(dep.SourceType == domain.NodeProject)만 판정하고 RUN/DEBUG/CI와
// UnverifiedScope를 고려한 규칙은 아직 구현되어 있지 않다(주석에 명시됨).
package app

import (
	"context"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

// ImpactService judges the impact of removing a resource on the projects
// that depend on it, using dependency edges an adapter has already resolved
// and persisted (e.g. via msbuild.ResolveDependencies + DependencyService).
// It does not resolve dependencies itself.
type ImpactService struct {
	dependencies DependencyRepository
}

func NewImpactService(dependencies DependencyRepository) *ImpactService {
	return &ImpactService{dependencies: dependencies}
}

// Assess reports BUILD impact for every project with a direct dependency on
// resourceID. Projects with no dependency are omitted entirely rather than
// reported as NONE, matching the "Affected projects: N" style in
// docs/libra_cli_commands_and_schedule.md §3.7, where only affected
// projects are listed at all.
//
// RUN, DEBUG, and CI scopes, and any rule that consults UnverifiedScope, are
// not implemented yet: they need further domain-modeling decisions (see
// docs/libra_integration_contracts.md §20.4) beyond what a direct
// Dependency edge alone can answer.
func (s *ImpactService) Assess(ctx context.Context, resourceID string) ([]domain.ImpactAssessment, error) {
	dependents, err := s.dependencies.FindProjectsByResource(ctx, resourceID)
	if err != nil {
		return nil, err
	}

	var assessments []domain.ImpactAssessment
	for _, dep := range dependents {
		if dep.SourceType != domain.NodeProject {
			continue
		}
		assessments = append(assessments, domain.ImpactAssessment{
			ProjectID: dep.SourceID,
			Scope:     domain.ImpactScopeBuild,
			Level:     domain.ImpactLevelHigh,
			Reason:    "project declares a dependency on this resource",
		})
	}
	return assessments, nil
}
