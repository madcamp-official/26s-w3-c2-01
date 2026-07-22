package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	gitadapter "github.com/madcamp-official/26s-w3-c2-01/internal/adapter/git"
	"github.com/madcamp-official/26s-w3-c2-01/internal/adapter/msbuild"
	nodeadapter "github.com/madcamp-official/26s-w3-c2-01/internal/adapter/node"
	pythonadapter "github.com/madcamp-official/26s-w3-c2-01/internal/adapter/python"
	swiftpmadapter "github.com/madcamp-official/26s-w3-c2-01/internal/adapter/swiftpm"
	xcodeprojadapter "github.com/madcamp-official/26s-w3-c2-01/internal/adapter/xcodeproj"
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

func TestNodeProjectDetectorReportsWorkspaceAndDeclaredMembers(t *testing.T) {
	root := t.TempDir()
	member := filepath.Join(root, "packages", "app")
	if err := os.MkdirAll(member, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"name":"web","workspaces":["packages/*"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(member, "package.json"), []byte(`{"name":"app"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	got := (NodeProjectDetector{Detector: nodeadapter.FilesystemDetector{}}).
		Observe(context.Background(), scanner.Entry{Path: root})
	if len(got.Items) != 1 || got.Items[0].Workspace == nil {
		t.Fatalf("Observe() = %#v, want Node workspace", got)
	}
	workspace := got.Items[0].Workspace
	if workspace.Type != domain.WorkspaceTypeNode || workspace.ManifestPath != filepath.Join(root, "package.json") {
		t.Fatalf("Workspace = %#v", workspace)
	}
	wantMember := filepath.Join(member, "package.json")
	if len(got.Items[0].WorkspaceProjectPaths) != 1 || got.Items[0].WorkspaceProjectPaths[0] != wantMember {
		t.Fatalf("WorkspaceProjectPaths = %v, want [%s]", got.Items[0].WorkspaceProjectPaths, wantMember)
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
	var _ ProjectDetector = PythonProjectDetector{Detector: pythonadapter.FilesystemDetector{}}
}

func TestPythonProjectDetectorAdaptsProjectFact(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "pyproject.toml"), []byte("[project]\nname=\"svc\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	modifiedAt := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	got := (PythonProjectDetector{Detector: pythonadapter.FilesystemDetector{}}).
		Observe(context.Background(), scanner.Entry{Path: root, ModifiedAt: modifiedAt})
	if len(got.Items) != 1 || len(got.Items[0].Projects) != 1 || len(got.Issues) != 0 {
		t.Fatalf("Observe() = %#v", got)
	}
	project := got.Items[0].Projects[0]
	if project.Type != domain.ProjectTypePython || !project.LastModifiedAt.Equal(modifiedAt) {
		t.Fatalf("project = %#v", project)
	}
}

func TestPythonProjectDetectorReportsVenvAndCacheAsProjectResources(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "poetry.lock"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "pyproject.toml"), []byte("[project]\nname=\"svc\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	venv := filepath.Join(root, ".venv")
	if err := os.MkdirAll(venv, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(venv, "pyvenv.cfg"), []byte("home = /usr/bin\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "__pycache__"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := (PythonProjectDetector{Detector: pythonadapter.FilesystemDetector{}}).
		Observe(context.Background(), scanner.Entry{Path: root})
	if len(got.Items) != 1 || len(got.Issues) != 0 {
		t.Fatalf("Observe() = %#v", got)
	}
	resources := got.Items[0].ProjectResources
	if len(resources) != 2 {
		t.Fatalf("ProjectResources = %#v, want venv + cache candidates", resources)
	}
	manifest := filepath.Join(root, "pyproject.toml")
	for _, r := range resources {
		if r.OwnerManifestPath != manifest {
			t.Errorf("OwnerManifestPath = %q, want %q", r.OwnerManifestPath, manifest)
		}
		if r.Resource.Type == domain.ResourceTypeVenv && !r.Resource.Regenerable {
			t.Errorf("venv resource = %#v, want Regenerable=true (poetry.lock is DECLARED-strength)", r.Resource)
		}
	}
}

func TestPythonProjectDetectorLinksDeclaredCondaEnvironment(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "pyproject.toml"), []byte("[project]\nname=\"svc\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "environment.yml"), []byte("name: svc-env\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := (PythonProjectDetector{Detector: pythonadapter.FilesystemDetector{}}).
		Observe(context.Background(), scanner.Entry{Path: root})
	if len(got.Items) != 1 {
		t.Fatalf("Observe() = %#v", got)
	}
	properties := got.Items[0].ProjectProperties
	if len(properties) != 1 || properties[0].Name != condaEnvPropertyName || properties[0].Value != "svc-env" {
		t.Fatalf("ProjectProperties = %#v, want one conda-env=svc-env property", properties)
	}
}

func TestPythonProjectDetectorReportsLocalCondaPrefixEnvAsOwnedWithoutCleanupEvidence(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "pyproject.toml"), []byte("[project]\nname=\"svc\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	envDir := filepath.Join(root, "envs", "conda-meta")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(envDir, "history"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	got := (PythonProjectDetector{Detector: pythonadapter.FilesystemDetector{}}).
		Observe(context.Background(), scanner.Entry{Path: root})
	if len(got.Items) != 1 {
		t.Fatalf("Observe() = %#v", got)
	}
	var condaResource *ProjectResourceCandidate
	for i := range got.Items[0].ProjectResources {
		if got.Items[0].ProjectResources[i].Resource.Type == domain.ResourceTypeCondaEnv {
			condaResource = &got.Items[0].ProjectResources[i]
		}
	}
	if condaResource == nil {
		t.Fatalf("ProjectResources = %#v, want a local conda prefix env (OWNS)", got.Items[0].ProjectResources)
	}
	// 결정 4: a conda environment is never a cleanup candidate even when
	// project-owned, so its CleanupEvidence is deliberately left zero-value
	// -- projectArtifactCleanupEvidence is never called for it.
	if condaResource.Cleanup != (CleanupEvidence{}) {
		t.Errorf("Cleanup = %#v, want zero-value (결정 4: conda envs never enter the SAFE path)", condaResource.Cleanup)
	}
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
	// resolve to true. KnownOutputPath stays false: nothing in this bare
	// <Project></Project> manifest declares OutDir/OutputPath, so bin/obj
	// were only ever found by name match (INFERRED), not confirmed by
	// project config (see TestMSBuildProjectDetectorTrustsDeclaredOutputPath).
	want := CleanupEvidence{ProjectOwned: true, KnownOutputPath: false, ReparsePointFree: true, GitTrackedOriginalsAbsent: true}
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

func TestMSBuildProjectDetectorTrustsDeclaredOutputPath(t *testing.T) {
	root := t.TempDir()
	manifest := filepath.Join(root, "App.vcxproj")
	if err := os.WriteFile(manifest, []byte(`<Project></Project>`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "Build"), 0o755); err != nil {
		t.Fatal(err)
	}
	parser := buildProjectParserFake{parsed: []msbuild.ParsedBuildProject{{
		Project:  domain.BuildProject{ManifestPath: manifest, RootPath: root},
		Declared: []msbuild.DeclaredProperty{{Name: "OutDir", Value: `Build\`}},
	}}}

	got := (MSBuildProjectDetector{Parser: parser}).Observe(context.Background(), scanner.Entry{Path: manifest})
	resources := got.Items[0].ProjectResources
	if len(resources) != 1 || resources[0].Resource.Name != "Build" {
		t.Fatalf("ProjectResources = %#v, want the declared Build dir", resources)
	}
	if !resources[0].Cleanup.KnownOutputPath {
		t.Errorf("Cleanup.KnownOutputPath = false, want true (OutDir was declared and resolved to a real directory)")
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

func TestMSBuildProjectDetectorSetsRegenerationCommandByProjectType(t *testing.T) {
	cases := []struct {
		name        string
		projectType domain.ProjectType
		manifest    string
		wantCommand string
	}{
		{"dotnet", domain.ProjectTypeMSBuildDotNet, "App.csproj", `dotnet build "%s"`},
		{"cpp", domain.ProjectTypeMSBuildCpp, "App.vcxproj", `msbuild "%s"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			manifest := filepath.Join(root, tc.manifest)
			if err := os.WriteFile(manifest, []byte(`<Project></Project>`), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(filepath.Join(root, "bin"), 0o755); err != nil {
				t.Fatal(err)
			}
			parser := buildProjectParserFake{parsed: []msbuild.ParsedBuildProject{{
				Project: domain.BuildProject{ManifestPath: manifest, RootPath: root, Type: tc.projectType},
			}}}

			got := (MSBuildProjectDetector{Parser: parser}).Observe(context.Background(), scanner.Entry{Path: manifest})
			resources := got.Items[0].ProjectResources
			if len(resources) != 1 {
				t.Fatalf("ProjectResources = %#v, want the bin dir", resources)
			}
			want := fmt.Sprintf(tc.wantCommand, manifest)
			if resources[0].Resource.RegenerationCommand != want {
				t.Errorf("RegenerationCommand = %q, want %q", resources[0].Resource.RegenerationCommand, want)
			}
		})
	}
}

func TestXcodeProjectDetectorAdaptsProjectFact(t *testing.T) {
	root := t.TempDir()
	bundle := filepath.Join(root, "MyApp.xcodeproj")
	if err := os.Mkdir(bundle, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundle, "project.pbxproj"), []byte("// pbxproj\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := (XcodeProjectDetector{Detector: xcodeprojadapter.Detector{}}).
		Observe(context.Background(), scanner.Entry{Path: bundle, Kind: scanner.EntryDirectory})
	if len(got.Items) != 1 || len(got.Items[0].Projects) != 1 || len(got.Issues) != 0 {
		t.Fatalf("Observe() = %#v", got)
	}
	if got.Items[0].Projects[0].Type != domain.ProjectTypeXcode {
		t.Fatalf("project = %#v", got.Items[0].Projects[0])
	}
}

func TestXcodeProjectDetectorReportsMalformedBundleAsIssue(t *testing.T) {
	root := t.TempDir()
	bundle := filepath.Join(root, "Backup.xcodeproj") // no project.pbxproj inside
	if err := os.Mkdir(bundle, 0o755); err != nil {
		t.Fatal(err)
	}

	got := (XcodeProjectDetector{Detector: xcodeprojadapter.Detector{}}).
		Observe(context.Background(), scanner.Entry{Path: bundle, Kind: scanner.EntryDirectory})
	if len(got.Items) != 0 {
		t.Fatalf("Observe() items = %#v, want none for a bundle with no project.pbxproj", got.Items)
	}
	if len(got.Issues) != 1 || got.Issues[0].Code != IssueMalformedManifest {
		t.Fatalf("Observe() issues = %#v, want one malformed-manifest issue", got.Issues)
	}
}

func TestXcodeProjectDetectorReportsPodsAsProjectResource(t *testing.T) {
	root := t.TempDir()
	bundle := filepath.Join(root, "MyApp.xcodeproj")
	if err := os.Mkdir(bundle, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundle, "project.pbxproj"), []byte("// pbxproj\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Podfile"), []byte("platform :ios, '17.0'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Podfile.lock"), []byte("PODS:\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "Pods"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := (XcodeProjectDetector{Detector: xcodeprojadapter.Detector{}}).
		Observe(context.Background(), scanner.Entry{Path: bundle, Kind: scanner.EntryDirectory})
	if len(got.Items) != 1 {
		t.Fatalf("Observe() = %#v", got)
	}
	resources := got.Items[0].ProjectResources
	if len(resources) != 1 || resources[0].Resource.Type != domain.ResourceTypePods {
		t.Fatalf("ProjectResources = %#v, want the Pods dir", resources)
	}
	wantManifest := filepath.Join(bundle, "project.pbxproj")
	if resources[0].OwnerManifestPath != wantManifest {
		t.Errorf("OwnerManifestPath = %q, want %q", resources[0].OwnerManifestPath, wantManifest)
	}
	if !resources[0].Resource.Regenerable {
		t.Error("want Regenerable=true (Podfile.lock is DECLARED-strength)")
	}
}

func TestXcodeWorkspaceDetectorAdaptsWorkspaceAndMembers(t *testing.T) {
	root := t.TempDir()
	workspacePath := filepath.Join(root, "MyApp.xcworkspace")
	if err := os.Mkdir(workspacePath, 0o755); err != nil {
		t.Fatal(err)
	}
	data := `<Workspace version="1.0"><FileRef location="group:MyApp.xcodeproj"></FileRef></Workspace>`
	if err := os.WriteFile(filepath.Join(workspacePath, "contents.xcworkspacedata"), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	got := (XcodeWorkspaceDetector{Detector: xcodeprojadapter.WorkspaceDetector{}}).
		Observe(context.Background(), scanner.Entry{Path: workspacePath, Kind: scanner.EntryDirectory})
	if len(got.Items) != 1 || got.Items[0].Workspace == nil {
		t.Fatalf("Observe() = %#v", got)
	}
	if got.Items[0].Workspace.Type != domain.WorkspaceTypeXcodeWorkspace {
		t.Fatalf("workspace = %#v", got.Items[0].Workspace)
	}
	if len(got.Items[0].WorkspaceProjectPaths) != 1 {
		t.Fatalf("WorkspaceProjectPaths = %#v, want one member", got.Items[0].WorkspaceProjectPaths)
	}
}

func TestSwiftPMProjectDetectorReportsBuildDirAndToolsVersionProperty(t *testing.T) {
	root := t.TempDir()
	manifest := filepath.Join(root, "Package.swift")
	if err := os.WriteFile(manifest, []byte("// swift-tools-version:5.9\nimport PackageDescription\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".build"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Package.resolved"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := (SwiftPMProjectDetector{Detector: swiftpmadapter.Detector{}}).
		Observe(context.Background(), scanner.Entry{Path: manifest, Kind: scanner.EntryFile})
	if len(got.Items) != 1 || len(got.Issues) != 0 {
		t.Fatalf("Observe() = %#v", got)
	}
	candidate := got.Items[0]
	if len(candidate.Projects) != 1 || candidate.Projects[0].Type != domain.ProjectTypeSwiftPM {
		t.Fatalf("Projects = %#v", candidate.Projects)
	}
	if len(candidate.ProjectResources) != 1 || candidate.ProjectResources[0].Resource.Type != domain.ResourceTypeBuildOutput {
		t.Fatalf("ProjectResources = %#v, want the .build dir", candidate.ProjectResources)
	}
	if len(candidate.ProjectProperties) != 1 || candidate.ProjectProperties[0].Name != swiftpmadapter.ToolsVersionPropertyName || candidate.ProjectProperties[0].Value != "5.9" {
		t.Fatalf("ProjectProperties = %#v, want swift-tools-version=5.9", candidate.ProjectProperties)
	}
}

type buildProjectParserFake struct {
	parsed []msbuild.ParsedBuildProject
}

func (buildProjectParserFake) CanParse(scanner.Entry) bool { return true }

func (p buildProjectParserFake) Parse(context.Context, scanner.Entry) ([]msbuild.ParsedBuildProject, error) {
	return p.parsed, nil
}
