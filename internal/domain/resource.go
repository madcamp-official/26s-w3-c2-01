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
	ResourceTypeAndroidSDK   ResourceType = "android-sdk"
	ResourceTypeNodeModules  ResourceType = "node-modules"
	// ResourceTypeXcodeInstall is the installed Xcode.app itself (version +
	// path via xcode-select/xcodebuild), the macOS analogue of
	// ResourceTypeVisualStudio -- a system-managed dev tool, not a cache.
	ResourceTypeXcodeInstall ResourceType = "xcode-install"
	// ResourceTypePods is a project-owned CocoaPods `Pods/` directory
	// (installed pods for one project), distinct from the global CocoaPods
	// download cache (ResourceTypeGlobalCache, Version "cocoapods-cache") --
	// same OWNS-vs-global-cache split as ResourceTypeNodeModules vs npm's
	// global cache.
	ResourceTypePods ResourceType = "cocoapods-pods"
	// ResourceTypeBuildOutput covers bin, obj, build, dist, .next, out,
	// Debug, Release, and (docs/libra_integration_contracts.md §19.4)
	// Python's __pycache__, .pytest_cache, .mypy_cache, *.egg-info.
	ResourceTypeBuildOutput  ResourceType = "build-output"
	ResourceTypeGlobalCache  ResourceType = "global-cache" // npm/pnpm global cache
	ResourceTypeDockerCache  ResourceType = "docker-cache"
	ResourceTypeDockerVolume ResourceType = "docker-volume"
	// ResourceTypeVenv is a Python virtual environment (.venv/venv/env),
	// confirmed only after pyvenv.cfg is found inside it (§19.4 결정 3).
	ResourceTypeVenv ResourceType = "python-venv"
	// ResourceTypeCondaEnv is a conda environment. It carries a REQUIRES edge
	// when it is a globally registered named environment, or an OWNS edge
	// when it is a local prefix environment created under a project root
	// (§19.4/§19.5 결정 4·5) -- the relation, not the resource type, is what
	// distinguishes the two cases.
	ResourceTypeCondaEnv ResourceType = "conda-env"
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
	// ConfidenceProfile is the decision-specific breakdown. Confidence is
	// retained as the minimum-axis summary for CLI/schema compatibility.
	ConfidenceProfile ConfidenceProfile
	RiskReasons       []RiskReason
	// RegenerationCommand is the command a developer would run to recreate
	// this resource (e.g. "npm ci", "dotnet build App.csproj"), set by the
	// detecting adapter at the same time it determines Regenerable -- it
	// already has the lockfile/project-type facts on hand right then. Empty
	// when no specific command is known, even if Regenerable is true.
	RegenerationCommand string
}

// ResourceID returns the stable identity shared by detectors and storage.
// NUL separators make the serialization unambiguous while preserving the
// agreed Type + Version + NormalizedPath key fields.
func ResourceID(resourceType ResourceType, version, normalizedPath string) string {
	key := strings.Join([]string{string(resourceType), version, normalizedPath}, "\x00")
	digest := sha256.Sum256([]byte(key))
	return hex.EncodeToString(digest[:])
}
