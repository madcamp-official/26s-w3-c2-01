package domain

import "testing"

func TestProjectIDGolden(t *testing.T) {
	got := ProjectID(ProjectTypeMSBuildCpp, `d:\game\client\client.vcxproj`)
	const want = "30255d30ccb1fae8701d1a78aa310505d7010d8170ee88ce7be9e5c648ee067a"
	if got != want {
		t.Fatalf("ProjectID() = %q, want %q", got, want)
	}
}

func TestWorkspaceIDGolden(t *testing.T) {
	got := WorkspaceID(WorkspaceTypeVSSolution, `d:\game\game.sln`)
	const want = "07c80af8fd122cb5fd12517f1a46aaad2771cd212425399c02791ce948a1ba4d"
	if got != want {
		t.Fatalf("WorkspaceID() = %q, want %q", got, want)
	}
}

func TestProjectIDIncludesProjectType(t *testing.T) {
	path := `d:\game\package.json`
	if ProjectID(ProjectTypeNode, path) == ProjectID(ProjectTypeGit, path) {
		t.Fatal("ProjectID() collided for different project types")
	}
}
