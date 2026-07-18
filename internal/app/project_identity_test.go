package app

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

func TestPrepareBuildProjectCreatesStableIdentity(t *testing.T) {
	root := t.TempDir()
	observedAt := time.Date(2026, 7, 18, 9, 0, 0, 0, time.UTC)
	got, err := PrepareBuildProject(domain.BuildProject{
		Name:         "client",
		Type:         domain.ProjectTypeMSBuildCpp,
		RootPath:     root,
		ManifestPath: filepath.Join(root, "Client.vcxproj"),
	}, observedAt)
	if err != nil {
		t.Fatalf("PrepareBuildProject() error = %v", err)
	}
	if got.ID == "" || got.NormalizedRootPath == "" || got.NormalizedManifestPath == "" {
		t.Fatalf("PrepareBuildProject() = %#v, want complete identity", got)
	}
	if got.ID != domain.ProjectID(got.Type, got.NormalizedManifestPath) {
		t.Fatalf("project ID = %q, want stable ID", got.ID)
	}
	if got.Status != domain.ProjectStatusActive || !got.LastObservedAt.Equal(observedAt) {
		t.Fatalf("project status/time = %q/%v", got.Status, got.LastObservedAt)
	}
}

func TestPrepareBuildProjectRejectsManifestOutsideRoot(t *testing.T) {
	_, err := PrepareBuildProject(domain.BuildProject{
		Name:         "client",
		Type:         domain.ProjectTypeNode,
		RootPath:     filepath.Join(t.TempDir(), "project"),
		ManifestPath: filepath.Join(t.TempDir(), "package.json"),
	}, time.Now())
	if err == nil {
		t.Fatal("PrepareBuildProject() error = nil, want outside-root error")
	}
}

func TestPrepareWorkspaceCreatesStableIdentity(t *testing.T) {
	manifest := filepath.Join(t.TempDir(), "Game.sln")
	got, err := PrepareWorkspace(domain.Workspace{
		Name: "Game", Type: domain.WorkspaceTypeVSSolution, ManifestPath: manifest,
	}, time.Now())
	if err != nil {
		t.Fatalf("PrepareWorkspace() error = %v", err)
	}
	if got.ID != domain.WorkspaceID(got.Type, got.NormalizedManifestPath) {
		t.Fatalf("workspace ID = %q, want stable ID", got.ID)
	}
}
