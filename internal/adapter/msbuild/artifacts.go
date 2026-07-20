package msbuild

import (
	"os"
	"path/filepath"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

// 이 파일은 msbuild 패키지에서 "SDK/도구 의존성"이 아니라 "빌드 산출물(bin/obj)" 탐지만
// 따로 떼어낸 파일이다. 프로젝트 루트 바로 아래에서 이름이 bin/obj인 디렉터리를 찾아
// domain.ResourceTypeBuildOutput으로 보고하는데, 실제 MSBuild의 OutputPath/
// IntermediateOutputPath 설정을 파싱하는 게 아니라 디렉터리 이름만 보고 판단하므로
// Confidence를 INFERRED 수준(낮음)으로 매긴다. 이 이름-매칭 방식과 신뢰도 상수는
// internal/adapter/node의 동일한 로직을 그대로 따른 것이며(§19.3), SDK 버전 매칭을 다루는
// resolve.go와는 목적이 달라 별도 파일로 분리되어 있다.

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
