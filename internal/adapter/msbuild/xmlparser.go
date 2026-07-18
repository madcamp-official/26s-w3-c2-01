package msbuild

import (
	"context"
	"encoding/xml"
	"os"
	"path/filepath"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/scanner"
)

// xmlProperty captures one child element of a PropertyGroup as a name/value
// pair, whatever the element happens to be called (e.g.
// <WindowsTargetPlatformVersion>10.0.22621.0</WindowsTargetPlatformVersion>).
type xmlProperty struct {
	XMLName xml.Name
	Value   string `xml:",chardata"`
}

// xmlPropertyGroup mirrors MSBuild's <PropertyGroup>: an arbitrary set of
// named properties.
type xmlPropertyGroup struct {
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

	var declared []DeclaredProperty
	for _, group := range file.PropertyGroups {
		for _, prop := range group.Properties {
			declared = append(declared, DeclaredProperty{
				Name:  prop.XMLName.Local,
				Value: prop.Value,
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
			Path:           root,
			Type:           projectType,
			Drive:          drive,
			LastModifiedAt: entry.ModifiedAt,
		},
		Declared: declared,
	}}, nil
}
