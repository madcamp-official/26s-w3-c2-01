package msbuild

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

// artifactDirs maps recognized build artifact directory names, immediately
// under a build project root, to the domain.ResourceType they become. This
// mirrors internal/adapter/node's artifactDirs map for the same reasoning
// (§19.3): a bare name match here, with nothing in declaredOutputProperties
// confirming it, is only ever INFERRED-strength evidence.
var artifactDirs = map[string]domain.ResourceType{
	"bin": domain.ResourceTypeBuildOutput,
	"obj": domain.ResourceTypeBuildOutput,
}

// declaredOutputProperties are the MSBuild properties that name where a
// project's build output goes: .NET SDK-style (.csproj) projects use
// OutputPath/BaseOutputPath/IntermediateOutputPath/BaseIntermediateOutputPath;
// VC++ (.vcxproj) projects use OutDir/IntDir. A project that declares one of
// these is trusted over the bin/obj name guess in artifactDirs -- this
// matters most for VC++, which does not default to "bin"/"obj" the way
// .csproj does, so an undeclared VC++ project matching artifactDirs by name
// alone is a much weaker signal than the same match on a .csproj.
var declaredOutputProperties = map[string]bool{
	"OutputPath":                 true,
	"BaseOutputPath":             true,
	"IntermediateOutputPath":     true,
	"BaseIntermediateOutputPath": true,
	"OutDir":                     true,
	"IntDir":                     true,
}

// confidenceInferredBuildOutput: a directory-name match alone is
// INFERRED-strength evidence, per the CONFIRMED shared Confidence scale
// (docs/libra_integration_contracts.md §20.2, domain.DefaultConfidence).
var confidenceInferredBuildOutput = domain.DefaultConfidence[domain.EvidenceInferred]

// confidenceDeclaredBuildOutput: an output directory read from the
// project's own declared OutputPath/OutDir-family property is
// DECLARED-strength evidence -- the same tier §20.2 gives an exact
// WindowsTargetPlatformVersion match.
var confidenceDeclaredBuildOutput = domain.DefaultConfidence[domain.EvidenceDeclared]

// DetectArtifacts finds a build project's output directories under root: any
// declared OutputPath/OutDir-family property that resolves to a real
// directory there (DECLARED-strength), plus any remaining recognized name
// (bin, obj) not already covered by a declaration (INFERRED-strength).
// declared is the same []DeclaredProperty XMLBuildProjectParser.Parse
// returns for the project -- passing nil falls back to name matching only.
func DetectArtifacts(root string, declared []DeclaredProperty) ([]domain.Resource, error) {
	declaredDirs, err := resolveDeclaredOutputDirs(root, declared)
	if err != nil {
		return nil, err
	}

	resources := make([]domain.Resource, 0, len(declaredDirs))
	coveredNames := make(map[string]bool, len(declaredDirs))
	for _, dir := range declaredDirs {
		resources = append(resources, domain.Resource{
			Name:        dir.name,
			Type:        domain.ResourceTypeBuildOutput,
			DisplayPath: dir.path,
			Regenerable: true,
			Confidence:  confidenceDeclaredBuildOutput,
		})
		coveredNames[dir.name] = true
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() || coveredNames[entry.Name()] {
			continue
		}
		resourceType, recognized := artifactDirs[entry.Name()]
		if !recognized {
			continue
		}
		resources = append(resources, domain.Resource{
			Name:        entry.Name(),
			Type:        resourceType,
			DisplayPath: filepath.Join(root, entry.Name()),
			Regenerable: true,
			Confidence:  confidenceInferredBuildOutput,
		})
	}
	return resources, nil
}

type declaredOutputDir struct {
	name string
	path string
}

// resolveDeclaredOutputDirs resolves each unconditional declaredOutputProperties
// value that doesn't contain an MSBuild macro ("$(...)", e.g.
// "$(SolutionDir)$(Platform)\$(Configuration)\") into a real directory under
// root. A macro can't be evaluated without knowing which configuration is
// being built -- the same reason this package never evaluates Condition --
// so a value containing one is skipped rather than guessed at. A declared
// path outside root, or one that doesn't exist on disk, is also skipped: it
// isn't this scan's job to report a directory that isn't there.
func resolveDeclaredOutputDirs(root string, declared []DeclaredProperty) ([]declaredOutputDir, error) {
	var dirs []declaredOutputDir
	for _, d := range declared {
		if !declaredOutputProperties[d.Name] || d.Condition != "" || strings.Contains(d.Value, "$(") {
			continue
		}
		rel := filepath.Clean(d.Value)
		if filepath.IsAbs(rel) || strings.HasPrefix(rel, "..") {
			continue
		}
		abs := filepath.Join(root, rel)
		info, err := os.Stat(abs)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		if !info.IsDir() {
			continue
		}
		dirs = append(dirs, declaredOutputDir{name: rel, path: abs})
	}
	return dirs, nil
}
