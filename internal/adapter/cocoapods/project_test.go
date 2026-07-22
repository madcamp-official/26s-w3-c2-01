package cocoapods

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

func TestDetectArtifactsReportsPodsRegenerableWithLockfile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Podfile"), []byte("platform :ios, '17.0'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Podfile.lock"), []byte("PODS:\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "Pods"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := DetectArtifacts(root)
	if err != nil {
		t.Fatalf("DetectArtifacts() error = %v", err)
	}
	if len(got) != 1 || got[0].Type != domain.ResourceTypePods || !got[0].Regenerable {
		t.Fatalf("got %#v", got)
	}
	if got[0].RegenerationCommand != "pod install" {
		t.Errorf("RegenerationCommand = %q", got[0].RegenerationCommand)
	}
}

func TestDetectArtifactsWithoutLockfileIsNotRegenerable(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Podfile"), []byte("platform :ios, '17.0'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "Pods"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := DetectArtifacts(root)
	if err != nil {
		t.Fatalf("DetectArtifacts() error = %v", err)
	}
	if len(got) != 1 || got[0].Regenerable {
		t.Fatalf("got %#v, want non-regenerable Pods resource", got)
	}
}

func TestDetectArtifactsReturnsNothingWithoutPodfile(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "Pods"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := DetectArtifacts(root)
	if err != nil || len(got) != 0 {
		t.Fatalf("got %#v, %v, want none without a Podfile", got, err)
	}
}
