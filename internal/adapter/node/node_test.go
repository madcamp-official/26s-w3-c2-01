package node

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
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
			if got := detector.CanDetect(tc.dir); got != tc.want {
				t.Errorf("CanDetect(%q) = %v, want %v", tc.dir, got, tc.want)
			}
		})
	}
}

func TestFilesystemDetector_Detect(t *testing.T) {
	var detector Detector = FilesystemDetector{}

	got, err := detector.Detect(context.Background(), "../../../testdata/node/basic")
	if err != nil {
		t.Fatalf("Detect returned error: %v", err)
	}
	if got.Name != "sample-app" {
		t.Errorf("Name = %q, want %q", got.Name, "sample-app")
	}
	if got.Type != domain.ProjectTypeNode {
		t.Errorf("Type = %v, want %v", got.Type, domain.ProjectTypeNode)
	}
	if got.LastModifiedAt.IsZero() {
		t.Errorf("LastModifiedAt is zero, want the directory's mod time")
	}
}

func TestFilesystemDetector_Detect_FallsBackToDirectoryNameWhenManifestNameIsEmpty(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, manifestFile), []byte(`{"version":"1.0.0"}`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var detector Detector = FilesystemDetector{}
	got, err := detector.Detect(context.Background(), dir)
	if err != nil {
		t.Fatalf("Detect returned error: %v", err)
	}
	if got.Name != filepath.Base(dir) {
		t.Errorf("Name = %q, want directory basename %q", got.Name, filepath.Base(dir))
	}
}

func TestFilesystemDetector_Detect_MalformedManifestIsRecoverableError(t *testing.T) {
	var detector Detector = FilesystemDetector{}

	_, err := detector.Detect(context.Background(), "../../../testdata/node/malformed-package-json")
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

	buildOutput, ok := byType[domain.ResourceTypeBuildOutput]
	if !ok {
		t.Fatal("DetectArtifacts did not report the dist build output")
	}
	if buildOutput.Name != "dist" {
		t.Errorf("build output Name = %q, want %q", buildOutput.Name, "dist")
	}
	if !buildOutput.Regenerable {
		t.Error("dist build output should be Regenerable")
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
