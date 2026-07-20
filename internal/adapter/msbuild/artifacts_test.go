package msbuild

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

func TestDetectArtifacts(t *testing.T) {
	root := "../../../testdata/msbuild/GameClient"

	got, err := DetectArtifacts(root, nil)
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
		if r.Confidence != confidenceInferredBuildOutput {
			t.Errorf("%s: Confidence = %d, want INFERRED (%d) -- name match only, nothing declared it", name, r.Confidence, confidenceInferredBuildOutput)
		}
		wantPath := filepath.Join(root, name)
		if r.DisplayPath != wantPath {
			t.Errorf("%s: DisplayPath = %q, want %q", name, r.DisplayPath, wantPath)
		}
	}
}

func TestDetectArtifacts_NoArtifacts(t *testing.T) {
	root := "../../../testdata/msbuild/SampleDotNetApp"

	got, err := DetectArtifacts(root, nil)
	if err != nil {
		t.Fatalf("DetectArtifacts() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d resources, want 0: %+v", len(got), got)
	}
}

func TestDetectArtifacts_DeclaredOutDirIsTrustedOverNameGuess(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "Build"), 0o755); err != nil {
		t.Fatal(err)
	}
	declared := []DeclaredProperty{{Name: "OutDir", Value: `Build\`}}

	got, err := DetectArtifacts(root, declared)
	if err != nil {
		t.Fatalf("DetectArtifacts() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d resources, want 1 (Build): %+v", len(got), got)
	}
	if got[0].Name != "Build" || got[0].Confidence != confidenceDeclaredBuildOutput {
		t.Errorf("got %+v, want Build at DECLARED confidence (%d)", got[0], confidenceDeclaredBuildOutput)
	}
}

func TestDetectArtifacts_DeclaredValueWithMacroFallsBackToNameMatch(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A macro-bearing OutDir can't be resolved without knowing which
	// configuration is being built, so it's skipped -- bin is still found,
	// but only via the weaker name-match path.
	declared := []DeclaredProperty{{Name: "OutDir", Value: `$(SolutionDir)$(Platform)\$(Configuration)\`}}

	got, err := DetectArtifacts(root, declared)
	if err != nil {
		t.Fatalf("DetectArtifacts() error = %v", err)
	}
	if len(got) != 1 || got[0].Name != "bin" || got[0].Confidence != confidenceInferredBuildOutput {
		t.Fatalf("got %+v, want bin at INFERRED confidence (%d)", got, confidenceInferredBuildOutput)
	}
}

func TestDetectArtifacts_ConditionalDeclarationNotTrusted(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	declared := []DeclaredProperty{{Name: "OutDir", Value: `bin\`, Condition: "'$(Configuration)' == 'Debug'"}}

	got, err := DetectArtifacts(root, declared)
	if err != nil {
		t.Fatalf("DetectArtifacts() error = %v", err)
	}
	if len(got) != 1 || got[0].Confidence != confidenceInferredBuildOutput {
		t.Fatalf("got %+v, want bin at INFERRED confidence (conditional declaration doesn't count)", got)
	}
}
