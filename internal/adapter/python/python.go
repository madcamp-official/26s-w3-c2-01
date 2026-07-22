// Package python detects Python projects (pyproject.toml/Pipfile/setup.py/
// requirements.txt), the virtual environment and cache directories they own
// (.venv/venv/env, __pycache__, .pytest_cache, .mypy_cache, build, dist,
// *.egg-info), and how strong the evidence is that a venv can be exactly
// regenerated.
//
// Scope for this iteration (see docs/libra_integration_contracts.md §19.4,
// docs/libra_python_conda_scope_decisions.md 결정 1/2/3/6 for the full
// reasoning this package resolves to CONFIRMED at MVP scope):
//
//   - Marker priority is pyproject.toml > Pipfile > setup.py >
//     requirements.txt (DetectMarkers). Every marker found is kept --
//     Primary decides project identity/manifest path, Secondary is recorded
//     for later use (e.g. lockfile discovery) but does not compete for
//     identity.
//   - requirements.txt alone is not trusted as a project marker unless the
//     same directory or one of its immediate source directories has a .py
//     file (containsPythonSource) -- unlike Node's package.json, a loose
//     requirements file alone does not establish a project.
//   - Regeneration evidence is a 4-tier scale (LockfileEvidence): a real
//     lockfile (poetry.lock/Pipfile.lock/uv.lock) is DECLARED; a fully
//     version-pinned requirements.txt is the new PINNED tier; a
//     partially/unpinned requirements.txt, or dependencies declared only in
//     pyproject.toml/Pipfile/setup.py with no lock, is INFERRED; nothing at
//     all is UNKNOWN.
//   - A venv (.venv/venv/env) is only confirmed -- not just name-matched --
//     when pyvenv.cfg exists inside it (DetectVenv). It is only marked
//     Regenerable (and so only eligible for the cleanup allowlist) when
//     LockfileEvidence is DECLARED or PINNED; INFERRED/UNKNOWN evidence
//     leaves it REVIEW.
//   - Cache/build-output directories (__pycache__, .pytest_cache,
//     .mypy_cache, build, dist, *.egg-info) are always Regenerable: they are
//     compiler/test byproducts, not dependency installs, so no lockfile-style
//     evidence gate applies to them.
//   - conda environment detection (both globally registered named
//     environments and local prefix environments under a project root) is
//     internal/adapter/conda's responsibility, not this package's --
//     internal/app/project_detector_adapters.go glues the two together the
//     same way NodeProjectDetector glues node.go and workspace.go.
package python

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/pathutil"
	"github.com/madcamp-official/26s-w3-c2-01/internal/scanner"
)

// Marker file names, in the priority order 결정 1 fixes.
const (
	markerPyproject    = "pyproject.toml"
	markerPipfile      = "Pipfile"
	markerSetupPy      = "setup.py"
	markerRequirements = "requirements.txt"
)

var markerPriority = []string{markerPyproject, markerPipfile, markerSetupPy, markerRequirements}

// lockfiles are real lockfiles, checked in this order -- the first hit is
// DECLARED-strength evidence and identifies which tool manages installs.
var lockfiles = []string{"poetry.lock", "Pipfile.lock", "uv.lock"}

var lockfileInstallCommand = map[string]string{
	"poetry.lock":  "poetry install",
	"Pipfile.lock": "pipenv install",
	"uv.lock":      "uv sync",
}

// venvNames are the recognized virtual environment directory names, only
// trusted once pyvenv.cfg is found inside (결정 3).
var venvNames = []string{".venv", "venv", "env"}

// cacheDirNames map recognized cache/build-artifact directory names to the
// shared domain.ResourceType. Always regenerable (결정 6) -- *.egg-info is
// matched by suffix separately since its prefix varies per package.
var cacheDirNames = map[string]struct{}{
	"__pycache__":   {},
	".pytest_cache": {},
	".mypy_cache":   {},
	"build":         {},
	"dist":          {},
}

const eggInfoSuffix = ".egg-info"

// vendoredDirs are directories that hold installed third-party packages, not
// authored projects -- a marker file (setup.py, pyproject.toml, ...) at or
// beneath one of these belongs to a dependency, mirroring node.isVendoredPath's
// node_modules guard (issue #36). The default config exclude list also skips
// walking into these by name (internal/config/config.go), but this guard
// still applies if a scan root is pointed directly inside one, or a user's
// config overrides the default excludes.
var vendoredDirs = []string{"site-packages", "dist-packages"}

