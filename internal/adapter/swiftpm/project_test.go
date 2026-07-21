package swiftpm

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/scanner"
)

func TestDetectorDetectsPackageSwift(t *testing.T) {
	root := t.TempDir()
	manifest := filepath.Join(root, "Package.swift")
	if err := os.WriteFile(manifest, []byte("// swift-tools-version:5.9\nimport PackageDescription\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	d := Detector{}
	if !d.CanDetect(scanner.Entry{Path: manifest, Kind: scanner.EntryFile}) {
		t.Fatal("want CanDetect true for Package.swift")
	}
	project, err := d.Detect(context.Background(), scanner.Entry{Path: manifest, Kind: scanner.EntryFile})
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if project.Type != domain.ProjectTypeSwiftPM || project.RootPath != root || project.ManifestPath != manifest {
		t.Fatalf("project = %#v", project)
	}
}

func TestToolsVersionParsesDeclaredComment(t *testing.T) {
	root := t.TempDir()
	manifest := filepath.Join(root, "Package.swift")
	if err := os.WriteFile(manifest, []byte("// swift-tools-version:5.9\nimport PackageDescription\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := ToolsVersion(manifest); got != "5.9" {
		t.Fatalf("ToolsVersion() = %q, want %q", got, "5.9")
	}
}

func TestToolsVersionEmptyWhenMissing(t *testing.T) {
	root := t.TempDir()
	manifest := filepath.Join(root, "Package.swift")
	if err := os.WriteFile(manifest, []byte("import PackageDescription\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := ToolsVersion(manifest); got != "" {
		t.Fatalf("ToolsVersion() = %q, want empty", got)
	}
}

func TestDetectArtifactsReportsBuildDirRegenerableWithLockfile(t *testing.T) {
	root := t.TempDir()
	buildDir := filepath.Join(root, ".build")
	if err := os.Mkdir(buildDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Package.resolved"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := DetectArtifacts(root)
	if err != nil {
		t.Fatalf("DetectArtifacts() error = %v", err)
	}
	if len(got) != 1 || got[0].Type != domain.ResourceTypeBuildOutput || !got[0].Regenerable {
		t.Fatalf("got %#v", got)
	}
	if got[0].RegenerationCommand != "swift build" {
		t.Errorf("RegenerationCommand = %q", got[0].RegenerationCommand)
	}
}

func TestDetectArtifactsReturnsNothingWithoutBuildDir(t *testing.T) {
	root := t.TempDir()
	got, err := DetectArtifacts(root)
	if err != nil || len(got) != 0 {
		t.Fatalf("got %#v, %v", got, err)
	}
}
