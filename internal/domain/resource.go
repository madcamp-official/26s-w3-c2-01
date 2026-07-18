package domain

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
	ID          string
	Name        string
	Type        ResourceType
	Version     string
	Path        string
	LogicalSize int64
	Regenerable bool
	Risk        RiskLevel
	// Confidence is analysis-coverage confidence (0-100), not a real
	// probability. See EvidenceKind weighting in evidence.go.
	Confidence int
}
