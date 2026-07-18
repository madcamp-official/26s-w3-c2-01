package git

import (
	"context"
	"os"
	"path/filepath"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/pathutil"
	"github.com/madcamp-official/26s-w3-c2-01/internal/scanner"
)

// Detector determines whether a directory entry is the root of a Git
// repository and, if so, builds the resulting domain.BuildProject.
//
// This is a fallback classification: callers should only invoke it for a
// directory that has no other recognized project marker (.vcxproj, .csproj,
// package.json). Otherwise the same directory would be registered as two
// BuildProject rows (e.g. both msbuild-cpp and git) with the same disk
// footprint counted under each, double-counting its LogicalSize.
type Detector interface {
	// CanDetect reports whether entry's directory contains a .git entry.
	// .git is normally a directory, but in a linked worktree it is a file
	// pointing at the real repository elsewhere -- either form counts.
	CanDetect(entry scanner.Entry) bool
	// Detect builds the domain.BuildProject(s) for the Git repository rooted
	// at entry. Callers should only call this after CanDetect reports true.
	// It returns a slice, rather than a single BuildProject, so that a Git
	// root containing more than one independent build project is not
	// precluded by the return type.
	Detect(ctx context.Context, entry scanner.Entry) ([]domain.BuildProject, error)
}

// FilesystemDetector is the real Detector implementation: it checks for a
// .git entry directly on disk, so it needs no mocking for tests.
type FilesystemDetector struct{}

func (FilesystemDetector) CanDetect(entry scanner.Entry) bool {
	_, err := os.Stat(filepath.Join(entry.Path, ".git"))
	return err == nil
}

func (FilesystemDetector) Detect(ctx context.Context, entry scanner.Entry) ([]domain.BuildProject, error) {
	abs, err := pathutil.Absolute(entry.Path)
	if err != nil {
		return nil, err
	}

	return []domain.BuildProject{{
		Name:           filepath.Base(abs),
		Path:           abs,
		Type:           domain.ProjectTypeGit,
		Drive:          filepath.VolumeName(abs),
		LastModifiedAt: entry.ModifiedAt,
	}}, nil
}
