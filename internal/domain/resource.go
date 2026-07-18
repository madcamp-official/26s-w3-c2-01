package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

// ResourceType classifies the kind of development resource libra detected.
type ResourceType string

const (
	ResourceTypeWindowsSDK   ResourceType = "windows-sdk"
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
