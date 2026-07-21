// Package node detects Node.js projects (package.json), the build
// artifacts they own (node_modules, dist, .next, build, out), and
// npm/Yarn/pnpm workspace relationships between them.
//
// Scope for this iteration (see docs/libra_integration_contracts.md §19.2,
// §19.3 -- resolved to CONFIRMED at MVP scope alongside this package):
//
//   - A workspace root is detected via package.json's "workspaces" field
//     (npm/Yarn) or a pnpm-workspace.yaml (pnpm); members are resolved with
//     single-segment glob patterns only (see workspace.go for the exact
//     limits -- no recursive "**", no negated patterns).
//   - Nested workspaces (a member that is itself a workspace root) are not
//     supported; only one level of root -> members is resolved.
//   - A member without its own node_modules is understood to share the
//     workspace root's node_modules (hoisting) rather than being reported
//     as missing one -- see DetectWorkspaceArtifacts.
//   - node_modules is treated as regenerable when a recognized lockfile
//     sits next to package.json, or -- for a workspace member -- at the
//     workspace root; no lockfile priority is needed for that yes/no
//     evidence check.
//   - A malformed package.json is a recoverable, per-candidate failure: it
//     does not stop detection of other projects or artifacts (§5).
package node

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/pathutil"
	"github.com/madcamp-official/26s-w3-c2-01/internal/scanner"
)

// manifestFile is the marker that identifies a Node BuildProject root.
const manifestFile = "package.json"

// vendoredDir is the directory installed dependencies live under. A
// package.json at or beneath a node_modules directory belongs to a
// third-party package, not an authored project, so CanDetect refuses it
// (issue #36). The owning project's node_modules is still reported as a
// Resource via DetectArtifacts and sized via scanner.MeasureResource, neither
// of which needs the discovery walk to descend into node_modules.
const vendoredDir = "node_modules"

// lockfiles are recognized as regenerability evidence for node_modules, and
// (via packageManagerOf) identify which package manager owns the project.
// Order is the priority checked in if more than one is somehow present.
var lockfiles = []string{
	"package-lock.json",
	"npm-shrinkwrap.json",
	"pnpm-lock.yaml",
	"yarn.lock",
}

// artifactDirs maps recognized build artifact directory names, immediately
// under a Node project root, to the domain.ResourceType they become.
var artifactDirs = map[string]domain.ResourceType{
	"node_modules": domain.ResourceTypeNodeModules,
	"dist":         domain.ResourceTypeBuildOutput,
	".next":        domain.ResourceTypeBuildOutput,
	"build":        domain.ResourceTypeBuildOutput,
	"out":          domain.ResourceTypeBuildOutput,
}

// Confidence scores draw from the CONFIRMED shared scale
// (docs/libra_integration_contracts.md §20.2, domain.DefaultConfidence).
var (
	// package.json + lockfile: dependencies are declared and resolvable.
	confidenceDeclaredNodeModules = domain.DefaultConfidence[domain.EvidenceDeclared]
	// node_modules exists but nothing declares how to regenerate it.
	confidenceInferredNodeModules = domain.DefaultConfidence[domain.EvidenceInferred]
	// directory name match only, no build config confirmation (§19.3).
	confidenceInferredBuildOutput = domain.DefaultConfidence[domain.EvidenceInferred]
)

// Detector determines whether a directory is the root of a Node project
// (contains package.json) and, if so, builds the resulting
// domain.BuildProject.
//
// This only returns the project fact. The application pipeline prepares its
// normalized identity and persists it through ProjectRepository. Artifacts
// follow the separate ResourceService pipeline.
type Detector interface {
	// CanDetect reports whether entry's directory is the root of a Node
	// project: it contains package.json and is not itself vendored (a
	// node_modules directory or anything beneath one). A package.json under
	// node_modules belongs to an installed dependency, not the developer, so
	// it is not a project (issue #36); node_modules is still detected and
	// sized as the owning project's resource, independently of the walk.
	CanDetect(entry scanner.Entry) bool
	// Detect builds the domain.BuildProject for the Node project rooted at
	// entry. Callers should only call this after CanDetect reports true. A
	// malformed package.json is returned as an error, not a panic or a
	// silently empty project, so orchestration can record it as a
	// recoverable per-candidate issue.
	Detect(ctx context.Context, entry scanner.Entry) (domain.BuildProject, error)
}

