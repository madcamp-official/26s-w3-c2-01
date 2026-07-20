package app

import (
	"context"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

func TestProjectListServiceFiltersAndCountsResources(t *testing.T) {
	projects := fakeProjectRepository{projects: []domain.BuildProject{
		{ID: "p1", Name: "web", Type: domain.ProjectTypeNode},
		{ID: "p2", Name: "game", Type: domain.ProjectTypeMSBuildCpp},
	}}
	dependencies := &dependencyRepositoryStub{byProject: map[string][]domain.Dependency{
		"p1": {{SourceID: "p1", TargetID: "r1"}, {SourceID: "p1", TargetID: "r2"}},
	}}

	service := NewProjectListService(projects, dependencies)
	got, err := service.List(context.Background(), func(p domain.BuildProject) bool {
		return p.Type == domain.ProjectTypeNode
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 1 || got[0].Project.ID != "p1" {
		t.Fatalf("List() = %#v, want only p1", got)
	}
	if got[0].ResourceCount != 2 {
		t.Errorf("ResourceCount = %d, want 2", got[0].ResourceCount)
	}
}

func TestProjectListServiceNilFilterReturnsEverything(t *testing.T) {
	projects := fakeProjectRepository{projects: []domain.BuildProject{{ID: "p1"}, {ID: "p2"}}}
	dependencies := &dependencyRepositoryStub{byProject: map[string][]domain.Dependency{}}

	service := NewProjectListService(projects, dependencies)
	got, err := service.List(context.Background(), nil)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("List() = %#v, want both projects", got)
	}
}
