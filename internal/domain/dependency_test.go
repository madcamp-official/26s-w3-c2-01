package domain

import "testing"

func TestDependencyIDGolden(t *testing.T) {
	got := DependencyID(NodeProject, "project-1", RelationRequires, NodeResource, "resource-1")
	const want = "8e9c6d6dd0e41178ed235ba637506f98e3e3201cec4da96badbb5d0ebb77ab54"
	if got != want {
		t.Fatalf("DependencyID() = %q, want %q", got, want)
	}
}

func TestDependencyIDIncludesEndpointTypes(t *testing.T) {
	projectResource := DependencyID(NodeProject, "same", RelationRequires, NodeResource, "target")
	resourceProject := DependencyID(NodeResource, "same", RelationRequires, NodeProject, "target")
	if projectResource == resourceProject {
		t.Fatal("DependencyID() collided for different endpoint types")
	}
}
