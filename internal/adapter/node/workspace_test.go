package node

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

func TestDetectWorkspace_NpmYarnField(t *testing.T) {
	info, ok, err := DetectWorkspace("../../../testdata/node/workspace-npm")
	if err != nil {
		t.Fatalf("DetectWorkspace returned error: %v", err)
	}
	if !ok {
		t.Fatal("DetectWorkspace ok = false, want true")
	}
	if info.Kind != WorkspaceKindNpmOrYarn {
		t.Errorf("Kind = %v, want %v", info.Kind, WorkspaceKindNpmOrYarn)
	}
	if len(info.Patterns) != 1 || info.Patterns[0] != "packages/*" {
		t.Errorf("Patterns = %v, want [packages/*]", info.Patterns)
	}
}

func TestDetectWorkspace_PnpmFile(t *testing.T) {
	info, ok, err := DetectWorkspace("../../../testdata/node/workspace-pnpm")
	if err != nil {
		t.Fatalf("DetectWorkspace returned error: %v", err)
	}
	if !ok {
		t.Fatal("DetectWorkspace ok = false, want true")
	}
	if info.Kind != WorkspaceKindPnpm {
		t.Errorf("Kind = %v, want %v", info.Kind, WorkspaceKindPnpm)
	}
	if len(info.Patterns) != 1 || info.Patterns[0] != "packages/*" {
		t.Errorf("Patterns = %v, want [packages/*]", info.Patterns)
	}
}

func TestDetectWorkspace_NotAWorkspace(t *testing.T) {
	_, ok, err := DetectWorkspace("../../../testdata/node/basic")
	if err != nil {
		t.Fatalf("DetectWorkspace returned error: %v", err)
	}
	if ok {
		t.Error("DetectWorkspace ok = true, want false for a non-workspace project")
	}
}

func TestDetectWorkspace_ObjectFormWorkspacesField(t *testing.T) {
	dir := t.TempDir()
	manifest := `{"name":"root","workspaces":{"packages":["apps/*","tools/*"]}}`
	if err := os.WriteFile(filepath.Join(dir, manifestFile), []byte(manifest), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	info, ok, err := DetectWorkspace(dir)
	if err != nil {
		t.Fatalf("DetectWorkspace returned error: %v", err)
	}
	if !ok {
		t.Fatal("DetectWorkspace ok = false, want true")
	}
	if len(info.Patterns) != 2 || info.Patterns[0] != "apps/*" || info.Patterns[1] != "tools/*" {
		t.Errorf("Patterns = %v, want [apps/* tools/*]", info.Patterns)
	}
}

func TestResolveMembers_Npm(t *testing.T) {
	info := WorkspaceInfo{Kind: WorkspaceKindNpmOrYarn, Patterns: []string{"packages/*"}}
	got, err := ResolveMembers("../../../testdata/node/workspace-npm", info)
	if err != nil {
		t.Fatalf("ResolveMembers returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ResolveMembers = %v, want 2 members", got)
	}
	for _, member := range got {
		if filepath.Base(member) != "app-a" && filepath.Base(member) != "app-b" {
			t.Errorf("unexpected member %q", member)
		}
	}
}

func TestResolveMembers_Pnpm(t *testing.T) {
	info := WorkspaceInfo{Kind: WorkspaceKindPnpm, Patterns: []string{"packages/*"}}
	got, err := ResolveMembers("../../../testdata/node/workspace-pnpm", info)
	if err != nil {
		t.Fatalf("ResolveMembers returned error: %v", err)
	}
	if len(got) != 1 || filepath.Base(got[0]) != "lib" {
		t.Fatalf("ResolveMembers = %v, want [.../packages/lib]", got)
	}
}

func TestResolveMembers_NegatedPatternIsSkippedNotSubtracted(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"kept", "excluded"} {
		dir := filepath.Join(root, "packages", name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, manifestFile), []byte(`{"name":"`+name+`"}`), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}

	info := WorkspaceInfo{Kind: WorkspaceKindNpmOrYarn, Patterns: []string{"packages/*", "!packages/excluded"}}
	got, err := ResolveMembers(root, info)
	if err != nil {
		t.Fatalf("ResolveMembers returned error: %v", err)
	}
	// Negation isn't supported in this MVP (§19.2): the "!" pattern is
	// skipped rather than applied, so both members still show up. This
	// documents the limitation rather than silently mismatching it.
	if len(got) != 2 {
		t.Fatalf("ResolveMembers = %v, want both members (negation unsupported)", got)
	}
}

func TestResolveMembers_IgnoresNonPackageDirectories(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "packages", "not-a-package"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	info := WorkspaceInfo{Kind: WorkspaceKindNpmOrYarn, Patterns: []string{"packages/*"}}
	got, err := ResolveMembers(root, info)
	if err != nil {
		t.Fatalf("ResolveMembers returned error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("ResolveMembers = %v, want no members (no package.json)", got)
	}
}

func TestDetectWorkspaceArtifacts_SharedAndOwnNodeModules(t *testing.T) {
	info := WorkspaceInfo{Kind: WorkspaceKindNpmOrYarn, Patterns: []string{"packages/*"}}
	rootArtifacts, members, err := DetectWorkspaceArtifacts("../../../testdata/node/workspace-npm", info)
	if err != nil {
		t.Fatalf("DetectWorkspaceArtifacts returned error: %v", err)
	}

	if len(rootArtifacts) != 1 || rootArtifacts[0].Type != domain.ResourceTypeNodeModules {
		t.Fatalf("rootArtifacts = %+v, want exactly the root node_modules", rootArtifacts)
	}
	if !rootArtifacts[0].Regenerable {
		t.Error("root node_modules should be Regenerable (root has a lockfile)")
	}

	if len(members) != 2 {
		t.Fatalf("members = %+v, want 2", members)
	}

	byName := map[string]MemberArtifacts{}
	for _, member := range members {
		byName[filepath.Base(member.MemberRoot)] = member
	}

	appA, ok := byName["app-a"]
	if !ok {
		t.Fatal("missing app-a in members")
	}
	if len(appA.OwnArtifacts) != 0 {
		t.Errorf("app-a OwnArtifacts = %+v, want none", appA.OwnArtifacts)
	}
	if !appA.SharesRootNodeModules {
		t.Error("app-a should share the root's node_modules (has none of its own)")
	}

	appB, ok := byName["app-b"]
	if !ok {
		t.Fatal("missing app-b in members")
	}
	if len(appB.OwnArtifacts) != 1 || appB.OwnArtifacts[0].Type != domain.ResourceTypeNodeModules {
		t.Fatalf("app-b OwnArtifacts = %+v, want its own node_modules", appB.OwnArtifacts)
	}
	if appB.SharesRootNodeModules {
		t.Error("app-b has its own node_modules, should not report sharing the root's")
	}
	if !appB.OwnArtifacts[0].Regenerable {
		t.Error("app-b node_modules should be Regenerable via the workspace root's lockfile, even without its own")
	}
}

func TestDetectMemberArtifacts_FallsBackToWorkspaceRootLockfile(t *testing.T) {
	got, err := DetectMemberArtifacts(
		"../../../testdata/node/workspace-npm/packages/app-b",
		"../../../testdata/node/workspace-npm",
	)
	if err != nil {
		t.Fatalf("DetectMemberArtifacts returned error: %v", err)
	}
	if len(got) != 1 || !got[0].Regenerable {
		t.Fatalf("DetectMemberArtifacts = %+v, want Regenerable node_modules", got)
	}
}
