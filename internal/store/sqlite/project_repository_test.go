package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/app"
	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

func TestProjectRepositoryUpsertsBatchAndFindsIdentity(t *testing.T) {
	db, scanID := newProjectStore(t)
	repository := NewProjectRepository(db)
	project := preparedProject(t, "Client.vcxproj", domain.ProjectTypeMSBuildCpp)
	if err := repository.UpsertObserved(context.Background(), scanID, []domain.BuildProject{project}); err != nil {
		t.Fatalf("UpsertObserved() error = %v", err)
	}

	project.Name = "updated-client"
	if err := repository.UpsertObserved(context.Background(), scanID, []domain.BuildProject{project}); err != nil {
		t.Fatalf("UpsertObserved(update) error = %v", err)
	}
	byID, err := repository.FindByID(context.Background(), project.ID)
	if err != nil || byID.Name != project.Name {
		t.Fatalf("FindByID() = %#v, %v", byID, err)
	}
	byManifest, err := repository.FindByManifestPath(context.Background(), project.Type, project.ManifestPath)
	if err != nil || byManifest.ID != project.ID {
		t.Fatalf("FindByManifestPath() = %#v, %v", byManifest, err)
	}
}

func TestProjectRepositoryListReturnsAllObservedProjects(t *testing.T) {
	db, scanID := newProjectStore(t)
	repository := NewProjectRepository(db)
	projects := []domain.BuildProject{
		preparedProject(t, "One.csproj", domain.ProjectTypeMSBuildDotNet),
		preparedProject(t, "Two.vcxproj", domain.ProjectTypeMSBuildCpp),
	}
	if err := repository.UpsertObserved(context.Background(), scanID, projects); err != nil {
		t.Fatalf("UpsertObserved() error = %v", err)
	}

	listed, err := repository.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("List() returned %d projects, want 2", len(listed))
	}
}

func TestProjectRepositoryReplacesAndSuppressesGitFallbackAtSameRoot(t *testing.T) {
	db, scanID := newProjectStore(t)
	repository := NewProjectRepository(db)
	root := t.TempDir()
	now := time.Now().UTC()
	prepare := func(name string, projectType domain.ProjectType) domain.BuildProject {
		manifest := filepath.Join(root, name)
		if err := os.WriteFile(manifest, []byte("marker"), 0o600); err != nil {
			t.Fatal(err)
		}
		project, err := app.PrepareBuildProject(domain.BuildProject{Name: "repo", Type: projectType, RootPath: root, ManifestPath: manifest}, now)
		if err != nil {
			t.Fatal(err)
		}
		project.SizeKnown = true
		return project
	}
	gitProject := prepare(".git", domain.ProjectTypeGit)
	goProject := prepare("go.mod", domain.ProjectTypeGo)
	if err := repository.UpsertObserved(context.Background(), scanID, []domain.BuildProject{gitProject}); err != nil {
		t.Fatal(err)
	}
	if err := repository.UpsertObserved(context.Background(), scanID, []domain.BuildProject{goProject}); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.FindByID(context.Background(), gitProject.ID); !errors.Is(err, ErrProjectNotFound) {
		t.Fatalf("Git fallback still exists: %v", err)
	}
	if err := repository.UpsertObserved(context.Background(), scanID, []domain.BuildProject{gitProject}); err != nil {
		t.Fatal(err)
	}
	listed, err := repository.List(context.Background())
	if err != nil || len(listed) != 1 || listed[0].Type != domain.ProjectTypeGo {
		t.Fatalf("List() = %#v, %v", listed, err)
	}
}

func TestProjectRepositoryBatchRollsBackOnMissingScan(t *testing.T) {
	db, _ := newProjectStore(t)
	repository := NewProjectRepository(db)
	projects := []domain.BuildProject{
		preparedProject(t, "One.csproj", domain.ProjectTypeMSBuildDotNet),
		preparedProject(t, "Two.csproj", domain.ProjectTypeMSBuildDotNet),
	}
	if err := repository.UpsertObserved(context.Background(), "missing-scan", projects); err == nil {
		t.Fatal("UpsertObserved() error = nil, want foreign key error")
	}
	_, err := repository.FindByID(context.Background(), projects[0].ID)
	if !errors.Is(err, ErrProjectNotFound) {
		t.Fatalf("FindByID() error = %v, want rollback", err)
	}
}

func TestWorkspaceRepositoryReplacesMembersAtomically(t *testing.T) {
	db, scanID := newProjectStore(t)
	projectRepository := NewProjectRepository(db)
	first := preparedProject(t, "First.vcxproj", domain.ProjectTypeMSBuildCpp)
	second := preparedProject(t, "Second.vcxproj", domain.ProjectTypeMSBuildCpp)
	if err := projectRepository.UpsertObserved(context.Background(), scanID, []domain.BuildProject{first, second}); err != nil {
		t.Fatalf("UpsertObserved() error = %v", err)
	}
	workspace, err := app.PrepareWorkspace(domain.Workspace{
		Name: "Game", Type: domain.WorkspaceTypeVSSolution,
		ManifestPath: filepath.Join(t.TempDir(), "Game.sln"),
	}, time.Now())
	if err != nil {
		t.Fatalf("PrepareWorkspace() error = %v", err)
	}
	repository := NewWorkspaceRepository(db)
	if err := repository.Upsert(context.Background(), scanID, workspace); err != nil {
		t.Fatalf("Upsert(workspace) error = %v", err)
	}
	if err := repository.ReplaceMembers(context.Background(), workspace.ID, []string{first.ID, second.ID, first.ID}); err != nil {
		t.Fatalf("ReplaceMembers() error = %v", err)
	}
	assertMemberCount(t, db, workspace.ID, 2)

	if err := repository.ReplaceMembers(context.Background(), workspace.ID, []string{"missing-project"}); err == nil {
		t.Fatal("ReplaceMembers(missing) error = nil, want foreign key error")
	}
	assertMemberCount(t, db, workspace.ID, 2)
}

func newProjectStore(t *testing.T) (*sql.DB, string) {
	t.Helper()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	scanID := "scan-projects"
	if err := NewScanRepository(db).Save(context.Background(), app.ScanRecord{
		ID: scanID, StartedAt: time.Now(), Roots: []string{t.TempDir()}, Status: app.ScanStatusRunning,
	}); err != nil {
		t.Fatalf("Save(scan) error = %v", err)
	}
	return db, scanID
}

func preparedProject(t *testing.T, manifestName string, projectType domain.ProjectType) domain.BuildProject {
	t.Helper()
	root := t.TempDir()
	project, err := app.PrepareBuildProject(domain.BuildProject{
		Name: manifestName, Type: projectType, RootPath: root,
		ManifestPath: filepath.Join(root, manifestName),
	}, time.Now())
	if err != nil {
		t.Fatalf("PrepareBuildProject() error = %v", err)
	}
	return project
}

func assertMemberCount(t *testing.T, db *sql.DB, workspaceID string, want int) {
	t.Helper()
	var got int
	if err := db.QueryRow("SELECT COUNT(*) FROM workspace_projects WHERE workspace_id = ?", workspaceID).Scan(&got); err != nil {
		t.Fatalf("count workspace members: %v", err)
	}
	if got != want {
		t.Fatalf("workspace member count = %d, want %d", got, want)
	}
}
