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
	// RegenerationCommand is the command a developer would run to recreate
	// this resource (e.g. "npm ci", "dotnet build App.csproj"), set by the
	// detecting adapter at the same time it determines Regenerable -- it
	// already has the lockfile/project-type facts on hand right then. Empty
	// when no specific command is known, even if Regenerable is true.
	RegenerationCommand string
	// Reason is why RiskPolicy.Classify assigned Risk its current value
	// (e.g. "project artifact is regenerable and all cleanup evidence is
	// verified"), taken from RiskAssessment.Reasons at classification time.
	// See app.DefaultRiskPolicy.Classify.
	Reason string
}

// ResourceID returns the stable identity shared by detectors and storage.
// NUL separators make the serialization unambiguous while preserving the
// agreed Type + Version + NormalizedPath key fields.
func ResourceID(resourceType ResourceType, version, normalizedPath string) string {
	key := strings.Join([]string{string(resourceType), version, normalizedPath}, "\x00")
	digest := sha256.Sum256([]byte(key))
	return hex.EncodeToString(digest[:])
}
