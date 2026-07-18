package msbuild

import (
	"context"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

// ToolLocator finds Visual Studio and MSBuild installations, typically via
// vswhere.exe, and reports them as domain.Resource values
// (Type == domain.ResourceTypeVisualStudio or domain.ResourceTypeMSBuild).
type ToolLocator interface {
	Locate(ctx context.Context) ([]domain.Resource, error)
}

// DeclaredProperty is a raw property read from a project file, before it has
// been resolved against any installed Resource. For example,
// WindowsTargetPlatformVersion = "10.0.22621.0" read from a .vcxproj still
// needs to be matched against a detected Windows SDK before it becomes a
// Dependency backed by Evidence.
// .vcxproj 파일 안에는 <WindowsTargetPlatformVersion>10.0.22621.0</WindowsTargetPlatformVersion> 같은 XML 속성들이 있는데, 이걸 그냥 "이름-값 쌍"으로 담는 작은 데이터 상자
type DeclaredProperty struct {
	Name  string
	Value string
}

// ParsedBuildProject is the result of parsing a single .vcxproj or .csproj
// file.
type ParsedBuildProject struct {
	Project  domain.BuildProject
	Declared []DeclaredProperty
}

// BuildProjectParser detects and parses .vcxproj and .csproj files,
// including properties inherited from Directory.Build.props.
type BuildProjectParser interface {
	// CanParse reports whether path is a project file this parser handles.
	CanParse(path string) bool
	// Parse reads the project file at path and returns the detected build
	// project(s) along with any declared properties relevant to dependency
	// analysis. It returns a slice, rather than a single ParsedBuildProject,
	// so that a project file describing more than one build project is not
	// precluded by the return type.
	Parse(ctx context.Context, path string) ([]ParsedBuildProject, error)
}

// ParsedWorkspace is the result of parsing a single .sln file: the workspace
// itself, plus the paths of the build projects it references. Those paths
// are not yet resolved to BuildProject IDs -- that resolution happens once
// every referenced path has been scanned and parsed on its own (see
// domain.WorkspaceProject).
type ParsedWorkspace struct {
	Workspace    domain.Workspace
	ProjectPaths []string
}

// WorkspaceParser detects and parses .sln files.
type WorkspaceParser interface {
	// CanParse reports whether path is a workspace file this parser handles.
	CanParse(path string) bool
	// Parse reads the workspace file at path and returns the detected
	// workspace along with the paths of the build projects it references.
	Parse(ctx context.Context, path string) (ParsedWorkspace, error)
}