// FilesystemDetector is the real Detector implementation: it checks for a
// package.json entry directly on disk.
type FilesystemDetector struct{}

func (FilesystemDetector) CanDetect(entry scanner.Entry) bool {
	if isVendoredPath(entry.Path) {
		return false
	}
	_, err := os.Stat(filepath.Join(entry.Path, manifestFile))
	return err == nil
}

// isVendoredPath reports whether path is, or lives beneath, a node_modules
// directory. Matching is segment-wise so a sibling like "node_modules-cache"
// is not treated as vendored. Paths are normalized to forward slashes first so
// the same check holds for Windows "\\"-separated paths the scanner produces.
func isVendoredPath(path string) bool {
	for _, segment := range strings.Split(filepath.ToSlash(path), "/") {
		if segment == vendoredDir {
			return true
		}
	}
	return false
}

func (FilesystemDetector) Detect(ctx context.Context, entry scanner.Entry) (domain.BuildProject, error) {
	abs, err := pathutil.Absolute(entry.Path)
	if err != nil {
		return domain.BuildProject{}, err
	}

	name, err := readManifestName(filepath.Join(abs, manifestFile))
	if err != nil {
		return domain.BuildProject{}, err
	}
	if name == "" {
		name = filepath.Base(abs)
	}

	return domain.BuildProject{
		Name:           name,
		Type:           domain.ProjectTypeNode,
		RootPath:       abs,
		ManifestPath:   filepath.Join(abs, manifestFile),
		Drive:          filepath.VolumeName(abs),
		LastModifiedAt: entry.ModifiedAt,
	}, nil
}

type packageManifest struct {
	Name    string            `json:"name"`
	Scripts map[string]string `json:"scripts"`
}

// readManifestName parses package.json for its "name" field. A malformed
// manifest returns an error -- it is up to the caller (and, once
// orchestration lands, the scan pipeline) to treat that as a recoverable
// issue rather than aborting the whole scan.
func readManifestName(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var manifest packageManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return "", err
	}
	return manifest.Name, nil
}

// hasBuildScript reports whether root's package.json declares a non-empty
// "build" script. Unlike node_modules (whose install command is implied by
// the presence of a lockfile), a build-output directory's regeneration
// command is whatever "npm run build" happens to run -- so a declared build
// script is the minimum real evidence that *some* regeneration process
// exists, as opposed to a bare dist/.next/build/out name match with nothing
// backing it. It says nothing about whether that script's output actually
// lands in the directory being considered -- that would need parsing each
// bundler's own config format, which is out of scope (§19.3 -- this MVP
// only reads package.json, not webpack/next/vite config).
func hasBuildScript(root string) bool {
	data, err := os.ReadFile(filepath.Join(root, manifestFile))
	if err != nil {
		return false
	}
	var manifest packageManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return false
	}
	return strings.TrimSpace(manifest.Scripts["build"]) != ""
}

// DetectArtifacts inspects a Node project root (a directory containing
// package.json) for recognized build artifact directories immediately
// beneath it and returns unenriched domain.Resource candidates.
//
// Candidates only carry the fields an adapter is responsible for (Name,
// Type, DisplayPath, Regenerable, Confidence, RegenerationCommand).
// app.ResourceService.Observe fills in the normalized path, stable ID,
// measured size, safety classification, and risk, and persists the result
// -- see docs/libra_integration_contracts.md §7.3 and §18.4.
func DetectArtifacts(root string) ([]domain.Resource, error) {
	return detectArtifacts(root, root)
}

// DetectMemberArtifacts is DetectArtifacts for a workspace member: it also
// accepts a lockfile at workspaceRoot as regenerability evidence for
// memberRoot's own node_modules, since npm/Yarn/pnpm workspaces normally
// keep a single lockfile at the workspace root even when a member has its
// own node_modules directory. See DetectWorkspaceArtifacts.
func DetectMemberArtifacts(memberRoot, workspaceRoot string) ([]domain.Resource, error) {
	return detectArtifacts(memberRoot, memberRoot, workspaceRoot)
}

