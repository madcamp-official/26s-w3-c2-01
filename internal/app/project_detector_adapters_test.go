package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	gitadapter "github.com/madcamp-official/26s-w3-c2-01/internal/adapter/git"
	"github.com/madcamp-official/26s-w3-c2-01/internal/adapter/msbuild"
	nodeadapter "github.com/madcamp-official/26s-w3-c2-01/internal/adapter/node"
	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/scanner"
)

func TestNodeProjectDetectorAdaptsProjectFact(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"name":"web"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	modifiedAt := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	detector := NodeProjectDetector{Detector: nodeadapter.FilesystemDetector{}}
	got := detector.Observe(context.Background(), scanner.Entry{Path: root, ModifiedAt: modifiedAt})
	if len(got.Items) != 1 || len(got.Items[0].Projects) != 1 || len(got.Issues) != 0 {
		t.Fatalf("Observe() = %#v", got)
	}
	if !got.Items[0].Projects[0].LastModifiedAt.Equal(modifiedAt) {
		t.Fatalf("project modified time = %v", got.Items[0].Projects[0].LastModifiedAt)
	}
}

func TestNodeProjectDetectorReportsOwnedArtifactsAsProjectResources(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"name":"web"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "package-lock.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "node_modules"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := (NodeProjectDetector{Detector: nodeadapter.FilesystemDetector{}}).
		Observe(context.Background(), scanner.Entry{Path: root})
	if len(got.Items) != 1 || len(got.Issues) != 0 {
		t.Fatalf("Observe() = %#v", got)
	}
	resources := got.Items[0].ProjectResources
	if len(resources) != 1 {
		t.Fatalf("ProjectResources = %#v, want one node_modules candidate", resources)
	}
	manifest := filepath.Join(root, "package.json")
	if resources[0].OwnerManifestPath != manifest {
		t.Fatalf("OwnerManifestPath = %q, want %q", resources[0].OwnerManifestPath, manifest)
	}
	if resources[0].Resource.Type != domain.ResourceTypeNodeModules {
		t.Fatalf("resource type = %q, want node-modules", resources[0].Resource.Type)
	}
	// root is a plain t.TempDir(): node_modules is a real directory (not a
	// reparse point) and root isn't inside any Git repository, so both
	// checks should resolve to true rather than stay unverified.
	want := CleanupEvidence{ProjectOwned: true, KnownOutputPath: true, ReparsePointFree: true, GitTrackedOriginalsAbsent: true}
	if resources[0].Cleanup != want {
		t.Errorf("Cleanup = %#v, want %#v", resources[0].Cleanup, want)
	}
}

