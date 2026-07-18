package msbuild

import (
	"os"
	"path/filepath"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

// artifactDirs maps recognized build artifact directory names, immediately
// under a build project root, to the domain.ResourceType they become. This
// mirrors internal/adapter/node's artifactDirs map for the same reasoning
// (§19.3): name match only, no OutputPath/build config parsing yet.
var artifactDirs = map[string]domain.ResourceType{
	"bin": domain.ResourceTypeBuildOutput,
	"obj": domain.ResourceTypeBuildOutput,
}

// confidenceInferredBuildOutput matches internal/adapter/node's placeholder
// of the same name and rationale: a directory-name match alone is
// INFERRED-strength evidence, pending the team's shared Confidence formula
// (docs/libra_integration_contracts.md §20.2, still DECISION_REQUIRED).
const confidenceInferredBuildOutput = 30

// DetectArtifacts finds recognized build-output directories (bin, obj)
// immediately under a build project's root.
func DetectArtifacts(root string) ([]domain.Resource, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	var resources []domain.Resource
	for _, entry := range entries {
		if !entry.IsDir() {
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
