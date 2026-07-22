package node

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/scanner"
)

func TestFilesystemDetector_CanDetect(t *testing.T) {
	cases := []struct {
		name string
		dir  string
		want bool
	}{
		{"basic fixture has package.json", "../../../testdata/node/basic", true},
		{"missing-lockfile fixture still has package.json", "../../../testdata/node/missing-lockfile", true},
		{"malformed manifest fixture still has package.json", "../../../testdata/node/malformed-package-json", true},
		{"directory without package.json", t.TempDir(), false},
	}

	var detector Detector = FilesystemDetector{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := detector.CanDetect(scanner.Entry{Path: tc.dir}); got != tc.want {
				t.Errorf("CanDetect(%q) = %v, want %v", tc.dir, got, tc.want)
			}
		})
	}
}

// TestFilesystemDetector_CanDetect_SkipsVendoredPackages reproduces issue #36:
// package.json files under node_modules are installed dependencies, not
// projects, and must not be detected as project roots -- while the real
// project roots around them still are.
func TestFilesystemDetector_CanDetect_SkipsVendoredPackages(t *testing.T) {
	root := t.TempDir()
	writeManifest := func(dir string) {
		t.Helper()
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, manifestFile), []byte(`{"name":"x"}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	frontend := filepath.Join(root, "frontend")
	member := filepath.Join(frontend, "packages", "app")
	vendored := filepath.Join(frontend, "node_modules", "react")
	vendoredNested := filepath.Join(frontend, "node_modules", "resolve", "test", "fixture")
	for _, dir := range []string{frontend, member, vendored, vendoredNested} {
		writeManifest(dir)
	}

	var detector Detector = FilesystemDetector{}
	cases := []struct {
		dir  string
		want bool
	}{
		{frontend, true},
		{member, true},
		{filepath.Join(frontend, "node_modules"), false},
		{vendored, false},
		{vendoredNested, false},
	}
	for _, tc := range cases {
		if got := detector.CanDetect(scanner.Entry{Path: tc.dir}); got != tc.want {
			t.Errorf("CanDetect(%q) = %v, want %v", tc.dir, got, tc.want)
		}
	}
}

// TestFilesystemDetector_CanDetect_SkipsUnityPackageManifests reproduces a
// scan false positive: Unity's Library/PackageCache/com.unity.* directories
// ship a package.json (Unity's own package-manager format, keyed by a
// "unity" field) that isn't an npm manifest and must not be detected as a
// Node project.
func TestFilesystemDetector_CanDetect_SkipsUnityPackageManifests(t *testing.T) {
	unityDir := t.TempDir()
	unityManifest := `{"name":"com.unity.ads","version":"2.0.8","unity":"2018.1","displayName":"Advertisement"}`
	if err := os.WriteFile(filepath.Join(unityDir, manifestFile), []byte(unityManifest), 0o644); err != nil {
		t.Fatal(err)
	}

	npmDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(npmDir, manifestFile), []byte(`{"name":"real-app"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	var detector Detector = FilesystemDetector{}
	if got := detector.CanDetect(scanner.Entry{Path: unityDir}); got {
		t.Errorf("CanDetect(unity package.json) = %v, want false", got)
	}
	if got := detector.CanDetect(scanner.Entry{Path: npmDir}); !got {
		t.Errorf("CanDetect(npm package.json) = %v, want true", got)
	}
}

func TestIsVendoredPath(t *testing.T) {
	j := filepath.Join
	cases := []struct {
		path string
		want bool
	}{
		{j("a", "b", "frontend"), false},
		{j("a", "node_modules"), true},
		{j("a", "node_modules", "react"), true},
		{j("a", "node_modules", "x", "node_modules", "y"), true},
		{j("a", "node_modules-cache", "pkg"), false},
	}
	for _, tc := range cases {
		if got := isVendoredPath(tc.path); got != tc.want {
			t.Errorf("isVendoredPath(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestFilesystemDetector_Detect(t *testing.T) {
	var detector Detector = FilesystemDetector{}
	modifiedAt := time.Date(2026, 7, 18, 3, 4, 5, 0, time.UTC)

	got, err := detector.Detect(context.Background(), scanner.Entry{Path: "../../../testdata/node/basic", ModifiedAt: modifiedAt})
	if err != nil {
		t.Fatalf("Detect returned error: %v", err)
	}
	if got.Name != "sample-app" {
		t.Errorf("Name = %q, want %q", got.Name, "sample-app")
	}
	if got.Type != domain.ProjectTypeNode {
		t.Errorf("Type = %v, want %v", got.Type, domain.ProjectTypeNode)
	}
	if !got.LastModifiedAt.Equal(modifiedAt) {
		t.Errorf("LastModifiedAt = %v, want scanner entry time %v", got.LastModifiedAt, modifiedAt)
	}
}

func TestFilesystemDetector_Detect_FallsBackToDirectoryNameWhenManifestNameIsEmpty(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, manifestFile), []byte(`{"version":"1.0.0"}`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var detector Detector = FilesystemDetector{}
	got, err := detector.Detect(context.Background(), scanner.Entry{Path: dir})
	if err != nil {
		t.Fatalf("Detect returned error: %v", err)
	}
	if got.Name != filepath.Base(dir) {
		t.Errorf("Name = %q, want directory basename %q", got.Name, filepath.Base(dir))
	}
}

func TestFilesystemDetector_Detect_MalformedManifestIsRecoverableError(t *testing.T) {
	var detector Detector = FilesystemDetector{}

	_, err := detector.Detect(context.Background(), scanner.Entry{Path: "../../../testdata/node/malformed-package-json"})
	if err == nil {
		t.Fatal("Detect() error = nil, want an error for malformed package.json")
	}
}

func TestDetectArtifacts_WithLockfile(t *testing.T) {
	got, err := DetectArtifacts("../../../testdata/node/basic")
	if err != nil {
		t.Fatalf("DetectArtifacts returned error: %v", err)
	}

	byType := map[domain.ResourceType]domain.Resource{}
	for _, resource := range got {
		byType[resource.Type] = resource
	}

	nodeModules, ok := byType[domain.ResourceTypeNodeModules]
	if !ok {
		t.Fatal("DetectArtifacts did not report node_modules")
	}
	if !nodeModules.Regenerable {
		t.Error("node_modules with a lockfile should be Regenerable")
	}
	if nodeModules.Confidence != confidenceDeclaredNodeModules {
		t.Errorf("node_modules Confidence = %d, want %d", nodeModules.Confidence, confidenceDeclaredNodeModules)
	}
	// testdata/node/basic has a package-lock.json, so the npm-specific
	// reinstall command should be recorded, not a generic hint.
	if nodeModules.RegenerationCommand != "npm ci" {
		t.Errorf("node_modules RegenerationCommand = %q, want %q", nodeModules.RegenerationCommand, "npm ci")
	}

	buildOutput, ok := byType[domain.ResourceTypeBuildOutput]
	if !ok {
		t.Fatal("DetectArtifacts did not report the dist build output")
	}
	if buildOutput.Name != "dist" {
		t.Errorf("build output Name = %q, want %q", buildOutput.Name, "dist")
	}
	// testdata/node/basic/package.json declares no "build" script, so there
	// is no evidence anything would regenerate dist -- see
	// TestDetectArtifacts_BuildOutputRegenerableOnlyWithBuildScript.
	if buildOutput.Regenerable {
		t.Error("dist build output should not be Regenerable without a declared build script")
	}
}

func TestDetectArtifacts_BuildOutputRegenerableOnlyWithBuildScript(t *testing.T) {
	withScript := t.TempDir()
	writeFixturePackageJSON(t, withScript, `{"name":"app","scripts":{"build":"tsc"}}`)
	if err := os.Mkdir(filepath.Join(withScript, "dist"), 0o755); err != nil {
		t.Fatal(err)
	}

	withoutScript := t.TempDir()
	writeFixturePackageJSON(t, withoutScript, `{"name":"app"}`)
	if err := os.Mkdir(filepath.Join(withoutScript, "dist"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := DetectArtifacts(withScript)
	if err != nil {
		t.Fatalf("DetectArtifacts(withScript) error = %v", err)
	}
	if len(got) != 1 || !got[0].Regenerable {
		t.Fatalf("DetectArtifacts(withScript) = %+v, want dist Regenerable = true", got)
	}

	got, err = DetectArtifacts(withoutScript)
	if err != nil {
		t.Fatalf("DetectArtifacts(withoutScript) error = %v", err)
	}
	if len(got) != 1 || got[0].Regenerable {
		t.Fatalf("DetectArtifacts(withoutScript) = %+v, want dist Regenerable = false", got)
	}
}

func writeFixturePackageJSON(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, manifestFile), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDetectArtifacts_MissingLockfileIsNotRegenerable(t *testing.T) {
	got, err := DetectArtifacts("../../../testdata/node/missing-lockfile")
	if err != nil {
		t.Fatalf("DetectArtifacts returned error: %v", err)
	}
	if len(got) != 1 || got[0].Type != domain.ResourceTypeNodeModules {
		t.Fatalf("DetectArtifacts = %+v, want exactly one node_modules candidate", got)
	}
	if got[0].Regenerable {
		t.Error("node_modules without a lockfile should not be Regenerable")
	}
	if got[0].Confidence != confidenceInferredNodeModules {
		t.Errorf("Confidence = %d, want %d", got[0].Confidence, confidenceInferredNodeModules)
	}
	if got[0].RegenerationCommand != "" {
		t.Errorf("RegenerationCommand = %q, want empty (no lockfile means no known install command)", got[0].RegenerationCommand)
	}
}

func TestDetectArtifacts_RegenerationCommandMatchesDetectedPackageManager(t *testing.T) {
	cases := []struct {
		lockfile    string
		wantInstall string
		wantBuild   string
	}{
		{"package-lock.json", "npm ci", "npm run build"},
		{"yarn.lock", "yarn install", "yarn run build"},
		{"pnpm-lock.yaml", "pnpm install", "pnpm run build"},
	}
	for _, tc := range cases {
		t.Run(tc.lockfile, func(t *testing.T) {
			root := t.TempDir()
			writeFixturePackageJSON(t, root, `{"name":"app","scripts":{"build":"tsc"}}`)
			if err := os.WriteFile(filepath.Join(root, tc.lockfile), []byte("{}"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(filepath.Join(root, "node_modules"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(filepath.Join(root, "dist"), 0o755); err != nil {
				t.Fatal(err)
			}

			got, err := DetectArtifacts(root)
			if err != nil {
				t.Fatalf("DetectArtifacts() error = %v", err)
			}
			byType := map[domain.ResourceType]domain.Resource{}
			for _, r := range got {
				byType[r.Type] = r
			}
			if cmd := byType[domain.ResourceTypeNodeModules].RegenerationCommand; cmd != tc.wantInstall {
				t.Errorf("node_modules RegenerationCommand = %q, want %q", cmd, tc.wantInstall)
			}
			if cmd := byType[domain.ResourceTypeBuildOutput].RegenerationCommand; cmd != tc.wantBuild {
				t.Errorf("dist RegenerationCommand = %q, want %q", cmd, tc.wantBuild)
			}
		})
	}
}

func TestDetectArtifacts_NoArtifacts(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, manifestFile), []byte(`{"name":"empty"}`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := DetectArtifacts(dir)
	if err != nil {
		t.Fatalf("DetectArtifacts returned error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("DetectArtifacts = %+v, want no candidates", got)
	}
}
