package safety

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	gitadapter "github.com/madcamp-official/26s-w3-c2-01/internal/adapter/git"
	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/pathutil"
	"github.com/madcamp-official/26s-w3-c2-01/internal/scanner"
)

var ErrCleanupBlocked = errors.New("cleanup blocked by safety policy")

func cleanupBlocked(message string) error {
	return fmt.Errorf("%s: %w", message, ErrCleanupBlocked)
}

var allowedArtifactNames = map[string]struct{}{
	"node_modules": {}, "bin": {}, "obj": {}, "build": {}, "dist": {},
	".next": {}, "out": {}, "debug": {}, "release": {},
	// Python (docs/libra_integration_contracts.md §19.4 결정 6): venv is
	// still gated on RiskPolicy/Regenerable elsewhere (only DECLARED/PINNED
	// lockfile evidence sets Regenerable=true for it) -- this list only
	// controls which basenames are structurally eligible at all. conda
	// environments (python-venv's sibling ResourceTypeCondaEnv) are
	// deliberately never added here: 결정 4 keeps them out of the cleanup
	// path entirely, regardless of basename.
	".venv": {}, "venv": {}, "env": {},
	"__pycache__": {}, ".pytest_cache": {}, ".mypy_cache": {},
	// macOS (docs/libra_integration_contracts.md §19.9): project-owned
	// CocoaPods Pods/ and SwiftPM .build/, the direct analogues of
	// node_modules. Like every other name here they still pass RiskPolicy
	// only when their regeneration evidence (Podfile.lock / Package.resolved)
	// makes them Regenerable -- this list only controls structural
	// eligibility. Basenames are compared lowercased, so "Pods" -> "pods".
	"pods": {}, ".build": {},
}

// eggInfoSuffix matches Python's *.egg-info build metadata directories,
// whose prefix is the package's own name and so can't be listed as a fixed
// basename the way the exact-match names above can (결정 6).
const eggInfoSuffix = ".egg-info"

func isAllowedArtifactName(basename string) bool {
	if _, ok := allowedArtifactNames[basename]; ok {
		return true
	}
	return strings.HasSuffix(basename, eggInfoSuffix)
}

type CleanupValidation struct {
	ActualSize       int64
	ActualModifiedAt int64
}

type CleanupValidator struct {
	Paths *PathClassifier
}

func (v CleanupValidator) Validate(ctx context.Context, item domain.CleanupPlanItem, resource domain.Resource, ownerRoot string) (CleanupValidation, error) {
	actual, err := pathutil.Normalize(item.NormalizedPath)
	if err != nil || actual != item.NormalizedPath {
		return CleanupValidation{}, cleanupBlocked("path identity changed")
	}
	if resource.NormalizedPath != item.NormalizedPath || resource.Type != item.ExpectedType || resource.Risk != domain.RiskSafe || !resource.Regenerable {
		return CleanupValidation{}, cleanupBlocked("resource snapshot is no longer SAFE and regenerable")
	}
	if !isAllowedArtifactName(strings.ToLower(filepath.Base(item.NormalizedPath))) {
		return CleanupValidation{}, cleanupBlocked("path basename is not in the cleanup allowlist")
	}
	if ownerRoot == "" {
		return CleanupValidation{}, cleanupBlocked("owner project is not verified")
	}
	owned, err := pathutil.IsSameOrChild(item.NormalizedPath, ownerRoot)
	if err != nil || !owned {
		return CleanupValidation{}, cleanupBlocked("path is outside its owner project")
	}
	classification, err := v.Paths.Classify(item.NormalizedPath)
	if err != nil {
		return CleanupValidation{}, err
	}
	if classification.SystemManaged {
		return CleanupValidation{}, cleanupBlocked(fmt.Sprintf("path is protected by %s", classification.ProtectedRoot))
	}
	info, err := os.Lstat(item.NormalizedPath)
	if err != nil {
		return CleanupValidation{}, fmt.Errorf("lstat cleanup path: %w", err)
	}
	if !info.IsDir() {
		return CleanupValidation{}, cleanupBlocked("cleanup target is not a directory")
	}
	reparse, err := IsReparsePoint(item.NormalizedPath)
	if err != nil {
		return CleanupValidation{}, err
	}
	if reparse {
		return CleanupValidation{}, cleanupBlocked("cleanup target is a symlink, junction, or reparse point")
	}
	repoRoot, found, err := gitadapter.FindRepoRoot(item.NormalizedPath)
	if err != nil {
		return CleanupValidation{}, fmt.Errorf("find Git root: %w", err)
	}
	if found {
		tracked, err := (gitadapter.TrackedFilesChecker{}).HasTrackedFiles(ctx, repoRoot, item.NormalizedPath)
		if err != nil {
			return CleanupValidation{}, fmt.Errorf("verify Git tracked files: %w", err)
		}
		if tracked {
			return CleanupValidation{}, cleanupBlocked("cleanup target contains Git tracked files")
		}
	}
	measured, err := scanner.MeasureResource(ctx, scanner.New(0), item.NormalizedPath)
	if err != nil || !measured.SizeKnown {
		return CleanupValidation{}, cleanupBlocked("re-measure cleanup target: size is not fully verified")
	}
	if measured.LogicalSize != item.ExpectedSize {
		return CleanupValidation{}, cleanupBlocked(fmt.Sprintf("size changed from %d to %d", item.ExpectedSize, measured.LogicalSize))
	}
	modified := info.ModTime().UTC().UnixNano()
	if measured.LastModifiedAt != nil {
		modified = measured.LastModifiedAt.UTC().UnixNano()
	}
	if !item.ExpectedModifiedTime.IsZero() && modified != item.ExpectedModifiedTime.UTC().UnixNano() {
		return CleanupValidation{}, cleanupBlocked("modified time changed")
	}
	return CleanupValidation{ActualSize: measured.LogicalSize, ActualModifiedAt: modified}, nil
}
