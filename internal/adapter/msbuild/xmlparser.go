package msbuild

import (
	"context"
	"encoding/xml"
	"os"
	"path/filepath"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/scanner"
)

// 이 파일은 msbuild.go가 정의하는 BuildProjectParser 인터페이스의 실제 구현체
// (XMLBuildProjectParser)로, .vcxproj/.csproj 파일의 <PropertyGroup> XML을 읽어
// DeclaredProperty 목록과 domain.BuildProject를 만들어낸다. Directory.Build.props 같은
// import로 상속되는 속성은 해석하지 않고 파일 자체가 선언한 것만 다룬다. Condition이 걸린
// PropertyGroup도 그대로 수집만 해서 Condition 문자열과 함께 넘기고, "이 조건을 평가해서
// 어떤 값을 쓸지"는 이 파일이 아니라 resolve.go(정책 계층)의 몫으로 남겨둔다. 참고로
// msbuild.go의 패키지 doc 주석은 이 파일이 WorkspaceParser(.sln)도 구현한다고 적어두었지만,
// 실제로는 아직 .sln 파싱 구현은 이 파일에도 다른 어디에도 없다 -- 현재는 BuildProjectParser
// 쪽만 채워져 있다.

// xmlProperty captures one child element of a PropertyGroup as a name/value
// pair, whatever the element happens to be called (e.g.
// <WindowsTargetPlatformVersion>10.0.22621.0</WindowsTargetPlatformVersion>).
type xmlProperty struct {
	XMLName xml.Name
	Value   string `xml:",chardata"`
}

// xmlPropertyGroup mirrors MSBuild's <PropertyGroup>: an arbitrary set of
// named properties, optionally gated by a Configuration/Platform Condition
// (e.g. Condition="'$(Configuration)|$(Platform)'=='Debug|x64'").
type xmlPropertyGroup struct {
	Condition  string        `xml:"Condition,attr"`
	Properties []xmlProperty `xml:",any"`
}

// xmlProjectFile is the subset of a .vcxproj/.csproj's structure libra reads:
// every <PropertyGroup> in the file.
type xmlProjectFile struct {
	PropertyGroups []xmlPropertyGroup `xml:"PropertyGroup"`
}

// XMLBuildProjectParser parses .vcxproj and .csproj files by reading every
// <PropertyGroup> in the file. It does not resolve properties inherited from
// Directory.Build.props or other imports -- only what the file itself
// declares.
type XMLBuildProjectParser struct{}

func (XMLBuildProjectParser) CanParse(entry scanner.Entry) bool {
	switch filepath.Ext(entry.Path) {
	case ".vcxproj", ".csproj":
		return true
	default:
		return false
	}
}

func (XMLBuildProjectParser) Parse(ctx context.Context, entry scanner.Entry) ([]ParsedBuildProject, error) {
	data, err := os.ReadFile(entry.Path)
	if err != nil {
		return nil, err
	}

	var file xmlProjectFile
	if err := xml.Unmarshal(data, &file); err != nil {
		return nil, err
	}

	// Conditional PropertyGroups (e.g. Debug/Release- or Platform-specific
	// overrides) are collected like any other, carrying their Condition
	// along. Evaluating the Condition against a specific Configuration/
	// Platform isn't implemented, so the resolver (not this parser) decides
	// what to do with a non-empty Condition -- typically recording it as a
	// domain.UnverifiedScope rather than guessing which configuration's
	// value applies.
	var declared []DeclaredProperty
	for _, group := range file.PropertyGroups {
		for _, prop := range group.Properties {
			declared = append(declared, DeclaredProperty{
				Name:      prop.XMLName.Local,
				Value:     prop.Value,
				Condition: group.Condition,
			})
		}
	}

	root, name, drive, err := ProjectRoot(entry.Path)
	if err != nil {
		return nil, err
	}

	projectType := domain.ProjectTypeMSBuildCpp
	if filepath.Ext(entry.Path) == ".csproj" {
		projectType = domain.ProjectTypeMSBuildDotNet
	}

	return []ParsedBuildProject{{
		Project: domain.BuildProject{
			Name:           name,
			Type:           projectType,
			RootPath:       root,
			ManifestPath:   entry.Path,
			Drive:          drive,
			LastModifiedAt: entry.ModifiedAt,
		},
		Declared: declared,
	}}, nil
}
