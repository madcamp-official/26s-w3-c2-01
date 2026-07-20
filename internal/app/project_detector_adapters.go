package app

import (
	"context"
	"errors"

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
		candidate.ProjectResources = append(candidate.ProjectResources, ProjectResourceCandidate{
			OwnerManifestPath: project.ManifestPath,
			Resource:          resource,
			// ReparsePointFree and GitTrackedOriginalsAbsent stay unverified
			// (zero value): no adapter yet checks NTFS reparse points or
			// Git-tracked files under this directory, and CleanupEvidence's
			// zero value means unverified, not false (see
			// MSBuildProjectDetector.Observe's identical Cleanup literal).
			Cleanup: CleanupEvidence{
				ProjectOwned:    true,
				KnownOutputPath: true,
			},
		})
	}
	return DetectionResult[ProjectCandidate]{Items: []ProjectCandidate{candidate}}
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

		artifacts, err := msbuild.DetectArtifacts(item.Project.RootPath)
		if err != nil {
			issues = append(issues, Issue{Code: IssueAdapterFailed, Phase: PhaseDiscoverProjects, Adapter: "msbuild",
				Path: item.Project.RootPath, Operation: "detect msbuild artifacts", Severity: IssueWarning, Message: err.Error(), Cause: err})
			continue
		}
		for _, resource := range artifacts {
			candidate.ProjectResources = append(candidate.ProjectResources, ProjectResourceCandidate{
				OwnerManifestPath: item.Project.ManifestPath,
				Resource:          resource,
				// ReparsePointFree and GitTrackedOriginalsAbsent stay
				// unverified (zero value): no adapter yet checks NTFS reparse
				// points or Git-tracked files under this directory, and
				// CleanupEvidence's zero value means unverified, not false.
				Cleanup: CleanupEvidence{
					ProjectOwned:    true,
					KnownOutputPath: true,
				},
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
