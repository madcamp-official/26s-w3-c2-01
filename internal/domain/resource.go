package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

// [파일 역할] Resource는 SDK/툴/캐시/빌드 산출물 등 스캔으로 발견된 리소스
// 하나를 나타내는 domain 모델이다. ResourceID는 Type + Version + 정규화된
// 경로로부터 안정적인 sha256 해시 ID를 만든다. internal/app/resource_service.go의
// ResourceService.Observe가 어댑터가 보고한 원시 fact를 크기 측정·위험도
// 분류까지 마친 뒤 이 구조체에 채워 저장하고, dependency.go의
// Dependency.TargetID가 여기 Resource.ID를 가리켜 PROJECT -> RESOURCE 간선의
// 대상(target)이 된다.

// ResourceType classifies the kind of development resource libra detected.
type ResourceType string

const (
	ResourceTypeWindowsSDK   ResourceType = "windows-sdk"
	ResourceTypeNetFXSDK     ResourceType = "netfx-sdk" // .NET Framework SDK, distinct from the .NET (Core) SDK
	ResourceTypeVisualStudio ResourceType = "visual-studio"
	ResourceTypeMSBuild      ResourceType = "msbuild"
	ResourceTypeDotNetSDK    ResourceType = "dotnet-sdk"
	ResourceTypeNodeModules  ResourceType = "node-modules"
	ResourceTypeBuildOutput  ResourceType = "build-output" // bin, obj, build, dist, .next, out, Debug, Release
	ResourceTypeGlobalCache  ResourceType = "global-cache" // npm/pnpm global cache
	ResourceTypeDockerCache  ResourceType = "docker-cache"
)

// RiskLevel indicates how safe a resource is to clean up.
type RiskLevel string

const (
	RiskSafe    RiskLevel = "SAFE"
	RiskReview  RiskLevel = "REVIEW"
	RiskBlocked RiskLevel = "BLOCKED"
)

// Resource is an SDK, tool, cache, or build artifact discovered by scan.
type Resource struct {
	ID              string
	Name            string
	Type            ResourceType
	Version         string
	DisplayPath     string
	NormalizedPath  string
	LogicalSize     int64
	SizeKnown       bool
	ReclaimableSize int64
	Regenerable     bool
	SystemManaged   bool
	LastModifiedAt  *time.Time
	LastObservedAt  time.Time
	Risk            RiskLevel
	// Confidence is analysis-coverage confidence (0-100), not a real
	// probability. See EvidenceKind weighting in evidence.go.
	Confidence int
}

// ResourceID returns the stable identity shared by detectors and storage.
// NUL separators make the serialization unambiguous while preserving the
// agreed Type + Version + NormalizedPath key fields.
func ResourceID(resourceType ResourceType, version, normalizedPath string) string {
	key := strings.Join([]string{string(resourceType), version, normalizedPath}, "\x00")
	digest := sha256.Sum256([]byte(key))
	return hex.EncodeToString(digest[:])
}
