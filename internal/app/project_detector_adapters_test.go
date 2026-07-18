package app

import (
	"context"
	"os"
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
