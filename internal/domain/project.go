package domain

import "time"

// ProjectType classifies the kind of project root libra detected.
type ProjectType string

const (
	ProjectTypeVSSolution    ProjectType = "vs-solution"    // .sln
	ProjectTypeMSBuildCpp    ProjectType = "msbuild-cpp"    // .vcxproj
	ProjectTypeMSBuildDotNet ProjectType = "msbuild-dotnet" // .csproj
	ProjectTypeNode          ProjectType = "node"           // package.json
	ProjectTypeGit           ProjectType = "git"            // .git
)

// ProjectStatus describes the activity state of a project.
type ProjectStatus string

const (
	ProjectStatusActive   ProjectStatus = "ACTIVE"
	ProjectStatusStale    ProjectStatus = "STALE"
	ProjectStatusArchived ProjectStatus = "ARCHIVED"
	ProjectStatusUnknown  ProjectStatus = "UNKNOWN"
)

// Project is a project root discovered by scan (Visual Studio solution,
// MSBuild project, Node project, or Git repository).
type Project struct {
	ID             string
	Name           string
	Path           string
	Type           ProjectType
	Drive          string
	LogicalSize    int64
	LastModifiedAt time.Time
	LastObservedAt time.Time
	Status         ProjectStatus
}
