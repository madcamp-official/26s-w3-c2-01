package app

import (
	"context"
	"errors"
	"os"

	gitadapter "github.com/madcamp-official/26s-w3-c2-01/internal/adapter/git"
	"github.com/madcamp-official/26s-w3-c2-01/internal/adapter/msbuild"
	nodeadapter "github.com/madcamp-official/26s-w3-c2-01/internal/adapter/node"
	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/scanner"
)

// project_detector_adapters.go wraps each internal/adapter/* package's own
// Detector/Parser type in the ProjectDetector interface
// AnalysisOrchestrator expects (analysis_contract.go), one adapter-specific
// struct per project kind. resource_detector_adapters.go is this file's
// counterpart for system resources instead of projects.
type GitProjectDetector struct{ Detector gitadapter.Detector }

func (d GitProjectDetector) Observe(ctx context.Context, entry scanner.Entry) DetectionResult[ProjectCandidate] {
	if d.Detector == nil || !d.Detector.CanDetect(entry) {
		return DetectionResult[ProjectCandidate]{}
	}
	projects, err := d.Detector.Detect(ctx, entry)
	if err != nil {
		return projectDetectionFailure("git", entry.Path, "detect Git project", err)
	}
	return DetectionResult[ProjectCandidate]{Items: []ProjectCandidate{{Projects: projects}}}
}

type NodeProjectDetector struct{ Detector nodeadapter.Detector }

func (d NodeProjectDetector) Observe(ctx context.Context, entry scanner.Entry) DetectionResult[ProjectCandidate] {
	if d.Detector == nil || !d.Detector.CanDetect(entry) {
		return DetectionResult[ProjectCandidate]{}
	}
	project, err := d.Detector.Detect(ctx, entry)
	if err != nil {
		return projectDetectionFailure("node", entry.Path, "parse package.json", err)
	}

	candidate := ProjectCandidate{Projects: []domain.BuildProject{project}}
	var issues []Issue
	// DetectArtifacts, not DetectWorkspaceArtifacts: Observe runs once per
	// project-root entry the scanner walks into, including every workspace
	// member independently, so scoping to entry.Path's own directory avoids
	// reporting a member's artifacts twice (once here, once from its own
	// Observe call).
	artifacts, err := nodeadapter.DetectArtifacts(entry.Path)
	if err != nil {
		return DetectionResult[ProjectCandidate]{
			Items: []ProjectCandidate{candidate},
			Issues: []Issue{{Code: IssueAdapterFailed, Phase: PhaseDiscoverProjects, Adapter: "node",
				Path: entry.Path, Operation: "detect node artifacts", Severity: IssueWarning, Message: err.Error(), Cause: err}},
		}
	}
	for _, resource := range artifacts {
		cleanup, evidenceIssues := projectArtifactCleanupEvidence(ctx, entry.Path, resource.DisplayPath)
		issues = append(issues, evidenceIssues...)
		candidate.ProjectResources = append(candidate.ProjectResources, ProjectResourceCandidate{
			OwnerManifestPath: project.ManifestPath,
			Resource:          resource,
			Cleanup:           cleanup,
		})
	}
	return DetectionResult[ProjectCandidate]{Items: []ProjectCandidate{candidate}, Issues: issues}
}

type MSBuildProjectDetector struct{ Parser msbuild.BuildProjectParser }

func (d MSBuildProjectDetector) Observe(ctx context.Context, entry scanner.Entry) DetectionResult[ProjectCandidate] {
	if d.Parser == nil || !d.Parser.CanParse(entry) {
		return DetectionResult[ProjectCandidate]{}
	}
	parsed, err := d.Parser.Parse(ctx, entry)
	if err != nil {
		return projectDetectionFailure("msbuild", entry.Path, "parse MSBuild project", err)
	}
	var candidate ProjectCandidate
	var issues []Issue
	for _, item := range parsed {
		candidate.Projects = append(candidate.Projects, item.Project)
		for _, declared := range item.Declared {
			sourcePath := declared.SourcePath
			if sourcePath == "" {
				sourcePath = item.Project.ManifestPath
			}
			candidate.ProjectProperties = append(candidate.ProjectProperties, ProjectProperty{
				OwnerManifestPath: item.Project.ManifestPath,
				SourcePath:        sourcePath,
				Name:              declared.Name,
				Value:             declared.Value,
				Condition:         declared.Condition,
			})
		}

		artifacts, err := msbuild.DetectArtifacts(item.Project.RootPath)
		if err != nil {
			issues = append(issues, Issue{Code: IssueAdapterFailed, Phase: PhaseDiscoverProjects, Adapter: "msbuild",
				Path: item.Project.RootPath, Operation: "detect msbuild artifacts", Severity: IssueWarning, Message: err.Error(), Cause: err})
			continue
		}
		for _, resource := range artifacts {
			cleanup, evidenceIssues := projectArtifactCleanupEvidence(ctx, item.Project.RootPath, resource.DisplayPath)
			issues = append(issues, evidenceIssues...)
			candidate.ProjectResources = append(candidate.ProjectResources, ProjectResourceCandidate{
				OwnerManifestPath: item.Project.ManifestPath,
				Resource:          resource,
				Cleanup:           cleanup,
			})
		}
	}
	return DetectionResult[ProjectCandidate]{Items: []ProjectCandidate{candidate}, Issues: issues}
}

