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

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/pathutil"
	"github.com/madcamp-official/26s-w3-c2-01/internal/scanner"
)

// manifestFile is the marker that identifies a Node BuildProject root.
const manifestFile = "package.json"

// lockfiles are recognized as regenerability evidence for node_modules. MVP
// scope only needs a yes/no answer ("is there a lockfile at all"), so no
// priority order between them is required yet (§19.2).
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
	// CanDetect reports whether entry's directory contains package.json.
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
	_, err := os.Stat(filepath.Join(entry.Path, manifestFile))
	return err == nil
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
	Name string `json:"name"`
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

// DetectArtifacts inspects a Node project root (a directory containing
// package.json) for recognized build artifact directories immediately
// beneath it and returns unenriched domain.Resource candidates.
//
// Candidates only carry the fields an adapter is responsible for (Name,
// Type, DisplayPath, Regenerable, Confidence). app.ResourceService.Observe
// fills in the normalized path, stable ID, measured size, safety
// classification, and risk, and persists the result -- see
// docs/libra_integration_contracts.md §7.3 and §18.4.
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
// order, for any recognized lockfile; the first hit is enough evidence.
func detectArtifacts(root string, lockfileDirs ...string) ([]domain.Resource, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	lockfilePresent := false
	for _, dir := range lockfileDirs {
		if hasAnyLockfile(dir) {
			lockfilePresent = true
			break
		}
	}

	var resources []domain.Resource
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		resourceType, recognized := artifactDirs[entry.Name()]
		if !recognized {
			continue
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
			} else {
				resource.Confidence = confidenceInferredNodeModules
			}
		} else {
			// Build output directories are only ever name-matched in this
			// MVP -- no OutputPath/build config parsing -- so they stay
			// INFERRED-strength regardless of lockfile presence (§19.3).
			resource.Regenerable = true
			resource.Confidence = confidenceInferredBuildOutput
		}
		resources = append(resources, resource)
	}
	return resources, nil
}

func hasAnyLockfile(root string) bool {
	for _, name := range lockfiles {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			return true
		}
	}
	return false
}
