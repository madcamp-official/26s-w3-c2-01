package cmd

import (
	"context"
	"errors"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

// fakeResourceRepository and fakeProjectRepository are minimal in-memory
// implementations of app.ResourceRepository/app.ProjectRepository, used to
// unit test resolveTarget's matching rules without a real database.

type fakeResourceRepository struct {
	byID []domain.Resource
}

func (f *fakeResourceRepository) Upsert(context.Context, domain.Resource) error { return nil }

func (f *fakeResourceRepository) FindByID(_ context.Context, id string) (domain.Resource, error) {
	for _, r := range f.byID {
		if r.ID == id {
			return r, nil
		}
	}
	return domain.Resource{}, errors.New("resource not found")
}

func (f *fakeResourceRepository) ListByType(_ context.Context, t domain.ResourceType) ([]domain.Resource, error) {
	var out []domain.Resource
	for _, r := range f.byID {
		if r.Type == t {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeResourceRepository) List(context.Context) ([]domain.Resource, error) {
	return f.byID, nil
}

type fakeProjectRepository struct {
	byID []domain.BuildProject
}

func (f *fakeProjectRepository) UpsertObserved(context.Context, string, []domain.BuildProject) error {
	return nil
}

func (f *fakeProjectRepository) FindByID(_ context.Context, id string) (domain.BuildProject, error) {
	for _, p := range f.byID {
		if p.ID == id {
			return p, nil
		}
	}
	return domain.BuildProject{}, errors.New("project not found")
}

func (f *fakeProjectRepository) FindByManifestPath(_ context.Context, t domain.ProjectType, path string) (domain.BuildProject, error) {
	for _, p := range f.byID {
		if p.Type == t && p.NormalizedManifestPath == path {
			return p, nil
		}
	}
	return domain.BuildProject{}, errors.New("project not found")
}

func (f *fakeProjectRepository) List(context.Context) ([]domain.BuildProject, error) {
	return f.byID, nil
}

func testResources() *fakeResourceRepository {
	return &fakeResourceRepository{byID: []domain.Resource{
		{ID: "res-sdk-1", Name: "Windows SDK", Type: domain.ResourceTypeWindowsSDK, Version: "10.0.22621.0",
			DisplayPath: "/kits/10.0.22621.0", NormalizedPath: "/kits/10.0.22621.0"},
		{ID: "res-nm-1", Name: "node_modules", Type: domain.ResourceTypeNodeModules, Version: "",
			DisplayPath: "/proj/a/node_modules", NormalizedPath: "/proj/a/node_modules"},
		{ID: "res-nm-2", Name: "node_modules", Type: domain.ResourceTypeNodeModules, Version: "",
			DisplayPath: "/proj/b/node_modules", NormalizedPath: "/proj/b/node_modules"},
	}}
}

func testProjects() *fakeProjectRepository {
	return &fakeProjectRepository{byID: []domain.BuildProject{
		{ID: "proj-game", Name: "GameClient", Type: domain.ProjectTypeMSBuildCpp,
			RootPath: "/proj/game", NormalizedRootPath: "/proj/game",
			ManifestPath: "/proj/game/GameClient.vcxproj", NormalizedManifestPath: "/proj/game/GameClient.vcxproj"},
	}}
}

func TestResolveTargetByResourceTypeAndVersion(t *testing.T) {
	got, err := resolveTarget(context.Background(), testResources(), testProjects(), "windows-sdk:10.0.22621.0")
	if err != nil {
		t.Fatalf("resolveTarget() error = %v", err)
	}
	if got.Kind != targetKindResource || got.Resource.ID != "res-sdk-1" {
		t.Fatalf("resolveTarget() = %+v, want res-sdk-1", got)
	}
}

func TestResolveTargetByResourceTypeAndVersionNotFound(t *testing.T) {
	_, err := resolveTarget(context.Background(), testResources(), testProjects(), "windows-sdk:9.9.9.9")
	if !errors.Is(err, ErrTargetNotFound) {
		t.Fatalf("resolveTarget() error = %v, want ErrTargetNotFound", err)
	}
}

func TestResolveTargetByProjectPrefixPath(t *testing.T) {
	got, err := resolveTarget(context.Background(), testResources(), testProjects(), `project:/proj/game`)
	if err != nil {
		t.Fatalf("resolveTarget() error = %v", err)
	}
	if got.Kind != targetKindProject || got.Project.ID != "proj-game" {
		t.Fatalf("resolveTarget() = %+v, want proj-game", got)
	}
}

func TestResolveTargetByAbsolutePath(t *testing.T) {
	got, err := resolveTarget(context.Background(), testResources(), testProjects(), "/proj/a/node_modules")
	if err != nil {
		t.Fatalf("resolveTarget() error = %v", err)
	}
	if got.Kind != targetKindResource || got.Resource.ID != "res-nm-1" {
		t.Fatalf("resolveTarget() = %+v, want res-nm-1", got)
	}
}

func TestResolveTargetByID(t *testing.T) {
	got, err := resolveTarget(context.Background(), testResources(), testProjects(), "res-nm-2")
	if err != nil {
		t.Fatalf("resolveTarget() error = %v", err)
	}
	if got.Kind != targetKindResource || got.Resource.ID != "res-nm-2" {
		t.Fatalf("resolveTarget() = %+v, want res-nm-2", got)
	}
}

func TestResolveTargetByNameIsAmbiguous(t *testing.T) {
	_, err := resolveTarget(context.Background(), testResources(), testProjects(), "node_modules")
	if !errors.Is(err, ErrTargetAmbiguous) {
		t.Fatalf("resolveTarget() error = %v, want ErrTargetAmbiguous", err)
	}
}

func TestResolveTargetUnknownIsNotFound(t *testing.T) {
	_, err := resolveTarget(context.Background(), testResources(), testProjects(), "nope-does-not-exist")
	if !errors.Is(err, ErrTargetNotFound) {
		t.Fatalf("resolveTarget() error = %v, want ErrTargetNotFound", err)
	}
}
