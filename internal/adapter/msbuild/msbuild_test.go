package msbuild

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

// fakeProjectParser is a placeholder ProjectParser used to validate this test
// harness before the real XML-parsing implementation lands (Day2/3). It only
// knows about the fixtures under testdata/msbuild.
type fakeProjectParser struct{}

func (fakeProjectParser) CanParse(path string) bool {
	switch filepath.Ext(path) {
	case ".sln", ".vcxproj", ".csproj":
		return true
	default:
		return false
	}
}

func (fakeProjectParser) Parse(ctx context.Context, path string) (ParsedProject, error) {
	switch filepath.Base(path) {
	case "GameClient.vcxproj":
		return ParsedProject{
			Project: domain.Project{Name: "GameClient", Path: path, Type: domain.ProjectTypeMSBuildCpp},
			Declared: []DeclaredProperty{
				{Name: "WindowsTargetPlatformVersion", Value: "10.0.22621.0"},
				{Name: "PlatformToolset", Value: "v143"},
			},
		}, nil
	case "SampleDotNetApp.csproj":
		return ParsedProject{
			Project: domain.Project{Name: "SampleDotNetApp", Path: path, Type: domain.ProjectTypeMSBuildDotNet},
			Declared: []DeclaredProperty{
				{Name: "TargetFramework", Value: "net8.0"},
			},
		}, nil
	default:
		return ParsedProject{}, nil
	}
}

func TestProjectParser_CanParse(t *testing.T) {
	var parser ProjectParser = fakeProjectParser{}

	cases := []struct {
		name string
		path string
		want bool
	}{
		{"solution file", "../../../testdata/msbuild/GameClient/GameClient.sln", true},
		{"cpp project", "../../../testdata/msbuild/GameClient/GameClient.vcxproj", true},
		{"dotnet project", "../../../testdata/msbuild/SampleDotNetApp/SampleDotNetApp.csproj", true},
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

func TestProjectParser_Parse(t *testing.T) {
	var parser ProjectParser = fakeProjectParser{}

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
		})
	}
}