// isVendoredPath reports whether path is, or lives beneath, a directory
// listed in vendoredDirs. Matching is segment-wise so a sibling like
// "my-site-packages-notes" is not treated as vendored.
func isVendoredPath(path string) bool {
	for _, segment := range strings.Split(filepath.ToSlash(path), "/") {
		for _, vendored := range vendoredDirs {
			if segment == vendored {
				return true
			}
		}
	}
	return false
}

// venvAllowlistTiers is the minimum LockfileEvidence tier (결정 2) a venv
// must have before 결정 6 allows it into the cleanup allowlist.
var venvAllowlistTiers = map[domain.EvidenceKind]bool{
	domain.EvidenceDeclared: true,
	domain.EvidencePinned:   true,
}

// Markers is which recognized Python project marker files were found
// directly under a candidate root.
type Markers struct {
	Primary   string
	Secondary []string
}

// Detector determines whether a directory is the root of a Python project
// and, if so, builds the resulting domain.BuildProject. Mirrors
// internal/adapter/node.Detector.
type Detector interface {
	CanDetect(entry scanner.Entry) bool
	Detect(ctx context.Context, entry scanner.Entry) (domain.BuildProject, error)
}

// FilesystemDetector is the real Detector implementation: it checks for a
// recognized marker file directly on disk.
type FilesystemDetector struct{}

func (FilesystemDetector) CanDetect(entry scanner.Entry) bool {
	if isVendoredPath(entry.Path) {
		return false
	}
	markers, err := DetectMarkers(entry.Path)
	if err != nil {
		return false
	}
	return markers.Primary != ""
}

func (FilesystemDetector) Detect(ctx context.Context, entry scanner.Entry) (domain.BuildProject, error) {
	abs, err := pathutil.Absolute(entry.Path)
	if err != nil {
		return domain.BuildProject{}, err
	}
	markers, err := DetectMarkers(abs)
	if err != nil {
		return domain.BuildProject{}, err
	}
	if markers.Primary == "" {
		return domain.BuildProject{}, fmt.Errorf("no Python project marker found in %s", abs)
	}
	return domain.BuildProject{
		Name:           filepath.Base(abs),
		Type:           domain.ProjectTypePython,
		RootPath:       abs,
		ManifestPath:   filepath.Join(abs, markers.Primary),
		Drive:          filepath.VolumeName(abs),
		LastModifiedAt: entry.ModifiedAt,
	}, nil
}

// DetectMarkers reports which recognized Python project marker files exist
// directly under root, in 결정 1's priority order. requirements.txt is only
// accepted as Primary when root or an immediate child directory contains a
// Python source file -- see containsPythonSource.
func DetectMarkers(root string) (Markers, error) {
	var found []string
	for _, name := range markerPriority {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			found = append(found, name)
		} else if !os.IsNotExist(err) {
			return Markers{}, err
		}
	}
	if len(found) == 0 {
		return Markers{}, nil
	}
	if found[0] == markerRequirements {
		hasPy, err := containsPythonSource(root)
		if err != nil {
			return Markers{}, err
		}
		if !hasPy {
			return Markers{}, nil
		}
	}
	return Markers{Primary: found[0], Secondary: found[1:]}, nil
}

// containsPythonSource reports whether root or one of its immediate child
// directories contains a *.py file. The one-level bound recognizes common
// layouts such as app/main.py and src/package.py while avoiding an unbounded
// walk that could turn a requirements file plus vendored/test data somewhere
// deep below it into a false project marker.
func containsPythonSource(root string) (bool, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".py") {
			return true, nil
		}
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") || isVendoredDirectory(entry.Name()) {
			continue
		}
		children, err := os.ReadDir(filepath.Join(root, entry.Name()))
		if err != nil {
			return false, err
		}
		for _, child := range children {
			if !child.IsDir() && strings.HasSuffix(child.Name(), ".py") {
				return true, nil
			}
		}
	}
	return false, nil
}

func isVendoredDirectory(name string) bool {
	for _, vendored := range vendoredDirs {
		if name == vendored {
			return true
		}
	}
	return false
}

