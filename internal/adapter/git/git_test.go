package git

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
	repoDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	nonRepoDir := t.TempDir()

	var detector Detector = FilesystemDetector{}

	if !detector.CanDetect(scanner.Entry{Path: repoDir}) {
		t.Errorf("CanDetect(%q) = false, want true", repoDir)
	}
	if detector.CanDetect(scanner.Entry{Path: nonRepoDir}) {
		t.Errorf("CanDetect(%q) = true, want false", nonRepoDir)
	}
}

func TestFilesystemDetector_Detect(t *testing.T) {
	repoDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	var detector Detector = FilesystemDetector{}
	// Deliberately not the directory's real mtime: proves Detect reuses the
	// entry's ModifiedAt instead of re-stat'ing the filesystem.
	modTime := time.Date(2026, 7, 18, 3, 4, 5, 0, time.UTC)

	got, err := detector.Detect(context.Background(), scanner.Entry{Path: repoDir, ModifiedAt: modTime})
	if err != nil {
		t.Fatalf("Detect returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d build projects, want 1: %+v", len(got), got)
	}
	if got[0].Type != domain.ProjectTypeGit {
		t.Errorf("Type = %v, want %v", got[0].Type, domain.ProjectTypeGit)
	}
	if got[0].Name != filepath.Base(repoDir) {
		t.Errorf("Name = %q, want %q", got[0].Name, filepath.Base(repoDir))
	}
	if !got[0].LastModifiedAt.Equal(modTime) {
		t.Errorf("LastModifiedAt = %v, want %v (reused from scanner.Entry)", got[0].LastModifiedAt, modTime)
	}
}
