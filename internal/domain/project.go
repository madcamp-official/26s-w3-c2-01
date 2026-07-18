package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

// WorkspaceType classifies the kind of workspace/grouping file libra detected.
type WorkspaceType string

const (
	WorkspaceTypeVSSolution WorkspaceType = "vs-solution" // .sln
)

// Workspace is a grouping file that references one or more BuildProjects
// (currently only a Visual Studio .sln). It has no build-tool dependencies
// of its own -- those live on the BuildProjects it references, via
// WorkspaceProject.
type Workspace struct {
	ID                     string
	Name                   string
	Type                   WorkspaceType
	ManifestPath           string
	NormalizedManifestPath string
	LastObservedAt         time.Time
}

// ProjectType classifies the kind of build project libra detected.
type ProjectType string

const (
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

// BuildProject is a directly buildable/analyzable project root discovered by
// scan (MSBuild C++/.NET project, Node project, or Git repository). SDK and
// tool dependencies attach here, not to any Workspace that groups it, since
// the same BuildProject can belong to more than one Workspace.
type BuildProject struct {
	ID                     string
	Name                   string
	Type                   ProjectType
	RootPath               string
	NormalizedRootPath     string
	ManifestPath           string
	NormalizedManifestPath string
	Drive                  string
	LogicalSize            int64
	LastModifiedAt         time.Time
	LastObservedAt         time.Time
	Status                 ProjectStatus
}

func ProjectID(projectType ProjectType, normalizedManifestPath string) string {
	return stablePathID(string(projectType), normalizedManifestPath)
}

func WorkspaceID(workspaceType WorkspaceType, normalizedManifestPath string) string {
	return stablePathID(string(workspaceType), normalizedManifestPath)
}

func stablePathID(kind, normalizedManifestPath string) string {
	key := strings.Join([]string{kind, normalizedManifestPath}, "\x00")
	digest := sha256.Sum256([]byte(key))
	return hex.EncodeToString(digest[:])
}

// WorkspaceProject is a many-to-many membership edge: a single BuildProject
// (e.g. a shared library referenced from more than one solution) may belong
// to more than one Workspace.
// workspaceproject는 sln과 vcxproj의 연결을 나타내는 테이블
type WorkspaceProject struct {
	WorkspaceID    string
	BuildProjectID string
}
