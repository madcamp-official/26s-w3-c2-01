package xcodeproj

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/scanner"
)

const sampleWorkspaceData = `<?xml version="1.0" encoding="UTF-8"?>
<Workspace
   version = "1.0">
   <FileRef
      location = "group:MyApp.xcodeproj">
   </FileRef>
   <FileRef
      location = "group:Pods/Pods.xcodeproj">
   </FileRef>
</Workspace>
`

func TestWorkspaceDetectorResolvesFlatFileRefMembers(t *testing.T) {
	root := t.TempDir()
	workspacePath := filepath.Join(root, "MyApp.xcworkspace")
	if err := os.Mkdir(workspacePath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspacePath, "contents.xcworkspacedata"), []byte(sampleWorkspaceData), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := (WorkspaceDetector{}).Detect(context.Background(), scanner.Entry{Path: workspacePath, Kind: scanner.EntryDirectory})
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if result.Workspace.Type != domain.WorkspaceTypeXcodeWorkspace || result.Workspace.Name != "MyApp" {
		t.Fatalf("workspace = %#v", result.Workspace)
	}
	want := []string{
		filepath.Join(root, "MyApp.xcodeproj", "project.pbxproj"),
		filepath.Join(root, "Pods", "Pods.xcodeproj", "project.pbxproj"),
	}
	if len(result.ProjectPaths) != len(want) {
		t.Fatalf("ProjectPaths = %#v, want %#v", result.ProjectPaths, want)
	}
	for i, p := range want {
		if result.ProjectPaths[i] != p {
			t.Errorf("ProjectPaths[%d] = %q, want %q", i, result.ProjectPaths[i], p)
		}
	}
}

func TestWorkspaceDetectorReturnsNoMembersWhenDataFileMissing(t *testing.T) {
	root := t.TempDir()
	workspacePath := filepath.Join(root, "Empty.xcworkspace")
	if err := os.Mkdir(workspacePath, 0o755); err != nil {
		t.Fatal(err)
	}
	result, err := (WorkspaceDetector{}).Detect(context.Background(), scanner.Entry{Path: workspacePath, Kind: scanner.EntryDirectory})
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if len(result.ProjectPaths) != 0 {
		t.Fatalf("ProjectPaths = %#v, want none", result.ProjectPaths)
	}
}
