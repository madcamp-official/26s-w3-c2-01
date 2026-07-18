package node

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"go.yaml.in/yaml/v3"
)

// pnpmWorkspaceFile is the marker for a pnpm workspace root.
const pnpmWorkspaceFile = "pnpm-workspace.yaml"

// WorkspaceKind identifies which package manager's workspace mechanism a
// project root declares.
type WorkspaceKind string

const (
	WorkspaceKindNpmOrYarn WorkspaceKind = "npm-yarn" // "workspaces" field in package.json
	WorkspaceKindPnpm      WorkspaceKind = "pnpm"     // pnpm-workspace.yaml
)

// WorkspaceInfo is what a workspace root declares about its member
// packages.
type WorkspaceInfo struct {
	Kind     WorkspaceKind
	Patterns []string
}

// DetectWorkspace reports whether root declares an npm/Yarn "workspaces"
// field in its package.json or a sibling pnpm-workspace.yaml, checked in
// that order. A root is expected to use exactly one mechanism; if a
// package.json "workspaces" field is present, pnpm-workspace.yaml is not
// consulted.
//
// A directory with no package.json, or a package.json without a
// "workspaces" field and no pnpm-workspace.yaml, is simply not a workspace
// root -- that is a valid result (ok == false), not an error.
func DetectWorkspace(root string) (info WorkspaceInfo, ok bool, err error) {
	patterns, hasField, err := readPackageWorkspacesField(filepath.Join(root, manifestFile))
	if err != nil {
		return WorkspaceInfo{}, false, err
	}
	if hasField {
		return WorkspaceInfo{Kind: WorkspaceKindNpmOrYarn, Patterns: patterns}, true, nil
	}

	patterns, hasFile, err := readPnpmWorkspaceFile(filepath.Join(root, pnpmWorkspaceFile))
	if err != nil {
		return WorkspaceInfo{}, false, err
	}
	if hasFile {
		return WorkspaceInfo{Kind: WorkspaceKindPnpm, Patterns: patterns}, true, nil
	}

	return WorkspaceInfo{}, false, nil
}

// readPackageWorkspacesField reads package.json's "workspaces" field, which
// npm and Yarn accept as either a plain array of glob patterns or an object
// with a "packages" array (a Yarn classic extension). A missing manifest is
// a valid "not a workspace" result; a malformed manifest is an error, same
// as FilesystemDetector.Detect.
func readPackageWorkspacesField(manifestPath string) ([]string, bool, error) {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	var manifest struct {
		Workspaces json.RawMessage `json:"workspaces"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, false, err
	}
	if len(manifest.Workspaces) == 0 {
		return nil, false, nil
	}

	var patterns []string
	if err := json.Unmarshal(manifest.Workspaces, &patterns); err == nil {
		return patterns, true, nil
	}
	var withPackages struct {
		Packages []string `json:"packages"`
	}
	if err := json.Unmarshal(manifest.Workspaces, &withPackages); err == nil {
		return withPackages.Packages, true, nil
	}
	return nil, false, fmt.Errorf("package.json workspaces field is neither an array nor {packages: [...]}")
}

// readPnpmWorkspaceFile reads pnpm-workspace.yaml's "packages" list. A
// missing file is a valid "not a workspace" result.
func readPnpmWorkspaceFile(path string) ([]string, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	var file struct {
		Packages []string `yaml:"packages"`
	}
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, false, err
	}
	return file.Packages, true, nil
}

// ResolveMembers expands a workspace's glob patterns relative to root and
// returns the absolute path of each matched directory that itself contains
// package.json, deduplicated and sorted.
//
// MVP scope (docs/libra_integration_contracts.md §19.2): patterns are
// resolved with filepath.Glob, which only matches within a single path
// segment -- recursive "**" patterns are not supported and will simply
// match nothing beyond their literal segment. Negated patterns ("!...",
// a Yarn/npm convention for excluding a match) are not supported either;
// they are skipped outright rather than silently mismatched, so they never
// suppress a match they were meant to exclude.
func ResolveMembers(root string, info WorkspaceInfo) ([]string, error) {
	seen := make(map[string]struct{})
	var members []string
	for _, pattern := range info.Patterns {
		if strings.HasPrefix(pattern, "!") {
			continue
		}
		matches, err := filepath.Glob(filepath.Join(root, pattern))
		if err != nil {
			return nil, fmt.Errorf("resolve workspace pattern %q: %w", pattern, err)
		}
		for _, match := range matches {
			stat, err := os.Stat(match)
			if err != nil || !stat.IsDir() {
				continue
			}
			if _, err := os.Stat(filepath.Join(match, manifestFile)); err != nil {
				continue
			}
			abs, err := filepath.Abs(match)
			if err != nil {
				abs = match
			}
			if _, dup := seen[abs]; dup {
				continue
			}
			seen[abs] = struct{}{}
			members = append(members, abs)
		}
	}
	sort.Strings(members)
	return members, nil
}

// MemberArtifacts is one workspace member's own build-artifact candidates,
// plus whether it relies on the workspace root's node_modules instead of
// having its own.
//
// This is deliberately not a domain.Dependency: a REQUIRES edge needs a
// stable BuildProject ID, which is still DECISION_REQUIRED
// (docs/libra_integration_contracts.md §7.2/§15.2). Keeping this at adapter
// level means it is ready to feed a real PROJECT -[REQUIRES]-> RESOURCE
// edge (§18.5) the moment Project IDs exist, without having guessed at an
// ID scheme that isn't the team's decision to make from this package.
type MemberArtifacts struct {
	MemberRoot            string
	OwnArtifacts          []domain.Resource
	SharesRootNodeModules bool
}

// DetectWorkspaceArtifacts resolves a workspace's members and detects each
// member's own build artifacts. The workspace root's node_modules is
// detected exactly once (as part of rootArtifacts) even though every member
// may depend on it -- reporting it again per member would double-count its
// logical size, which docs/libra_integration_contracts.md §3.1 rules out
// ("디렉터리 논리 크기는 실제 디렉터리를 기준으로 한 번만 계산한다"). Instead,
// a member with no node_modules of its own gets SharesRootNodeModules=true
// when the root has one, so callers know that absence is expected, not a
// gap.
func DetectWorkspaceArtifacts(root string, info WorkspaceInfo) (rootArtifacts []domain.Resource, members []MemberArtifacts, err error) {
	rootArtifacts, err = DetectArtifacts(root)
	if err != nil {
		return nil, nil, err
	}
	rootHasNodeModules := false
	for _, resource := range rootArtifacts {
		if resource.Type == domain.ResourceTypeNodeModules {
			rootHasNodeModules = true
			break
		}
	}

	memberRoots, err := ResolveMembers(root, info)
	if err != nil {
		return nil, nil, err
	}

	for _, memberRoot := range memberRoots {
		artifacts, err := DetectMemberArtifacts(memberRoot, root)
		if err != nil {
			return nil, nil, err
		}
		memberHasOwnNodeModules := false
		for _, resource := range artifacts {
			if resource.Type == domain.ResourceTypeNodeModules {
				memberHasOwnNodeModules = true
				break
			}
		}
		members = append(members, MemberArtifacts{
			MemberRoot:            memberRoot,
			OwnArtifacts:          artifacts,
			SharesRootNodeModules: !memberHasOwnNodeModules && rootHasNodeModules,
		})
	}
	return rootArtifacts, members, nil
}
