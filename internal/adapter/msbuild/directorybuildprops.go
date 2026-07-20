package msbuild

import (
	"encoding/xml"
	"os"
	"path/filepath"
)

const directoryBuildPropsFilename = "Directory.Build.props"

// findDirectoryBuildProps walks up from projectDir through its ancestors and
// returns the path of the nearest Directory.Build.props it finds. found is
// false if none exists up to the filesystem root -- that is not an error,
// just the common case for a project with no solution-wide props file.
//
// Only the nearest file is used. A Directory.Build.props that itself
// explicitly imports a further ancestor's Directory.Build.props (a pattern
// some templates use) is not followed -- that would require evaluating
// arbitrary MSBuild <Import> expressions, which this parser does not do.
func findDirectoryBuildProps(projectDir string) (path string, found bool, err error) {
	dir := projectDir
	for {
		candidate := filepath.Join(dir, directoryBuildPropsFilename)
		info, statErr := os.Stat(candidate)
		if statErr == nil && !info.IsDir() {
			return candidate, true, nil
		}
		if statErr != nil && !os.IsNotExist(statErr) {
			return "", false, statErr
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false, nil
		}
		dir = parent
	}
}

// parseDirectoryBuildProps reads a Directory.Build.props file and returns
// its declared properties, each tagged with path as its SourcePath. It reads
// the same <Project><PropertyGroup> shape as a .vcxproj/.csproj, since
// Directory.Build.props is an ordinary MSBuild project file.
func parseDirectoryBuildProps(path string) ([]DeclaredProperty, error) {
	data, err := os.ReadFile(path)
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
				Name:       prop.XMLName.Local,
				Value:      prop.Value,
				Condition:  group.Condition,
				SourcePath: path,
			})
		}
	}
	return declared, nil
}

// mergeInheritedProperties combines a project's own declared properties with
// properties inherited from Directory.Build.props, applying MSBuild's
// override rule: a property the project file declares itself always wins
// over an inherited declaration of the same name, regardless of Condition on
// either side. This deliberately doesn't replicate MSBuild's real
// Import-position-based evaluation order (which this parser doesn't track)
// -- "the project's own file wins" is a simpler, always-correct-in-practice
// approximation, since dropping a shadowed inherited declaration is what
// prevents ResolveDependencies from producing two conflicting Dependency
// edges for the same property name.
func mergeInheritedProperties(own, inherited []DeclaredProperty) []DeclaredProperty {
	ownNames := make(map[string]bool, len(own))
	for _, p := range own {
		ownNames[p.Name] = true
	}

	merged := make([]DeclaredProperty, 0, len(own)+len(inherited))
	for _, p := range inherited {
		if !ownNames[p.Name] {
			merged = append(merged, p)
		}
	}
	merged = append(merged, own...)
	return merged
}
