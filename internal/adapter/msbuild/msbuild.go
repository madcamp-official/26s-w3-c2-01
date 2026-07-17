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

// ParsedProject is the result of parsing a single project file.
type ParsedProject struct {
	Project  domain.Project
	Declared []DeclaredProperty
}

// ProjectParser detects and parses .sln, .vcxproj, and .csproj files,
// including properties inherited from Directory.Build.props.
type ProjectParser interface {
	// CanParse reports whether path is a project file this parser handles.
	CanParse(path string) bool
	// Parse reads the project file at path and returns the detected project
	// along with any declared properties relevant to dependency analysis.
	Parse(ctx context.Context, path string) (ParsedProject, error)
}
