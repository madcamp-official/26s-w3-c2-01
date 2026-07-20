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
	service := NewImpactService(repository)

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
	service := NewImpactService(repository)

	got, err := service.Assess(context.Background(), "resource-with-no-dependents")
	if err != nil {
		t.Fatalf("Assess() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d assessments, want 0 (unaffected projects are omitted, not listed as NONE): %+v", len(got), got)
	}
}
