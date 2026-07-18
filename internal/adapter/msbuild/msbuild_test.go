package msbuild

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/scanner"
)

// fakeWorkspaceParser is a placeholder WorkspaceParser used to validate this
// test harness before the real .sln-parsing implementation lands.
type fakeWorkspaceParser struct{}

func (fakeWorkspaceParser) CanParse(entry scanner.Entry) bool {
	return filepath.Ext(entry.Path) == ".sln"
}

func (fakeWorkspaceParser) Parse(ctx context.Context, entry scanner.Entry) (ParsedWorkspace, error) {
	switch filepath.Base(entry.Path) {
	case "GameClient.sln":
		return ParsedWorkspace{
			Workspace:    domain.Workspace{Name: "GameClient", ManifestPath: entry.Path, Type: domain.WorkspaceTypeVSSolution},
			ProjectPaths: []string{filepath.Join(filepath.Dir(entry.Path), "GameClient.vcxproj")},
		}, nil
	default:
		return ParsedWorkspace{}, nil
	}
}

func TestXMLBuildProjectParser_CanParse(t *testing.T) {
	var parser BuildProjectParser = XMLBuildProjectParser{}

	cases := []struct {
		name string
		path string
		want bool
	}{
		{"cpp project", "../../../testdata/msbuild/GameClient/GameClient.vcxproj", true},
		{"dotnet project", "../../../testdata/msbuild/SampleDotNetApp/SampleDotNetApp.csproj", true},
		{"solution file is not a build project", "../../../testdata/msbuild/GameClient/GameClient.sln", false},
		{"unrelated file", "../../../testdata/msbuild/GameClient/Directory.Build.props", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parser.CanParse(scanner.Entry{Path: tc.path}); got != tc.want {
				t.Errorf("CanParse(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

func TestXMLBuildProjectParser_Parse(t *testing.T) {
	var parser BuildProjectParser = XMLBuildProjectParser{}
	// Deliberately not the fixture's real mtime: proves Parse reuses the
	// entry's ModifiedAt instead of re-stat'ing the filesystem.
	modTime := time.Date(2026, 7, 18, 3, 4, 5, 0, time.UTC)

	cases := []struct {
		name         string
		path         string
		wantProject  string
		wantType     domain.ProjectType
		wantDeclared []DeclaredProperty
	}{
		{
			name:        "cpp project declares its Windows SDK and toolset properties",
			path:        "../../../testdata/msbuild/GameClient/GameClient.vcxproj",
			wantProject: "GameClient",
			wantType:    domain.ProjectTypeMSBuildCpp,
			wantDeclared: []DeclaredProperty{
				{Name: "ProjectGuid", Value: "{11111111-2222-3333-4444-555555555555}"},
				{Name: "WindowsTargetPlatformVersion", Value: "10.0.22621.0"},
				{Name: "PlatformToolset", Value: "v143"},
				{Name: "ConfigurationType", Value: "Application"},
				{Name: "UseDebugLibraries", Value: "true"},
			},
		},
		{
			name:        "dotnet project declares its output type and target framework",
			path:        "../../../testdata/msbuild/SampleDotNetApp/SampleDotNetApp.csproj",
			wantProject: "SampleDotNetApp",
			wantType:    domain.ProjectTypeMSBuildDotNet,
			wantDeclared: []DeclaredProperty{
				{Name: "OutputType", Value: "Exe"},
				{Name: "TargetFramework", Value: "net8.0"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parser.Parse(context.Background(), scanner.Entry{Path: tc.path, ModifiedAt: modTime})
			if err != nil {
				t.Fatalf("Parse(%q) returned error: %v", tc.path, err)
			}
			if len(got) != 1 {
				t.Fatalf("got %d parsed build projects, want 1: %+v", len(got), got)
			}
			if got[0].Project.Name != tc.wantProject {
				t.Errorf("Project.Name = %q, want %q", got[0].Project.Name, tc.wantProject)
			}
			if got[0].Project.Type != tc.wantType {
				t.Errorf("Project.Type = %v, want %v", got[0].Project.Type, tc.wantType)
			}
			if !reflect.DeepEqual(got[0].Declared, tc.wantDeclared) {
				t.Errorf("Declared = %+v, want %+v", got[0].Declared, tc.wantDeclared)
			}
			if !got[0].Project.LastModifiedAt.Equal(modTime) {
				t.Errorf("Project.LastModifiedAt = %v, want %v (reused from scanner.Entry)", got[0].Project.LastModifiedAt, modTime)
			}
		})
	}
}

func TestWorkspaceParser_CanParse(t *testing.T) {
	var parser WorkspaceParser = fakeWorkspaceParser{}

	cases := []struct {
		name string
		path string
		want bool
	}{
		{"solution file", "../../../testdata/msbuild/GameClient/GameClient.sln", true},
		{"cpp project is not a workspace", "../../../testdata/msbuild/GameClient/GameClient.vcxproj", false},
		{"dotnet project is not a workspace", "../../../testdata/msbuild/SampleDotNetApp/SampleDotNetApp.csproj", false},
		{"unrelated file", "../../../testdata/msbuild/GameClient/Directory.Build.props", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parser.CanParse(scanner.Entry{Path: tc.path}); got != tc.want {
				t.Errorf("CanParse(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

func TestWorkspaceParser_Parse(t *testing.T) {
	var parser WorkspaceParser = fakeWorkspaceParser{}

	path := "../../../testdata/msbuild/GameClient/GameClient.sln"
	got, err := parser.Parse(context.Background(), scanner.Entry{Path: path})
	if err != nil {
		t.Fatalf("Parse(%q) returned error: %v", path, err)
	}
	if got.Workspace.Name != "GameClient" {
		t.Errorf("Workspace.Name = %q, want %q", got.Workspace.Name, "GameClient")
	}
	wantPaths := []string{filepath.Join(filepath.Dir(path), "GameClient.vcxproj")}
	if !reflect.DeepEqual(got.ProjectPaths, wantPaths) {
		t.Errorf("ProjectPaths = %+v, want %+v", got.ProjectPaths, wantPaths)
	}
}