func TestNodeProjectDetectorReturnsStructuredParseIssue(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"name":`), 0o644); err != nil {
		t.Fatal(err)
	}
	got := (NodeProjectDetector{Detector: nodeadapter.FilesystemDetector{}}).
		Observe(context.Background(), scanner.Entry{Path: root})
	if len(got.Items) != 0 || len(got.Issues) != 1 || len(got.Unverified) != 1 {
		t.Fatalf("Observe() = %#v, want structured recoverable issue", got)
	}
	if got.Issues[0].Code != IssueMalformedManifest || got.Issues[0].Adapter != "node" {
		t.Fatalf("issue = %#v", got.Issues[0])
	}
}

func TestGitAndMSBuildAdaptersSatisfyProjectDetector(t *testing.T) {
	var _ ProjectDetector = GitProjectDetector{Detector: gitadapter.FilesystemDetector{}}
	var _ ProjectDetector = MSBuildProjectDetector{Parser: msbuild.XMLBuildProjectParser{}}
}

func TestMSBuildProjectDetectorReportsOwnedArtifactsAsProjectResources(t *testing.T) {
	root := t.TempDir()
	manifest := filepath.Join(root, "App.csproj")
	if err := os.WriteFile(manifest, []byte(`<Project></Project>`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "obj"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := (MSBuildProjectDetector{Parser: msbuild.XMLBuildProjectParser{}}).
		Observe(context.Background(), scanner.Entry{Path: manifest})
	if len(got.Items) != 1 || len(got.Issues) != 0 {
		t.Fatalf("Observe() = %#v", got)
	}
	resources := got.Items[0].ProjectResources
	if len(resources) != 2 {
		t.Fatalf("ProjectResources = %#v, want bin and obj", resources)
	}
	// root is a plain t.TempDir(): bin/obj are real directories (not reparse
	// points) and root isn't inside any Git repository, so both checks
	// should resolve to true rather than stay unverified.
	want := CleanupEvidence{ProjectOwned: true, KnownOutputPath: true, ReparsePointFree: true, GitTrackedOriginalsAbsent: true}
	for _, r := range resources {
		if r.OwnerManifestPath != manifest {
			t.Errorf("OwnerManifestPath = %q, want %q", r.OwnerManifestPath, manifest)
		}
		if r.Resource.Type != domain.ResourceTypeBuildOutput {
			t.Errorf("resource type = %q, want build-output", r.Resource.Type)
		}
		if r.Cleanup != want {
			t.Errorf("Cleanup = %#v, want %#v", r.Cleanup, want)
		}
	}
}

func TestMSBuildProjectDetectorMarksReparsePointArtifactUnverified(t *testing.T) {
	root := t.TempDir()
	realDir := t.TempDir()
	manifest := filepath.Join(root, "App.csproj")
	if err := os.WriteFile(manifest, []byte(`<Project></Project>`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realDir, filepath.Join(root, "bin")); err != nil {
		t.Skipf("creating symlink is not permitted: %v", err)
	}

	got := (MSBuildProjectDetector{Parser: msbuild.XMLBuildProjectParser{}}).
		Observe(context.Background(), scanner.Entry{Path: manifest})
	resources := got.Items[0].ProjectResources
	if len(resources) != 1 {
		t.Fatalf("ProjectResources = %#v, want the symlinked bin only", resources)
	}
	if resources[0].Cleanup.ReparsePointFree {
		t.Errorf("Cleanup.ReparsePointFree = true, want false (bin is a symlink)")
	}
}

func TestMSBuildProjectDetectorMarksTrackedArtifactUnverified(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available on PATH")
	}
	repoRoot := t.TempDir()
	runGit(t, repoRoot, "init", "-q")

	manifest := filepath.Join(repoRoot, "App.csproj")
	if err := os.WriteFile(manifest, []byte(`<Project></Project>`), 0o644); err != nil {
		t.Fatal(err)
	}
	objDir := filepath.Join(repoRoot, "obj")
	if err := os.Mkdir(objDir, 0o755); err != nil {
		t.Fatal(err)
	}
	trackedFile := filepath.Join(objDir, "Licenses.txt")
	if err := os.WriteFile(trackedFile, []byte("original license text"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repoRoot, "add", trackedFile)

	got := (MSBuildProjectDetector{Parser: msbuild.XMLBuildProjectParser{}}).
		Observe(context.Background(), scanner.Entry{Path: manifest})
	resources := got.Items[0].ProjectResources
	if len(resources) != 1 {
		t.Fatalf("ProjectResources = %#v, want the obj directory only", resources)
	}
	if resources[0].Cleanup.GitTrackedOriginalsAbsent {
		t.Errorf("Cleanup.GitTrackedOriginalsAbsent = true, want false (obj contains a tracked file)")
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestMSBuildProjectDetectorPreservesDeclaredProperties(t *testing.T) {
	manifest := filepath.Join(t.TempDir(), "app.csproj")
	parser := buildProjectParserFake{parsed: []msbuild.ParsedBuildProject{{
		Project: domain.BuildProject{ManifestPath: manifest},
		Declared: []msbuild.DeclaredProperty{{
			Name: "TargetFramework", Value: "net8.0", Condition: "'$(Configuration)' == 'Debug'",
		}},
	}}}

	got := (MSBuildProjectDetector{Parser: parser}).Observe(context.Background(), scanner.Entry{Path: manifest})
	if len(got.Items) != 1 || len(got.Items[0].ProjectProperties) != 1 {
		t.Fatalf("Observe() = %#v, want one project property", got)
	}
	property := got.Items[0].ProjectProperties[0]
	if property.OwnerManifestPath != manifest || property.SourcePath != manifest ||
		property.Name != "TargetFramework" || property.Value != "net8.0" ||
		property.Condition != "'$(Configuration)' == 'Debug'" {
		t.Fatalf("property = %#v", property)
	}
}

type buildProjectParserFake struct {
	parsed []msbuild.ParsedBuildProject
}

func (buildProjectParserFake) CanParse(scanner.Entry) bool { return true }

func (p buildProjectParserFake) Parse(context.Context, scanner.Entry) ([]msbuild.ParsedBuildProject, error) {
	return p.parsed, nil
}
