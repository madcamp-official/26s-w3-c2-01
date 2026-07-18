package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

func TestFilesystemDetector_CanDetect(t *testing.T) {
	repoDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	nonRepoDir := t.TempDir()

	var detector Detector = FilesystemDetector{}

	if !detector.CanDetect(repoDir) {
		t.Errorf("CanDetect(%q) = false, want true", repoDir)
	}
	if detector.CanDetect(nonRepoDir) {
		t.Errorf("CanDetect(%q) = true, want false", nonRepoDir)
	}
}

func TestFilesystemDetector_Detect(t *testing.T) {
	repoDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	var detector Detector = FilesystemDetector{}

	got, err := detector.Detect(context.Background(), repoDir)
	if err != nil {
		t.Fatalf("Detect returned error: %v", err)
	}
	if got.Type != domain.ProjectTypeGit {
		t.Errorf("Type = %v, want %v", got.Type, domain.ProjectTypeGit)
	}
	if got.Name != filepath.Base(repoDir) {
		t.Errorf("Name = %q, want %q", got.Name, filepath.Base(repoDir))
	}
	if got.LastModifiedAt.IsZero() {
		t.Errorf("LastModifiedAt is zero, want the directory's mod time")
	}
}
