package git

import (
	"context"
	"os"
	"path/filepath"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

// Detector determines whether a directory is the root of a Git repository
// and, if so, builds the resulting domain.BuildProject.
//
// This is a fallback classification: callers should only invoke it for a
// directory that has no other recognized project marker (.vcxproj, .csproj,
// package.json). Otherwise the same directory would be registered as two
// BuildProject rows (e.g. both msbuild-cpp and git) with the same disk
// footprint counted under each, double-counting its LogicalSize.
type Detector interface {
	// CanDetect reports whether dir contains a .git entry. .git is normally a
	// directory, but in a linked worktree it is a file pointing at the real
	// repository elsewhere -- either form counts.
	CanDetect(dir string) bool
	// Detect builds the domain.BuildProject for the Git repository rooted at
	// dir. Callers should only call this after CanDetect reports true.
	Detect(ctx context.Context, dir string) (domain.BuildProject, error)
}

// FilesystemDetector is the real Detector implementation: it checks for a
// .git entry directly on disk, so it needs no mocking for tests.
type FilesystemDetector struct{}

func (FilesystemDetector) CanDetect(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

func (FilesystemDetector) Detect(ctx context.Context, dir string) (domain.BuildProject, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}

	info, err := os.Stat(abs)
	if err != nil {
		return domain.BuildProject{}, err
	}

	return domain.BuildProject{
		Name:           filepath.Base(abs),
		Path:           abs,
		Type:           domain.ProjectTypeGit,
		Drive:          filepath.VolumeName(abs),
		LastModifiedAt: info.ModTime(),
	}, nil
}