// detectArtifacts is the shared implementation. lockfileDirs are checked, in
// order, for any recognized lockfile; the first hit is enough evidence, and
// also identifies which package manager owns the project (for
// RegenerationCommand) -- see detectPackageManager.
func detectArtifacts(root string, lockfileDirs ...string) ([]domain.Resource, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	manager := detectPackageManager(lockfileDirs...)
	lockfilePresent := manager != ""
	buildScriptPresent := hasBuildScript(root)

	var resources []domain.Resource
	for _, entry := range entries {
		resourceType, recognized := artifactDirs[entry.Name()]
		if !recognized {
			continue
		}
		// entry.IsDir() is Lstat-based: false for a symlink or (on Windows)
		// a reparse point regardless of what it points to, so a symlinked/
		// junctioned node_modules would otherwise be silently dropped here
		// -- before projectArtifactCleanupEvidence's reparse-point check
		// ever gets a chance to flag it. os.Stat follows the link to see
		// whether it actually resolves to a directory.
		if !entry.IsDir() {
			info, err := os.Stat(filepath.Join(root, entry.Name()))
			if err != nil || !info.IsDir() {
				continue
			}
		}

		resource := domain.Resource{
			Name:        entry.Name(),
			Type:        resourceType,
			DisplayPath: filepath.Join(root, entry.Name()),
		}
		if resourceType == domain.ResourceTypeNodeModules {
			resource.Regenerable = lockfilePresent
			if lockfilePresent {
				resource.Confidence = confidenceDeclaredNodeModules
				resource.RegenerationCommand = installCommand(manager)
			} else {
				resource.Confidence = confidenceInferredNodeModules
			}
		} else {
			// Build output directories are only ever name-matched in this
			// MVP -- no bundler config parsing -- so they stay
			// INFERRED-strength regardless (§19.3). Regenerable is gated on
			// a declared "build" script existing at all: without one, there
			// is no evidence any process would recreate this directory, so
			// guessing Regenerable=true would be exactly the "name alone
			// isn't SAFE" mistake §19.3 warns about.
			resource.Regenerable = buildScriptPresent
			resource.Confidence = confidenceInferredBuildOutput
			if buildScriptPresent {
				// Run the build script through whichever package manager
				// owns the project (npm if none of the recognized lockfiles
				// were found -- npm ships with Node itself, the safest
				// generic default).
				resource.RegenerationCommand = buildCommand(manager)
			}
		}
		resources = append(resources, resource)
	}
	return resources, nil
}

// packageManagerOf maps each recognized lockfile to the package manager it
// implies. Checked in the same priority order as lockfiles: the first
// lockfile found wins if more than one is somehow present (e.g. a project
// mid-migration between package managers).
var packageManagerOf = map[string]string{
	"package-lock.json":   "npm",
	"npm-shrinkwrap.json": "npm",
	"yarn.lock":           "yarn",
	"pnpm-lock.yaml":      "pnpm",
}

// detectPackageManager returns which package manager's lockfile is present
// in dirs (checked in order), or "" if none is.
func detectPackageManager(dirs ...string) string {
	for _, dir := range dirs {
		for _, name := range lockfiles {
			if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
				return packageManagerOf[name]
			}
		}
	}
	return ""
}

// installCommand returns the command that reinstalls dependencies for the
// given package manager. manager must be non-empty (a recognized lockfile
// was found) -- callers only call this once lockfilePresent is true.
func installCommand(manager string) string {
	switch manager {
	case "yarn":
		return "yarn install"
	case "pnpm":
		return "pnpm install"
	default:
		return "npm ci"
	}
}

// buildCommand returns the command that runs the project's declared "build"
// script through the given package manager. An empty manager (no lockfile
// found, but a build script still exists) defaults to npm.
func buildCommand(manager string) string {
	if manager == "" {
		manager = "npm"
	}
	return manager + " run build"
}
