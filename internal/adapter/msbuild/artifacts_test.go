package msbuild

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

// TestDetectArtifacts_SymlinkedArtifactIsStillACandidate guards against a
// regression: os.ReadDir's DirEntry.IsDir() is Lstat-based, so it's false
// for a symlink or reparse point regardless of what it points to. Filtering
// on it directly (as this function used to) silently drops a symlinked bin/
// obj before it ever reaches app.projectArtifactCleanupEvidence's
// reparse-point check -- exactly the case that check exists to catch.
func TestDetectArtifacts_SymlinkedArtifactIsStillACandidate(t *testing.T) {
	root := t.TempDir()
	realDir := t.TempDir()
	if err := os.Symlink(realDir, filepath.Join(root, "bin")); err != nil {
		t.Skipf("creating symlink is not permitted: %v", err)
	}

	got, err := DetectArtifacts(root, nil)
	if err != nil {
		t.Fatalf("DetectArtifacts() error = %v", err)
	}
	if len(got) != 1 || got[0].Name != "bin" {
		t.Fatalf("got %+v, want the symlinked bin reported as a candidate", got)
	}
}

// TestDetectArtifacts_JunctionedArtifactIsStillACandidate is
// TestDetectArtifacts_SymlinkedArtifactIsStillACandidate's Windows-specific
// counterpart: a real NTFS junction (mklink /J, no elevated privilege
// needed, unlike a symlink) is reported by Go as ModeIrregular, not
// ModeSymlink -- so a fix that only checks DirEntry.Type()&os.ModeSymlink
// would still miss it. Confirmed by hand against a real junction before
// writing this: os.ReadDir on a junction gives IsDir()=false and
// Type()&os.ModeSymlink=false both.
func TestDetectArtifacts_JunctionedArtifactIsStillACandidate(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("NTFS junctions are Windows-only")
	}
	root := t.TempDir()
	realDir := filepath.Join(root, "realbin")
	if err := os.Mkdir(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "bin")
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, realDir).CombinedOutput(); err != nil {
		t.Skipf("creating a junction is not permitted: %v\n%s", err, out)
	}

	got, err := DetectArtifacts(root, nil)
	if err != nil {
		t.Fatalf("DetectArtifacts() error = %v", err)
	}
	if len(got) != 1 || got[0].Name != "bin" {
		t.Fatalf("got %+v, want the junctioned bin reported as a candidate", got)
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