// LockfileEvidence reports how strongly root's dependency declarations back
// "this can be regenerated exactly" (결정 2), and the command that performs
// that regeneration when the tier is strong enough to trust one.
func LockfileEvidence(root string) (kind domain.EvidenceKind, installCommand string) {
	for _, name := range lockfiles {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			return domain.EvidenceDeclared, lockfileInstallCommand[name]
		}
	}
	if pinned, exists, err := requirementsPinned(root); err == nil && exists {
		if pinned {
			return domain.EvidencePinned, "pip install -r requirements.txt"
		}
		return domain.EvidenceInferred, "pip install -r requirements.txt"
	}
	if hasAnyFile(root, markerPyproject, markerPipfile, markerSetupPy) {
		return domain.EvidenceInferred, ""
	}
	return domain.EvidenceUnknown, ""
}

// requirementsPinned reports whether root has a requirements.txt (exists)
// and, if so, whether every real dependency line in it pins an exact version
// with "==". Comments, blank lines, and option lines (-r, -e, --...) are
// skipped rather than counted against pinning.
func requirementsPinned(root string) (pinned, exists bool, err error) {
	data, err := os.ReadFile(filepath.Join(root, markerRequirements))
	if err != nil {
		if os.IsNotExist(err) {
			return false, false, nil
		}
		return false, false, err
	}
	sawEntry := false
	allPinned := true
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "-") {
			continue
		}
		sawEntry = true
		if !strings.Contains(line, "==") {
			allPinned = false
		}
	}
	return sawEntry && allPinned, true, nil
}

func hasAnyFile(root string, names ...string) bool {
	for _, name := range names {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			return true
		}
	}
	return false
}

// DetectVenv reports the confirmed virtual environment directory directly
// under root, if any (결정 3: a name match alone is not enough -- pyvenv.cfg
// must actually be present inside).
func DetectVenv(root string) (dirName string, ok bool, err error) {
	for _, name := range venvNames {
		candidate := filepath.Join(root, name)
		info, statErr := os.Stat(candidate)
		if statErr != nil || !info.IsDir() {
			continue
		}
		if _, cfgErr := os.Stat(filepath.Join(candidate, "pyvenv.cfg")); cfgErr == nil {
			return name, true, nil
		}
	}
	return "", false, nil
}

// DetectArtifacts inspects a Python project root for recognized cache/
// build-output directories and a confirmed virtual environment, returning
// unenriched domain.Resource candidates -- same contract as
// node.DetectArtifacts (docs/libra_integration_contracts.md §7.3): only
// Name, Type, DisplayPath, Regenerable, Confidence, RegenerationCommand are
// set; app.ResourceService fills in the rest.
func DetectArtifacts(root string) ([]domain.Resource, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	lockfileKind, installCommand := LockfileEvidence(root)
	venvName, hasVenv, err := DetectVenv(root)
	if err != nil {
		return nil, err
	}

	var resources []domain.Resource
	for _, entry := range entries {
		name := entry.Name()
		isDir := entry.IsDir()
		if !isDir {
			// entry.IsDir() is Lstat-based (false for symlinks/reparse
			// points); os.Stat follows the link so a junctioned venv or
			// cache dir is not silently dropped, same reasoning as
			// node.detectArtifacts.
			info, statErr := os.Stat(filepath.Join(root, name))
			if statErr != nil || !info.IsDir() {
				continue
			}
			isDir = true
		}
		if !isDir {
			continue
		}

		switch {
		case hasVenv && name == venvName:
			resources = append(resources, venvResource(root, name, lockfileKind, installCommand))
		case isCacheDir(name):
			resources = append(resources, cacheResource(root, name))
		}
	}
	return resources, nil
}

func isCacheDir(name string) bool {
	if _, ok := cacheDirNames[name]; ok {
		return true
	}
	return strings.HasSuffix(name, eggInfoSuffix)
}

func cacheResource(root, name string) domain.Resource {
	return domain.Resource{
		Name:        name,
		Type:        domain.ResourceTypeBuildOutput,
		DisplayPath: filepath.Join(root, name),
		Regenerable: true,
		Confidence:  domain.DefaultConfidence[domain.EvidenceInferred],
	}
}

func venvResource(root, name string, kind domain.EvidenceKind, installCommand string) domain.Resource {
	resource := domain.Resource{
		Name:        name,
		Type:        domain.ResourceTypeVenv,
		DisplayPath: filepath.Join(root, name),
		Confidence:  domain.DefaultConfidence[kind],
		Regenerable: venvAllowlistTiers[kind],
	}
	if resource.Regenerable {
		resource.RegenerationCommand = installCommand
	}
	return resource
}
