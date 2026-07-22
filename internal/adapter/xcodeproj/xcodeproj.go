// Package xcodeproj detects Xcode projects (.xcodeproj) and workspaces
// (.xcworkspace). Both are directory bundles, not files -- CanDetect matches
// on the scanner.Entry being a directory whose name ends in the bundle
// suffix, the same way git.Detector matches on a .git directory (or file, in
// a linked worktree) rather than a single well-known filename.
package xcodeproj

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/scanner"
)

const projectSuffix = ".xcodeproj"

// Detector recognizes an .xcodeproj bundle as the manifest of a BuildProject
// rooted at its parent directory (the project's actual source files are
// siblings of the .xcodeproj, not inside it).
type Detector struct{}

func (Detector) CanDetect(entry scanner.Entry) bool {
	return entry.Kind == scanner.EntryDirectory && strings.HasSuffix(entry.Path, projectSuffix)
}

func (Detector) Detect(_ context.Context, entry scanner.Entry) (domain.BuildProject, error) {
	// A valid .xcodeproj bundle contains project.pbxproj; without it the
	// directory is a leftover/backup/empty bundle, not a real project. Reject
	// it so a nonexistent manifest never anchors a stored project's ID or
	// evidence (the caller turns this error into a malformed-manifest issue).
	manifestPath := filepath.Join(entry.Path, "project.pbxproj")
	info, err := os.Stat(manifestPath)
	if err != nil {
		return domain.BuildProject{}, fmt.Errorf("missing %s: %w", manifestPath, err)
	}
	if info.IsDir() {
		return domain.BuildProject{}, fmt.Errorf("%s is a directory, not a project manifest", manifestPath)
	}

	root := filepath.Dir(entry.Path)
	name := strings.TrimSuffix(filepath.Base(entry.Path), projectSuffix)
	return domain.BuildProject{
		Name:           name,
		Type:           domain.ProjectTypeXcode,
		RootPath:       root,
		ManifestPath:   manifestPath,
		LastModifiedAt: entry.ModifiedAt,
	}, nil
}
