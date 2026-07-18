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
	if len(got) != 2 {
		t.Fatalf("got %d assessments, want 2: %+v", len(got), got)
	}
	for _, a := range got {
		if a.Scope != domain.ImpactScopeBuild || a.Level != domain.ImpactLevelHigh {
			t.Errorf("assessment = %+v, want BUILD/HIGH", a)
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