type MSBuildWorkspaceDetector struct{ Parser msbuild.WorkspaceParser }

func (d MSBuildWorkspaceDetector) Observe(ctx context.Context, entry scanner.Entry) DetectionResult[ProjectCandidate] {
	if d.Parser == nil || !d.Parser.CanParse(entry) {
		return DetectionResult[ProjectCandidate]{}
	}
	parsed, err := d.Parser.Parse(ctx, entry)
	if err != nil {
		return projectDetectionFailure("msbuild", entry.Path, "parse MSBuild workspace", err)
	}
	return DetectionResult[ProjectCandidate]{Items: []ProjectCandidate{{
		Workspace: &parsed.Workspace, WorkspaceProjectPaths: parsed.ProjectPaths,
	}}}
}

// projectArtifactCleanupEvidence checks the two safety facts DetectArtifacts
// itself doesn't determine -- whether resourcePath is an NTFS reparse point
// (junction/mount point/symlink) and whether Git tracks any file at or under
// it -- and folds them in alongside the ProjectOwned/KnownOutputPath facts
// every project-owned build artifact already carries just by being found
// under projectRoot with a recognized artifact name. A check that fails
// (can't stat the path, no git binary, etc.) is reported as an Issue and
// leaves the corresponding evidence field unverified (false), never
// guessed true -- CleanupEvidence.complete() requires an affirmative answer
// on all four facts before RiskPolicy will consider SAFE.
func projectArtifactCleanupEvidence(ctx context.Context, projectRoot, resourcePath string) (CleanupEvidence, []Issue) {
	evidence := CleanupEvidence{ProjectOwned: true, KnownOutputPath: true}
	var issues []Issue

	if info, err := os.Lstat(resourcePath); err != nil {
		issues = append(issues, cleanupEvidenceIssue(resourcePath, "check reparse point", err))
	} else if !scanner.IsLinkLike(info) {
		evidence.ReparsePointFree = true
	}

	repoRoot, found, err := gitadapter.FindRepoRoot(projectRoot)
	switch {
	case err != nil:
		issues = append(issues, cleanupEvidenceIssue(resourcePath, "locate git repository", err))
	case !found:
		// No repository at all above this project: vacuously no tracked
		// files can exist under resourcePath.
		evidence.GitTrackedOriginalsAbsent = true
	default:
		tracked, err := (gitadapter.TrackedFilesChecker{}).HasTrackedFiles(ctx, repoRoot, resourcePath)
		if err != nil {
			issues = append(issues, cleanupEvidenceIssue(resourcePath, "check git tracked files", err))
		} else if !tracked {
			evidence.GitTrackedOriginalsAbsent = true
		}
	}

	return evidence, issues
}

func cleanupEvidenceIssue(path, operation string, err error) Issue {
	return Issue{Code: IssueAdapterFailed, Phase: PhaseDiscoverProjects, Adapter: "cleanup-evidence",
		Path: path, Operation: operation, Severity: IssueWarning, Message: err.Error(), Cause: err}
}

func projectDetectionFailure(adapterName, path, operation string, err error) DetectionResult[ProjectCandidate] {
	code := IssueMalformedManifest
	if errors.Is(err, context.Canceled) {
		code = IssueCancelled
	}
	return DetectionResult[ProjectCandidate]{
		Issues: []Issue{{Code: code, Phase: PhaseDiscoverProjects, Adapter: adapterName,
			Path: path, Operation: operation, Severity: IssueWarning, Message: err.Error(), Cause: err}},
		Unverified: []UnverifiedScope{{Path: path, Phase: PhaseDiscoverProjects, Reason: err.Error()}},
	}
}
