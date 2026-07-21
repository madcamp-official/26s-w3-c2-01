package xcodeproj

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/scanner"
)

func TestDetectorCanDetectMatchesXcodeprojDirectoriesOnly(t *testing.T) {
	d := Detector{}
	if !d.CanDetect(scanner.Entry{Path: "/repo/MyApp.xcodeproj", Kind: scanner.EntryDirectory}) {
		t.Error("want match for a .xcodeproj directory")
	}
	if d.CanDetect(scanner.Entry{Path: "/repo/MyApp.xcodeproj", Kind: scanner.EntryFile}) {
		t.Error("want no match for a file named .xcodeproj")
	}
	if d.CanDetect(scanner.Entry{Path: "/repo/MyApp", Kind: scanner.EntryDirectory}) {
		t.Error("want no match for a plain directory")
	}
}

func TestDetectorDetectRootsAtParentOfBundle(t *testing.T) {
	root := t.TempDir()
	bundle := filepath.Join(root, "MyApp.xcodeproj")
	if err := os.Mkdir(bundle, 0o755); err != nil {
		t.Fatal(err)
	}
	modifiedAt := time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)

	project, err := (Detector{}).Detect(context.Background(), scanner.Entry{Path: bundle, Kind: scanner.EntryDirectory, ModifiedAt: modifiedAt})
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if project.Type != domain.ProjectTypeXcode || project.Name != "MyApp" || project.RootPath != root {
		t.Fatalf("project = %#v", project)
	}
	if project.ManifestPath != filepath.Join(bundle, "project.pbxproj") {
		t.Errorf("ManifestPath = %q", project.ManifestPath)
	}
	if !project.LastModifiedAt.Equal(modifiedAt) {
		t.Errorf("LastModifiedAt = %v, want %v", project.LastModifiedAt, modifiedAt)
	}
}
