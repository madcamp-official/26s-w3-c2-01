package xcodeproj

import (
	"context"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/scanner"
)

const workspaceSuffix = ".xcworkspace"

// WorkspaceDetector recognizes an .xcworkspace bundle as a Workspace
// grouping, resolving its member .xcodeproj references from
// contents.xcworkspacedata the same way msbuild's solution parser resolves
// .sln project references.
type WorkspaceDetector struct{}

func (WorkspaceDetector) CanDetect(entry scanner.Entry) bool {
	return entry.Kind == scanner.EntryDirectory && strings.HasSuffix(entry.Path, workspaceSuffix)
}

// WorkspaceResult pairs the detected Workspace with the manifest paths
// (project.pbxproj) of member projects it references, resolved eagerly here
// so the caller doesn't need to know xcworkspacedata's XML shape.
type WorkspaceResult struct {
	Workspace    domain.Workspace
	ProjectPaths []string
}

func (WorkspaceDetector) Detect(_ context.Context, entry scanner.Entry) (WorkspaceResult, error) {
	name := strings.TrimSuffix(filepath.Base(entry.Path), workspaceSuffix)
	workspace := domain.Workspace{
		Name:         name,
		Type:         domain.WorkspaceTypeXcodeWorkspace,
		ManifestPath: filepath.Join(entry.Path, "contents.xcworkspacedata"),
	}
	members, err := resolveMembers(entry.Path)
	if err != nil {
		return WorkspaceResult{Workspace: workspace}, err
	}
	return WorkspaceResult{Workspace: workspace, ProjectPaths: members}, nil
}

// xcworkspaceLocationPrefixes are the URI-like schemes contents.xcworkspacedata
// uses before the actual path in a FileRef's location attribute. They are all
// stripped and the remainder joined onto the workspace's parent directory,
// which is correct for the top-level flat case handled here. NOTE: once
// nested <Group> support is added, group:/container:/self: can no longer be
// treated identically -- group: is relative to the enclosing Group's own
// path, container: to the workspace container -- so per-element base paths
// will need to be tracked rather than always joining onto one base.
var xcworkspaceLocationPrefixes = []string{"group:", "container:", "self:", "absolute:"}

// resolveMembers parses contents.xcworkspacedata for top-level <FileRef
// location="..."/> entries pointing at .xcodeproj bundles. Nested <Group>
// elements (used to organize large workspaces into folders in Xcode's UI)
// are not descended into -- a real but narrower gap than accepting the flat
// case, which covers the common single-app-plus-CocoaPods-pods shape.
func resolveMembers(workspacePath string) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(workspacePath, "contents.xcworkspacedata"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var doc struct {
		FileRefs []struct {
			Location string `xml:"location,attr"`
		} `xml:"FileRef"`
	}
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}

	base := filepath.Dir(workspacePath)
	var manifests []string
	for _, ref := range doc.FileRefs {
		location := ref.Location
		for _, prefix := range xcworkspaceLocationPrefixes {
			location = strings.TrimPrefix(location, prefix)
		}
		if !strings.HasSuffix(location, projectSuffix) {
			continue
		}
		resolved := location
		if !filepath.IsAbs(resolved) {
			resolved = filepath.Join(base, location)
		}
		manifests = append(manifests, filepath.Join(resolved, "project.pbxproj"))
	}
	return manifests, nil
}
