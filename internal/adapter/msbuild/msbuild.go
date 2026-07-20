// Package msbuild detects and parses MSBuild C++/.NET projects
// (.vcxproj/.csproj), locates Visual Studio and MSBuild installations via
// vswhere.exe, and matches declared SDK properties against installed
// resources. It also declares (but does not yet implement) parsing for
// Visual Studio Solutions (.sln) -- see WorkspaceParser and xmlparser.go's
// note below. Split by concern across several files:
//
//   - msbuild.go (this file): the shared contract types every other file in
//     this package implements against (BuildProjectParser, WorkspaceParser,
//     ToolLocator, DeclaredProperty).
//   - xmlparser.go: BuildProjectParser's real implementation -- reads
//     .vcxproj/.csproj XML. No WorkspaceParser (.sln) implementation exists
//     anywhere in this package yet, despite this file declaring the
//     interface -- corrected here after a review pass caught this doc
//     comment overclaiming it.
//   - root.go: project-root/drive determination shared by the parsers.
//   - version.go: SDK/TargetFramework version string parsing and comparison.
//   - resolve.go: matches a DeclaredProperty against installed resources to
//     produce a domain.Dependency + domain.Evidence pair (not yet called by
//     any production code path -- see issue #22).
//   - vswhere.go: ToolLocator's real implementation, shelling out to
//     vswhere.exe.
//   - artifacts.go: MSBuild build-artifact (bin/obj) detection.
package msbuild

import (
	"context"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/scanner"
)

// 이 파일(msbuild.go) 자체의 역할: 패키지 doc 주석에서 이미 7개 파일의 역할을 목록으로
// 정리했지만, 정작 이 파일이 하는 일은 그 7개 파일이 서로 의존 없이 맞물릴 수 있도록 하는
// "계약(contract)"만 정의하는 것이다. 즉 ToolLocator/BuildProjectParser/WorkspaceParser
// 인터페이스와 DeclaredProperty/ParsedBuildProject/ParsedWorkspace 값 타입이 전부이고,
// 실제 로직(XML 파싱, vswhere.exe 실행, 버전 비교, 루트 경로 유도, 산출물 탐지, SDK 매칭)은
// 단 한 줄도 없다. msbuild는 ".vcxproj/.csproj/.sln 파싱 -> 선언된 속성 추출 -> 설치된
// 도구/SDK 탐지 -> 둘을 매칭해 Dependency 생성"까지 단계가 많고 각 단계가 독립적으로
// 테스트 가능해야 하므로, 이 파일이 정의하는 인터페이스를 기준으로 나머지 6개 파일이 각자
// 하나의 관심사만 맡도록 쪼개져 있다.

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
	// Condition is the MSBuild Condition attribute on the PropertyGroup this
	// property came from (e.g. Condition="'$(Configuration)|$(Platform)'
	// =='Debug|x64'"). Empty if the PropertyGroup was unconditional.
	Condition string
}

// ParsedBuildProject is the result of parsing a single .vcxproj or .csproj
// file.
type ParsedBuildProject struct {
	Project  domain.BuildProject
	Declared []DeclaredProperty
}

// BuildProjectParser detects and parses .vcxproj and .csproj files,
// including properties inherited from Directory.Build.props. It takes
// scanner.Entry, rather than a bare path, so it can reuse the file metadata
// (size, modified time) the scanner already collected while walking instead
// of re-querying the filesystem for it.
type BuildProjectParser interface {
	// CanParse reports whether entry is a project file this parser handles.
	CanParse(entry scanner.Entry) bool
	// Parse reads the project file at entry.Path and returns the detected
	// build project(s) along with any declared properties relevant to
	// dependency analysis. It returns a slice, rather than a single
	// ParsedBuildProject, so that a project file describing more than one
	// build project is not precluded by the return type.
	Parse(ctx context.Context, entry scanner.Entry) ([]ParsedBuildProject, error)
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

// WorkspaceParser detects and parses .sln files. It takes scanner.Entry for
// the same reason as BuildProjectParser: to reuse metadata the scanner
// already collected instead of re-querying the filesystem.
type WorkspaceParser interface {
	// CanParse reports whether entry is a workspace file this parser handles.
	CanParse(entry scanner.Entry) bool
	// Parse reads the workspace file at entry.Path and returns the detected
	// workspace along with the paths of the build projects it references.
	Parse(ctx context.Context, entry scanner.Entry) (ParsedWorkspace, error)
}
