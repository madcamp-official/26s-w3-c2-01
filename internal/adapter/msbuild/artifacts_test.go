package msbuild

import (
	"path/filepath"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

func TestDetectArtifacts(t *testing.T) {
	root := "../../../testdata/msbuild/GameClient"

	got, err := DetectArtifacts(root)
	if err != nil {
		t.Fatalf("DetectArtifacts() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d resources, want 2 (bin, obj): %+v", len(got), got)
	}

	byName := map[string]domain.Resource{}
	for _, r := range got {
		byName[r.Name] = r
	}

	for _, name := range []string{"bin", "obj"} {
		r, ok := byName[name]
		if !ok {
			t.Fatalf("missing resource for %q", name)
		}
		if r.Type != domain.ResourceTypeBuildOutput {
			t.Errorf("%s: Type = %v, want %v", name, r.Type, domain.ResourceTypeBuildOutput)
		}
		if !r.Regenerable {
			t.Errorf("%s: Regenerable = false, want true", name)
		}
		wantPath := filepath.Join(root, name)
		if r.DisplayPath != wantPath {
			t.Errorf("%s: DisplayPath = %q, want %q", name, r.DisplayPath, wantPath)
		}
	}
}

func TestDetectArtifacts_NoArtifacts(t *testing.T) {
	root := "../../../testdata/msbuild/SampleDotNetApp"

	got, err := DetectArtifacts(root)
	if err != nil {
		t.Fatalf("DetectArtifacts() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d resources, want 0: %+v", len(got), got)
	}
}
