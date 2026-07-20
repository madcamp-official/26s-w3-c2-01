package app

import (
	"context"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

func TestResourceListServiceFiltersAndCountsProjects(t *testing.T) {
	resources := fakeResourceRepository{resources: []domain.Resource{
		{ID: "r1", Name: "sdk", Type: domain.ResourceTypeWindowsSDK},
		{ID: "r2", Name: "modules", Type: domain.ResourceTypeNodeModules},
	}}
	dependencies := &dependencyRepositoryStub{byResource: map[string][]domain.Dependency{
		"r1": {{SourceID: "p1", TargetID: "r1"}},
	}}

	service := NewResourceListService(resources, dependencies)
	got, err := service.List(context.Background(), func(r domain.Resource) bool {
		return r.Type == domain.ResourceTypeWindowsSDK
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 1 || got[0].Resource.ID != "r1" {
		t.Fatalf("List() = %#v, want only r1", got)
	}
	if got[0].ProjectCount != 1 {
		t.Errorf("ProjectCount = %d, want 1", got[0].ProjectCount)
	}
}

func TestResourceListServiceNilFilterReturnsEverything(t *testing.T) {
	resources := fakeResourceRepository{resources: []domain.Resource{{ID: "r1"}, {ID: "r2"}}}
	dependencies := &dependencyRepositoryStub{byResource: map[string][]domain.Dependency{}}

	service := NewResourceListService(resources, dependencies)
	got, err := service.List(context.Background(), nil)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("List() = %#v, want both resources", got)
	}
}
