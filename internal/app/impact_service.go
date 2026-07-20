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

// Assess reports RUN/BUILD/DEBUG/CI impact for every project with a direct
// dependency on resourceID. Projects with no dependency are omitted entirely
// rather than reported as NONE, matching the "Affected projects: N" style in
// docs/libra_cli_commands_and_schedule.md §3.7, where only affected
// projects are listed at all.
//
// The rules below are the only signal a direct Dependency edge can support
// without deeper domain modeling (see docs/libra_integration_contracts.md
// §20.4, DECISION_REQUIRED):
//
//   - BUILD is HIGH: the project declares the resource as required to build.
//   - DEBUG mirrors BUILD, HIGH: IDEs typically rebuild before starting a
//     debug session (e.g. Visual Studio F5), so a build failure fails
//     debugging too.
//   - RUN is LOW: an already-built executable does not normally need the
//     SDK again. This does not cover the case where the executable loads a
//     runtime DLL that shipped with the SDK -- libra cannot distinguish
//     build-time-only from runtime dependencies from a REQUIRES edge alone.
//   - CI is UNKNOWN: libra only analyzes the local machine, so it cannot
//     verify whether a remote CI environment provisions this resource
//     independently. This is UnverifiedScope territory, not an evaluated
//     absence.
func (s *ImpactService) Assess(ctx context.Context, resourceID string) ([]domain.ImpactAssessment, error) {
	dependents, err := s.dependencies.FindProjectsByResource(ctx, resourceID)
	if err != nil {
		return nil, err
	}

	var assessments []domain.ImpactAssessment
	for _, dep := range dependents {
		if dep.SourceType != domain.NodeProject || dep.Relation != domain.RelationRequires {
			continue
		}
		assessments = append(assessments,
			domain.ImpactAssessment{
				ProjectID: dep.SourceID,
				Scope:     domain.ImpactScopeRun,
				Level:     domain.ImpactLevelLow,
				Reason:    "already-built executables do not require the resource again unless they load a runtime DLL it provides",
			},
			domain.ImpactAssessment{
				ProjectID: dep.SourceID,
				Scope:     domain.ImpactScopeBuild,
				Level:     domain.ImpactLevelHigh,
				Reason:    "project declares a dependency on this resource",
			},
			domain.ImpactAssessment{
				ProjectID: dep.SourceID,
				Scope:     domain.ImpactScopeDebug,
				Level:     domain.ImpactLevelHigh,
				Reason:    "IDE debugging triggers a rebuild, which fails without the resource",
			},
			domain.ImpactAssessment{
				ProjectID: dep.SourceID,
				Scope:     domain.ImpactScopeCI,
				Level:     domain.ImpactLevelUnknown,
				Reason:    "remote CI environments are not verified by a local scan",
			},
		)
	}
	return assessments, nil
}
