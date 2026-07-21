package xcode

import (
	"context"
	"errors"
	"os/exec"
	"strings"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

// InstallLister detects the installed Xcode.app itself -- the macOS
// analogue of msbuild.VSWhereToolLocator for Visual Studio: a system-managed
// dev tool, not a cache, so it is reported with SystemManaged set (BLOCKED
// regardless of install location -- some developers keep Xcode under
// ~/Applications rather than the system-wide /Applications to avoid sudo,
// which the path-based protected-root classifier alone would not catch).
//
// Only reported when `xcodebuild -version` actually succeeds, i.e. a full
// Xcode.app is active -- `xcode-select`'s developer directory alone can
// point at a much lighter Command Line Tools install (no Xcode.app at all),
// which is not the same system-managed resource this models.
type InstallLister struct {
	LookPath func(string) (string, error)
	Run      func(ctx context.Context, path string, args ...string) ([]byte, error)
}

func (l InstallLister) ListResources(ctx context.Context) ([]domain.Resource, error) {
	look := l.LookPath
	if look == nil {
		look = exec.LookPath
	}
	run := l.run

	selectPath, err := look("xcode-select")
	if errors.Is(err, exec.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	devDirOut, err := run(ctx, selectPath, "-p")
	if err != nil {
		// No active developer directory configured is not an error.
		return nil, nil
	}

	buildPath, err := look("xcodebuild")
	if errors.Is(err, exec.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	versionOut, err := run(ctx, buildPath, "-version")
	if err != nil {
		// xcodebuild fails with a descriptive error when only Command Line
		// Tools (no full Xcode.app) are active -- a valid "not installed"
		// result for this resource, not a real failure.
		return nil, nil
	}

	devDir := strings.TrimSpace(string(devDirOut))
	version := parseXcodebuildVersion(string(versionOut))
	return []domain.Resource{{
		Name:          "Xcode",
		Type:          domain.ResourceTypeXcodeInstall,
		Version:       version,
		DisplayPath:   xcodeAppPath(devDir),
		SystemManaged: true,
		Confidence:    domain.DefaultConfidence[domain.EvidenceResolved],
	}}, nil
}

func (l InstallLister) run(ctx context.Context, path string, args ...string) ([]byte, error) {
	if l.Run != nil {
		return l.Run(ctx, path, args...)
	}
	return exec.CommandContext(ctx, path, args...).Output()
}

// parseXcodebuildVersion extracts "15.4" from `xcodebuild -version`'s output:
//
//	Xcode 15.4
//	Build version 15F31d
func parseXcodebuildVersion(output string) string {
	lines := strings.Split(output, "\n")
	if len(lines) == 0 {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[0]), "Xcode"))
}

// xcodeAppPath derives the Xcode.app bundle path from xcode-select's
// developer directory (".../Xcode.app/Contents/Developer").
func xcodeAppPath(developerDir string) string {
	const suffix = "/Contents/Developer"
	return strings.TrimSuffix(developerDir, suffix)
}
