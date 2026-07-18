package msbuild

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

// fakeBuildProjectParser is a placeholder BuildProjectParser used to
// validate this test harness before the real XML-parsing implementation
// lands (Day2/3). It only knows about the fixtures under testdata/msbuild.
type fakeBuildProjectParser struct{}

func (fakeBuildProjectParser) CanParse(path string) bool {
	switch filepath.Ext(path) {
	case ".vcxproj", ".csproj":
		return true
	default:
		return false
	}
}

func (fakeBuildProjectParser) Parse(ctx context.Context, path string) (ParsedBuildProject, error) {
	info, err := os.Stat(path)
	if err != nil {
		return ParsedBuildProject{}, err
	}

	switch filepath.Base(path) {
	case "GameClient.vcxproj":
		return ParsedBuildProject{
			Project: domain.BuildProject{Name: "GameClient", Path: path, Type: domain.ProjectTypeMSBuildCpp, LastModifiedAt: info.ModTime()},
			Declared: []DeclaredProperty{
				{Name: "WindowsTargetPlatformVersion", Value: "10.0.22621.0"},
				{Name: "PlatformToolset", Value: "v143"},
			},
		}, nil
	case "SampleDotNetApp.csproj":
		return ParsedBuildProject{
			Project: domain.BuildProject{Name: "SampleDotNetApp", Path: path, Type: domain.ProjectTypeMSBuildDotNet, LastModifiedAt: info.ModTime()},
			Declared: []DeclaredProperty{
				{Name: "TargetFramework", Value: "net8.0"},
			},
		}, nil
	default:
		return ParsedBuildProject{}, nil
	}
}

// fakeWorkspaceParser is a placeholder WorkspaceParser, same purpose as
// fakeBuildProjectParser above but for .sln files.
type fakeWorkspaceParser struct{}

func (fakeWorkspaceParser) CanParse(path string) bool {
	return filepath.Ext(path) == ".sln"
}

func (fakeWorkspaceParser) Parse(ctx context.Context, path string) (ParsedWorkspace, error) {
	switch filepath.Base(path) {
	case "GameClient.sln":
		return ParsedWorkspace{
			Workspace:    domain.Workspace{Name: "GameClient", Path: path, Type: domain.WorkspaceTypeVSSolution},
			ProjectPaths: []string{filepath.Join(filepath.Dir(path), "GameClient.vcxproj")},
		}, nil
	default:
		return ParsedWorkspace{}, nil
	}
}

func TestBuildProjectParser_CanParse(t *testing.T) {
	var parser BuildProjectParser = fakeBuildProjectParser{}

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
			if got := parser.CanParse(tc.path); got != tc.want {
				t.Errorf("CanParse(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

func TestBuildProjectParser_Parse(t *testing.T) {
	var parser BuildProjectParser = fakeBuildProjectParser{}

	cases := []struct {
		name         string
		path         string
		wantProject  string
		wantDeclared []DeclaredProperty
	}{
		{
			name:        "cpp project declares a Windows SDK version",
			path:        "../../../testdata/msbuild/GameClient/GameClient.vcxproj",
			wantProject: "GameClient",
			wantDeclared: []DeclaredProperty{
				{Name: "WindowsTargetPlatformVersion", Value: "10.0.22621.0"},
				{Name: "PlatformToolset", Value: "v143"},
			},
		},
		{
			name:        "dotnet project declares a target framework",
			path:        "../../../testdata/msbuild/SampleDotNetApp/SampleDotNetApp.csproj",
			wantProject: "SampleDotNetApp",
			wantDeclared: []DeclaredProperty{
				{Name: "TargetFramework", Value: "net8.0"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parser.Parse(context.Background(), tc.path)
			if err != nil {
				t.Fatalf("Parse(%q) returned error: %v", tc.path, err)
			}
			if got.Project.Name != tc.wantProject {
				t.Errorf("Project.Name = %q, want %q", got.Project.Name, tc.wantProject)
			}
			if !reflect.DeepEqual(got.Declared, tc.wantDeclared) {
				t.Errorf("Declared = %+v, want %+v", got.Declared, tc.wantDeclared)
			}
			if got.Project.LastModifiedAt.IsZero() {
				t.Errorf("Project.LastModifiedAt is zero, want the fixture file's mod time")
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
			if got := parser.CanParse(tc.path); got != tc.want {
				t.Errorf("CanParse(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

func TestWorkspaceParser_Parse(t *testing.T) {
	var parser WorkspaceParser = fakeWorkspaceParser{}

	path := "../../../testdata/msbuild/GameClient/GameClient.sln"
	got, err := parser.Parse(context.Background(), path)
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
