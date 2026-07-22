package app

import (
	"context"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

func TestImpactServiceAssessesDirectDependents(t *testing.T) {
	repository := &dependencyRepositoryStub{
		byResource: map[string][]domain.Dependency{
			"resource-1": {
				{SourceType: domain.NodeProject, SourceID: "project-1", TargetType: domain.NodeResource, TargetID: "resource-1", Relation: domain.RelationRequires},
				{SourceType: domain.NodeProject, SourceID: "project-2", TargetType: domain.NodeResource, TargetID: "resource-1", Relation: domain.RelationRequires},
			},
		},
	}
	service := NewImpactService(repository, &resourceRepositoryStub{})

	got, err := service.Assess(context.Background(), "resource-1")
	if err != nil {
		t.Fatalf("Assess() error = %v", err)
	}
	// 2 dependent projects x 4 scopes (RUN, BUILD, DEBUG, CI) each.
	if len(got) != 8 {
		t.Fatalf("got %d assessments, want 8: %+v", len(got), got)
	}

	want := map[domain.ImpactScope]domain.ImpactLevel{
		domain.ImpactScopeRun:   domain.ImpactLevelLow,
		domain.ImpactScopeBuild: domain.ImpactLevelHigh,
		domain.ImpactScopeDebug: domain.ImpactLevelHigh,
		domain.ImpactScopeCI:    domain.ImpactLevelUnknown,
	}
	for _, a := range got {
		if a.Level != want[a.Scope] {
			t.Errorf("assessment = %+v, want level %s for scope %s", a, want[a.Scope], a.Scope)
		}
	}
}

func TestImpactServiceAssess_NoDependents(t *testing.T) {
	repository := &dependencyRepositoryStub{
		byResource: map[string][]domain.Dependency{},
	}
	service := NewImpactService(repository, &resourceRepositoryStub{})

	got, err := service.Assess(context.Background(), "resource-with-no-dependents")
	if err != nil {
		t.Fatalf("Assess() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d assessments, want 0 (unaffected projects are omitted, not listed as NONE): %+v", len(got), got)
	}
}

// TestImpactServiceAssessesOwnedDependencyStore covers node_modules/Pods --
// a project-owned dependency store consumed while building, not the output
// itself. Before this, RelationOwns edges were skipped entirely and every
// scope rendered as UNKNOWN (see cmd/explain.go's impactScopes loop), even
// though the resource clearly has a dependent: the owning project.
func TestImpactServiceAssessesOwnedDependencyStore(t *testing.T) {
	dependencies := &dependencyRepositoryStub{
		byResource: map[string][]domain.Dependency{
			"node-modules-1": {
				{SourceType: domain.NodeProject, SourceID: "project-1", TargetType: domain.NodeResource, TargetID: "node-modules-1", Relation: domain.RelationOwns},
			},
		},
	}
	resources := &resourceRepositoryStub{byID: domain.Resource{ID: "node-modules-1", Type: domain.ResourceTypeNodeModules}}
	service := NewImpactService(dependencies, resources)

	got, err := service.Assess(context.Background(), "node-modules-1")
	if err != nil {
		t.Fatalf("Assess() error = %v", err)
	}

	want := map[domain.ImpactScope]domain.ImpactLevel{
		domain.ImpactScopeRun:   domain.ImpactLevelLow,
		domain.ImpactScopeBuild: domain.ImpactLevelHigh,
		domain.ImpactScopeDebug: domain.ImpactLevelHigh,
	}
	if len(got) != len(want) {
		t.Fatalf("got %d assessments, want %d: %+v", len(got), len(want), got)
	}
	for _, a := range got {
		if a.ProjectID != "project-1" {
			t.Errorf("assessment = %+v, want ProjectID project-1", a)
		}
		if a.Level != want[a.Scope] {
			t.Errorf("assessment = %+v, want level %s for scope %s", a, want[a.Scope], a.Scope)
		}
	}
}

// TestImpactServiceAssessesOwnedBuildOutput covers bin/obj/dist-style
// resources, split on Regenerable -- a regenerable build-output directory
// (e.g. GameClient's bin, rebuilt from source) recovers cleanly, while a
// non-regenerable one (e.g. a Node dist with no build script) does not, so
// BUILD/DEBUG must not claim LOW for both alike.
func TestImpactServiceAssessesOwnedBuildOutput(t *testing.T) {
	for _, tc := range []struct {
		name        string
		regenerable bool
		wantBuild   domain.ImpactLevel
	}{
		{"regenerable", true, domain.ImpactLevelLow},
		{"not regenerable", false, domain.ImpactLevelUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dependencies := &dependencyRepositoryStub{
				byResource: map[string][]domain.Dependency{
					"build-output-1": {
						{SourceType: domain.NodeProject, SourceID: "project-1", TargetType: domain.NodeResource, TargetID: "build-output-1", Relation: domain.RelationOwns},
					},
				},
			}
			resources := &resourceRepositoryStub{byID: domain.Resource{ID: "build-output-1", Type: domain.ResourceTypeBuildOutput, Regenerable: tc.regenerable}}
			service := NewImpactService(dependencies, resources)

			got, err := service.Assess(context.Background(), "build-output-1")
			if err != nil {
				t.Fatalf("Assess() error = %v", err)
			}

			want := map[domain.ImpactScope]domain.ImpactLevel{
				domain.ImpactScopeRun:   domain.ImpactLevelUnknown,
				domain.ImpactScopeBuild: tc.wantBuild,
				domain.ImpactScopeDebug: tc.wantBuild,
			}
			if len(got) != len(want) {
				t.Fatalf("got %d assessments, want %d: %+v", len(got), len(want), got)
			}
			for _, a := range got {
				if a.Level != want[a.Scope] {
					t.Errorf("assessment = %+v, want level %s for scope %s", a, want[a.Scope], a.Scope)
				}
			}
		})
	}
}

// TestImpactServiceOwnedUnknownTypeStaysUnassessed covers an OWNS resource
// type ownsAssessments has no rule for (e.g. a project-owned conda-env):
// Assess must not guess, and must not fail the resource lookup path either.
func TestImpactServiceOwnedUnknownTypeStaysUnassessed(t *testing.T) {
	dependencies := &dependencyRepositoryStub{
		byResource: map[string][]domain.Dependency{
			"conda-env-1": {
				{SourceType: domain.NodeProject, SourceID: "project-1", TargetType: domain.NodeResource, TargetID: "conda-env-1", Relation: domain.RelationOwns},
			},
		},
	}
	resources := &resourceRepositoryStub{byID: domain.Resource{ID: "conda-env-1", Type: domain.ResourceTypeCondaEnv}}
	service := NewImpactService(dependencies, resources)

	got, err := service.Assess(context.Background(), "conda-env-1")
	if err != nil {
		t.Fatalf("Assess() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d assessments, want 0 (unhandled OWNS type renders as UNKNOWN via absence, not a guess): %+v", len(got), got)
	}
}
