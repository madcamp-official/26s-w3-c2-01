package safety

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	gitadapter "github.com/madcamp-official/26s-w3-c2-01/internal/adapter/git"
	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/pathutil"
	"github.com/madcamp-official/26s-w3-c2-01/internal/scanner"
)

var allowedArtifactNames = map[string]struct{}{
	"node_modules": {}, "bin": {}, "obj": {}, "build": {}, "dist": {},
	".next": {}, "out": {}, "debug": {}, "release": {},
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
		return CleanupValidation{}, fmt.Errorf("path identity changed")
	}
	if resource.NormalizedPath != item.NormalizedPath || resource.Type != item.ExpectedType || resource.Risk != domain.RiskSafe || !resource.Regenerable {
		return CleanupValidation{}, fmt.Errorf("resource snapshot is no longer SAFE and regenerable")
	}
	if _, ok := allowedArtifactNames[strings.ToLower(filepath.Base(item.NormalizedPath))]; !ok {
		return CleanupValidation{}, fmt.Errorf("path basename is not in the cleanup allowlist")
	}
	if ownerRoot == "" {
		return CleanupValidation{}, fmt.Errorf("owner project is not verified")
	}
	owned, err := pathutil.IsSameOrChild(item.NormalizedPath, ownerRoot)
	if err != nil || !owned {
		return CleanupValidation{}, fmt.Errorf("path is outside its owner project")
	}
	classification, err := v.Paths.Classify(item.NormalizedPath)
	if err != nil {
		return CleanupValidation{}, err
	}
	if classification.SystemManaged {
		return CleanupValidation{}, fmt.Errorf("path is protected by %s", classification.ProtectedRoot)
	}
	info, err := os.Lstat(item.NormalizedPath)
	if err != nil {
		return CleanupValidation{}, fmt.Errorf("lstat cleanup path: %w", err)
	}
	if !info.IsDir() {
		return CleanupValidation{}, fmt.Errorf("cleanup target is not a directory")
	}
	reparse, err := IsReparsePoint(item.NormalizedPath)
	if err != nil {
		return CleanupValidation{}, err
	}
	if reparse {
		return CleanupValidation{}, fmt.Errorf("cleanup target is a symlink, junction, or reparse point")
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
			return CleanupValidation{}, fmt.Errorf("cleanup target contains Git tracked files")
		}
	}
	measured, err := scanner.MeasureResource(ctx, scanner.New(0), item.NormalizedPath)
	if err != nil || !measured.SizeKnown {
		return CleanupValidation{}, fmt.Errorf("re-measure cleanup target: size is not fully verified")
	}
	if measured.LogicalSize != item.ExpectedSize {
		return CleanupValidation{}, fmt.Errorf("size changed from %d to %d", item.ExpectedSize, measured.LogicalSize)
	}
	modified := info.ModTime().UTC().UnixNano()
	if measured.LastModifiedAt != nil {
		modified = measured.LastModifiedAt.UTC().UnixNano()
	}
	if !item.ExpectedModifiedTime.IsZero() && modified != item.ExpectedModifiedTime.UTC().UnixNano() {
		return CleanupValidation{}, fmt.Errorf("modified time changed")
	}
	return CleanupValidation{ActualSize: measured.LogicalSize, ActualModifiedAt: modified}, nil
}
