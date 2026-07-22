package app

import (
	"context"
	"fmt"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

// ImpactService judges the impact of removing a resource on the projects
// that depend on it, using dependency edges an adapter has already resolved
// and persisted (e.g. via msbuild.ResolveDependencies + DependencyService).
// It does not resolve dependencies itself.
type ImpactService struct {
	dependencies DependencyRepository
	resources    ResourceRepository
}

// NewImpactService takes a ResourceRepository in addition to
// DependencyRepository so ownsAssessments can read the resource's own Type
// and Regenerable fact -- RelationOwns judgment depends on what the resource
// itself is, unlike RelationRequires judgment, which only needs the edge.
func NewImpactService(dependencies DependencyRepository, resources ResourceRepository) *ImpactService {
	return &ImpactService{dependencies: dependencies, resources: resources}
}

// Assess reports RUN/BUILD/DEBUG impact for every project with a direct
// dependency on resourceID. Projects with no dependency are omitted entirely
// rather than reported as NONE, matching the "Affected projects: N" style in
// docs/libra_cli_commands_and_schedule.md §3.7, where only affected
// projects are listed at all.
//
// RelationRequires (a project declares the resource as a build/run
// dependency, e.g. a Windows SDK) and RelationOwns (the resource *is* a
// project-produced artifact, e.g. node_modules or a bin/obj/dist directory)
// get different rule sets -- see requiresAssessments and ownsAssessments.
// They cannot share one: deleting your own regenerable output has the
// opposite BUILD/DEBUG polarity from deleting a required SDK (the next
// build recreates it, rather than failing without it).
func (s *ImpactService) Assess(ctx context.Context, resourceID string) ([]domain.ImpactAssessment, error) {
	dependents, err := s.dependencies.FindProjectsByResource(ctx, resourceID)
	if err != nil {
		return nil, err
	}

	var requiresProjectIDs, ownsProjectIDs []string
	for _, dep := range dependents {
		if dep.SourceType != domain.NodeProject {
			continue
		}
		switch dep.Relation {
		case domain.RelationRequires:
			requiresProjectIDs = append(requiresProjectIDs, dep.SourceID)
		case domain.RelationOwns:
			ownsProjectIDs = append(ownsProjectIDs, dep.SourceID)
		}
	}

	var assessments []domain.ImpactAssessment
	for _, projectID := range requiresProjectIDs {
		assessments = append(assessments, requiresAssessments(projectID)...)
	}

	if len(ownsProjectIDs) > 0 {
		resource, err := s.resources.FindByID(ctx, resourceID)
		if err != nil {
			return nil, fmt.Errorf("find resource %q: %w", resourceID, err)
		}
		for _, projectID := range ownsProjectIDs {
			assessments = append(assessments, ownsAssessments(projectID, resource)...)
		}
	}

	return assessments, nil
}

// requiresAssessments is the impact of removing a resource a project
// declares it needs to build (RelationRequires) -- e.g. a Windows SDK or the
// active Xcode install. These are the only rules a direct Dependency edge
// can support without deeper domain modeling:
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
func requiresAssessments(projectID string) []domain.ImpactAssessment {
	return []domain.ImpactAssessment{
		{ProjectID: projectID, Scope: domain.ImpactScopeRun, Level: domain.ImpactLevelLow,
			Reason: "already-built executables do not require the resource again unless they load a runtime DLL it provides"},
		{ProjectID: projectID, Scope: domain.ImpactScopeBuild, Level: domain.ImpactLevelHigh,
			Reason: "project declares a dependency on this resource"},
		{ProjectID: projectID, Scope: domain.ImpactScopeDebug, Level: domain.ImpactLevelHigh,
			Reason: "IDE debugging triggers a rebuild, which fails without the resource"},
		{ProjectID: projectID, Scope: domain.ImpactScopeCI, Level: domain.ImpactLevelUnknown,
			Reason: "remote CI environments are not verified by a local scan"},
	}
}

// ownsAssessments is the impact of removing a resource the project itself
// produced or installed into its own tree (RelationOwns) -- the resource
// *is* the artifact, not something borrowed from outside it. Only the two
// resource shapes libra can currently tell apart get a real judgment; any
// other OWNS type (e.g. a project-owned conda/venv environment) returns nil,
// which renders as UNKNOWN via cmd/explain.go's fixed impactScopes loop --
// the same honest "don't know" those commands already use for any scope
// with no signal at all, docs/libra_integration_contracts.md §20.4.
func ownsAssessments(projectID string, resource domain.Resource) []domain.ImpactAssessment {
	switch resource.Type {
	case domain.ResourceTypeNodeModules, domain.ResourceTypePods:
		// A dependency store consumed while building, not the output
		// itself -- BUILD/DEBUG are HIGH regardless of Regenerable, because
		// the build fails without it being present whether or not
		// reinstalling it is well-evidenced.
		return []domain.ImpactAssessment{
			{ProjectID: projectID, Scope: domain.ImpactScopeRun, Level: domain.ImpactLevelLow,
				Reason: "an already-built or bundled output does not read from this directory at runtime, unless the project loads these packages directly without bundling"},
			{ProjectID: projectID, Scope: domain.ImpactScopeBuild, Level: domain.ImpactLevelHigh,
				Reason: "the next build fails until dependencies are reinstalled here"},
			{ProjectID: projectID, Scope: domain.ImpactScopeDebug, Level: domain.ImpactLevelHigh,
				Reason: "debugging triggers a rebuild, which fails until dependencies are reinstalled"},
		}
	case domain.ResourceTypeBuildOutput:
		// The resource is the build output itself. RUN stays UNKNOWN even
		// here: libra types an OutDir (holds the final executable) and an
		// IntDir (intermediate objects only) identically as build-output,
		// so it cannot tell which this instance is. BUILD/DEBUG instead
		// track Regenerable, not a blanket claim -- a bin/obj directory
		// regenerates on rebuild, but a dist/build directory with no known
		// build step (Regenerable=false) does not.
		buildLevel, buildReason := domain.ImpactLevelUnknown, "no known way to regenerate this output was found"
		debugReason := "the IDE rebuilds before debugging, but no known way to regenerate this output was found"
		if resource.Regenerable {
			buildLevel, buildReason = domain.ImpactLevelLow, "rebuilding regenerates this directory from source"
			debugReason = "the IDE rebuilds before debugging, which regenerates this directory too"
		}
		return []domain.ImpactAssessment{
			{ProjectID: projectID, Scope: domain.ImpactScopeRun, Level: domain.ImpactLevelUnknown,
				Reason: "libra cannot tell whether this directory holds the artifact currently being run or only intermediate build output"},
			{ProjectID: projectID, Scope: domain.ImpactScopeBuild, Level: buildLevel, Reason: buildReason},
			{ProjectID: projectID, Scope: domain.ImpactScopeDebug, Level: buildLevel, Reason: debugReason},
		}
	default:
		return nil
	}
}
