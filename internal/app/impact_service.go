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
