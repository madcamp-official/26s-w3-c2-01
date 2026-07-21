package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	cocoapodsadapter "github.com/madcamp-official/26s-w3-c2-01/internal/adapter/cocoapods"
	condaadapter "github.com/madcamp-official/26s-w3-c2-01/internal/adapter/conda"
	gitadapter "github.com/madcamp-official/26s-w3-c2-01/internal/adapter/git"
	"github.com/madcamp-official/26s-w3-c2-01/internal/adapter/msbuild"
	nodeadapter "github.com/madcamp-official/26s-w3-c2-01/internal/adapter/node"
	projectmarkeradapter "github.com/madcamp-official/26s-w3-c2-01/internal/adapter/projectmarker"
	pythonadapter "github.com/madcamp-official/26s-w3-c2-01/internal/adapter/python"
	swiftpmadapter "github.com/madcamp-official/26s-w3-c2-01/internal/adapter/swiftpm"
	xcodeprojadapter "github.com/madcamp-official/26s-w3-c2-01/internal/adapter/xcodeproj"
	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/scanner"
)

type EcosystemProjectDetector struct{ Detector projectmarkeradapter.Detector }

func (d EcosystemProjectDetector) Observe(ctx context.Context, entry scanner.Entry) DetectionResult[ProjectCandidate] {
	project, matched, err := d.Detector.Detect(ctx, entry)
	if err != nil {
		return projectDetectionFailure("ecosystem", entry.Path, "parse ecosystem project", err)
	}
	if !matched {
		return DetectionResult[ProjectCandidate]{}
	}
	return DetectionResult[ProjectCandidate]{Items: []ProjectCandidate{{Projects: []domain.BuildProject{project}}}}
}

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
	workspace, workspaceIssues, workspaceUnverified := detectNodeWorkspace(entry.Path, project)
	if workspace != nil {
		candidate.Workspace = &workspace.workspace
		candidate.WorkspaceProjectPaths = workspace.memberManifestPaths
	}
	issues = append(issues, workspaceIssues...)
	// Detect the root's own artifacts here. Declared workspace members are
	// added below even when discovery pruning skipped their directory; the
	// orchestrator deduplicates any member also seen by the regular walk.
	artifacts, err := nodeadapter.DetectArtifacts(entry.Path)
	if err != nil {
		issues = append(issues, Issue{Code: IssueAdapterFailed, Phase: PhaseDiscoverProjects, Adapter: "node",
			Path: entry.Path, Operation: "detect node artifacts", Severity: IssueWarning, Message: err.Error(), Cause: err})
		return DetectionResult[ProjectCandidate]{
			Items:  []ProjectCandidate{candidate},
			Issues: issues, Unverified: workspaceUnverified,
		}
	}
	for _, resource := range artifacts {
		cleanup, evidenceIssues := projectArtifactCleanupEvidence(ctx, entry.Path, resource)
		issues = append(issues, evidenceIssues...)
		candidate.ProjectResources = append(candidate.ProjectResources, ProjectResourceCandidate{
			OwnerManifestPath: project.ManifestPath,
			Resource:          resource,
			Cleanup:           cleanup,
		})
	}
	if workspace != nil {
		for _, memberRoot := range workspace.memberRoots {
			info, err := os.Stat(memberRoot)
			if err != nil {
				issues = append(issues, cleanupEvidenceIssue(memberRoot, "stat Node workspace member", err))
				continue
			}
			memberEntry := scanner.Entry{Path: memberRoot, Kind: scanner.EntryDirectory, Mode: info.Mode(), ModifiedAt: info.ModTime()}
			if !d.Detector.CanDetect(memberEntry) {
				continue
			}
			memberProject, err := d.Detector.Detect(ctx, memberEntry)
			if err != nil {
				issues = append(issues, projectDetectionFailure("node", memberRoot, "parse workspace member package.json", err).Issues...)
				workspaceUnverified = append(workspaceUnverified, UnverifiedScope{Path: memberRoot, Phase: PhaseDiscoverProjects, Reason: err.Error()})
				continue
			}
			candidate.Projects = append(candidate.Projects, memberProject)
			memberArtifacts, err := nodeadapter.DetectMemberArtifacts(memberRoot, entry.Path)
			if err != nil {
				issues = append(issues, cleanupEvidenceIssue(memberRoot, "detect workspace member artifacts", err))
				continue
			}
			for _, resource := range memberArtifacts {
				cleanup, evidenceIssues := projectArtifactCleanupEvidence(ctx, memberRoot, resource)
				issues = append(issues, evidenceIssues...)
				candidate.ProjectResources = append(candidate.ProjectResources, ProjectResourceCandidate{
					OwnerManifestPath: memberProject.ManifestPath, Resource: resource, Cleanup: cleanup,
				})
			}
		}
	}
	return DetectionResult[ProjectCandidate]{Items: []ProjectCandidate{candidate}, Issues: issues, Unverified: workspaceUnverified}
}

type nodeWorkspaceCandidate struct {
	workspace           domain.Workspace
	memberManifestPaths []string
	memberRoots         []string
}

func detectNodeWorkspace(root string, project domain.BuildProject) (*nodeWorkspaceCandidate, []Issue, []UnverifiedScope) {
	info, ok, err := nodeadapter.DetectWorkspace(root)
	if err != nil {
		issue := Issue{Code: IssueMalformedManifest, Phase: PhaseDiscoverProjects, Adapter: "node",
			Path: root, Operation: "detect Node workspace", Severity: IssueWarning, Message: err.Error(), Cause: err}
		return nil, []Issue{issue}, []UnverifiedScope{{Path: root, Phase: PhaseDiscoverProjects, Reason: err.Error()}}
	}
	if !ok {
		return nil, nil, nil
	}

	members, err := nodeadapter.ResolveMembers(root, info)
	if err != nil {
		issue := Issue{Code: IssueAdapterFailed, Phase: PhaseDiscoverProjects, Adapter: "node",
			Path: root, Operation: "resolve Node workspace members", Severity: IssueWarning, Message: err.Error(), Cause: err}
		return nil, []Issue{issue}, []UnverifiedScope{{Path: root, Phase: PhaseDiscoverProjects, Reason: err.Error()}}
	}
	manifestPath := filepath.Join(root, "package.json")
	if info.Kind == nodeadapter.WorkspaceKindPnpm {
		manifestPath = filepath.Join(root, "pnpm-workspace.yaml")
	}
	memberManifests := make([]string, 0, len(members))
	for _, member := range members {
		memberManifests = append(memberManifests, filepath.Join(member, "package.json"))
	}
	return &nodeWorkspaceCandidate{
		workspace: domain.Workspace{
			Name: project.Name, Type: domain.WorkspaceTypeNode, ManifestPath: manifestPath,
		},
		memberManifestPaths: memberManifests,
		memberRoots:         members,
	}, nil, nil
}

// PythonProjectDetector wraps internal/adapter/python (project markers,
// venv, cache) and internal/adapter/conda (local prefix environments,
// declared environment.yml name) the same way NodeProjectDetector wraps
// node.go and workspace.go -- see docs/libra_integration_contracts.md §19.4
// and docs/libra_python_conda_scope_decisions.md.
type PythonProjectDetector struct{ Detector pythonadapter.Detector }

func (d PythonProjectDetector) Observe(ctx context.Context, entry scanner.Entry) DetectionResult[ProjectCandidate] {
	if d.Detector == nil || !d.Detector.CanDetect(entry) {
		return DetectionResult[ProjectCandidate]{}
	}
	project, err := d.Detector.Detect(ctx, entry)
	if err != nil {
		return projectDetectionFailure("python", entry.Path, "detect Python project", err)
	}

	candidate := ProjectCandidate{Projects: []domain.BuildProject{project}}
	var issues []Issue

	artifacts, err := pythonadapter.DetectArtifacts(entry.Path)
	if err != nil {
		issues = append(issues, Issue{Code: IssueAdapterFailed, Phase: PhaseDiscoverProjects, Adapter: "python",
			Path: entry.Path, Operation: "detect python artifacts", Severity: IssueWarning, Message: err.Error(), Cause: err})
		return DetectionResult[ProjectCandidate]{Items: []ProjectCandidate{candidate}, Issues: issues}
	}
	for _, resource := range artifacts {
		cleanup, evidenceIssues := projectArtifactCleanupEvidence(ctx, entry.Path, resource)
		issues = append(issues, evidenceIssues...)
		candidate.ProjectResources = append(candidate.ProjectResources, ProjectResourceCandidate{
			OwnerManifestPath: project.ManifestPath,
			Resource:          resource,
			Cleanup:           cleanup,
		})
	}

	declaredEnvName, declaredEnvSource, hasEnvDecl, envErr := condaadapter.DeclaredEnvironmentName(entry.Path)
	if envErr != nil {
		issues = append(issues, Issue{Code: IssueMalformedManifest, Phase: PhaseDiscoverProjects, Adapter: "conda",
			Path: entry.Path, Operation: "parse environment.yml", Severity: IssueWarning, Message: envErr.Error(), Cause: envErr})
	} else if hasEnvDecl {
		candidate.ProjectProperties = append(candidate.ProjectProperties, ProjectProperty{
			OwnerManifestPath: project.ManifestPath,
			SourcePath:        declaredEnvSource,
			Name:              condaEnvPropertyName,
			Value:             declaredEnvName,
		})
	}

	// Local conda prefix environments (결정 5 예외): OWNS, but -- unlike the
	// venv/cache artifacts above -- deliberately left with zero-value
	// CleanupEvidence. A conda environment is never a cleanup candidate even
	// when project-owned (결정 4), so projectArtifactCleanupEvidence is never
	// called for it; CleanupEvidence.complete() stays false and RiskPolicy
	// can never classify it SAFE.
	localEnvs, err := condaadapter.DetectLocalPrefixEnvs(entry.Path, hasEnvDecl)
	if err != nil {
		issues = append(issues, Issue{Code: IssueAdapterFailed, Phase: PhaseDiscoverProjects, Adapter: "conda",
			Path: entry.Path, Operation: "detect local conda prefix environments", Severity: IssueWarning, Message: err.Error(), Cause: err})
	}
	for _, resource := range localEnvs {
		candidate.ProjectResources = append(candidate.ProjectResources, ProjectResourceCandidate{
			OwnerManifestPath: project.ManifestPath,
			Resource:          resource,
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

		artifacts, err := msbuild.DetectArtifacts(item.Project.RootPath, item.Declared)
		if err != nil {
			issues = append(issues, Issue{Code: IssueAdapterFailed, Phase: PhaseDiscoverProjects, Adapter: "msbuild",
				Path: item.Project.RootPath, Operation: "detect msbuild artifacts", Severity: IssueWarning, Message: err.Error(), Cause: err})
			continue
		}
		for _, resource := range artifacts {
			resource.RegenerationCommand = msbuildRegenerationCommand(item.Project)
			cleanup, evidenceIssues := projectArtifactCleanupEvidence(ctx, item.Project.RootPath, resource)
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

// XcodeProjectDetector wraps internal/adapter/xcodeproj.Detector (the
// .xcodeproj manifest) and internal/adapter/cocoapods.DetectArtifacts (the
// project-owned Pods/ directory a sibling Podfile implies) the same way
// NodeProjectDetector wraps node.go and its artifact detection.
type XcodeProjectDetector struct{ Detector xcodeprojadapter.Detector }

func (d XcodeProjectDetector) Observe(ctx context.Context, entry scanner.Entry) DetectionResult[ProjectCandidate] {
	if !d.Detector.CanDetect(entry) {
		return DetectionResult[ProjectCandidate]{}
	}
	project, err := d.Detector.Detect(ctx, entry)
	if err != nil {
		return projectDetectionFailure("xcodeproj", entry.Path, "detect Xcode project", err)
	}

	candidate := ProjectCandidate{Projects: []domain.BuildProject{project}}
	var issues []Issue

	artifacts, err := cocoapodsadapter.DetectArtifacts(project.RootPath)
	if err != nil {
		issues = append(issues, Issue{Code: IssueAdapterFailed, Phase: PhaseDiscoverProjects, Adapter: "cocoapods",
			Path: project.RootPath, Operation: "detect CocoaPods artifacts", Severity: IssueWarning, Message: err.Error(), Cause: err})
		return DetectionResult[ProjectCandidate]{Items: []ProjectCandidate{candidate}, Issues: issues}
	}
	for _, resource := range artifacts {
		cleanup, evidenceIssues := projectArtifactCleanupEvidence(ctx, project.RootPath, resource)
		issues = append(issues, evidenceIssues...)
		candidate.ProjectResources = append(candidate.ProjectResources, ProjectResourceCandidate{
			OwnerManifestPath: project.ManifestPath,
			Resource:          resource,
			Cleanup:           cleanup,
		})
	}
	return DetectionResult[ProjectCandidate]{Items: []ProjectCandidate{candidate}, Issues: issues}
}

// XcodeWorkspaceDetector wraps internal/adapter/xcodeproj.WorkspaceDetector
// (.xcworkspace + its resolved member .xcodeproj references) the same way
// MSBuildWorkspaceDetector wraps .sln parsing.
type XcodeWorkspaceDetector struct {
	Detector xcodeprojadapter.WorkspaceDetector
}

func (d XcodeWorkspaceDetector) Observe(ctx context.Context, entry scanner.Entry) DetectionResult[ProjectCandidate] {
	if !d.Detector.CanDetect(entry) {
		return DetectionResult[ProjectCandidate]{}
	}
	result, err := d.Detector.Detect(ctx, entry)
	if err != nil {
		return projectDetectionFailure("xcodeproj", entry.Path, "parse Xcode workspace", err)
	}
	return DetectionResult[ProjectCandidate]{Items: []ProjectCandidate{{
		Workspace: &result.Workspace, WorkspaceProjectPaths: result.ProjectPaths,
	}}}
}

// SwiftPMProjectDetector wraps internal/adapter/swiftpm's Package.swift
// project detection and its project-owned .build/ artifact, plus the
// declared swift-tools-version comment (carried through as a
// ProjectProperty for XcodeDependencyAnalyzer, the same way MSBuild's
// declared properties reach MSBuildDependencyAnalyzer).
type SwiftPMProjectDetector struct{ Detector swiftpmadapter.Detector }

func (d SwiftPMProjectDetector) Observe(ctx context.Context, entry scanner.Entry) DetectionResult[ProjectCandidate] {
	if !d.Detector.CanDetect(entry) {
		return DetectionResult[ProjectCandidate]{}
	}
	project, err := d.Detector.Detect(ctx, entry)
	if err != nil {
		return projectDetectionFailure("swiftpm", entry.Path, "detect SwiftPM project", err)
	}

	candidate := ProjectCandidate{Projects: []domain.BuildProject{project}}
	if toolsVersion := swiftpmadapter.ToolsVersion(entry.Path); toolsVersion != "" {
		candidate.ProjectProperties = append(candidate.ProjectProperties, ProjectProperty{
			OwnerManifestPath: project.ManifestPath,
			SourcePath:        entry.Path,
			Name:              swiftpmadapter.ToolsVersionPropertyName,
			Value:             toolsVersion,
		})
	}

	var issues []Issue
	artifacts, err := swiftpmadapter.DetectArtifacts(project.RootPath)
	if err != nil {
		issues = append(issues, Issue{Code: IssueAdapterFailed, Phase: PhaseDiscoverProjects, Adapter: "swiftpm",
			Path: project.RootPath, Operation: "detect SwiftPM artifacts", Severity: IssueWarning, Message: err.Error(), Cause: err})
		return DetectionResult[ProjectCandidate]{Items: []ProjectCandidate{candidate}, Issues: issues}
	}
	for _, resource := range artifacts {
		cleanup, evidenceIssues := projectArtifactCleanupEvidence(ctx, project.RootPath, resource)
		issues = append(issues, evidenceIssues...)
		candidate.ProjectResources = append(candidate.ProjectResources, ProjectResourceCandidate{
			OwnerManifestPath: project.ManifestPath,
			Resource:          resource,
			Cleanup:           cleanup,
		})
	}
	return DetectionResult[ProjectCandidate]{Items: []ProjectCandidate{candidate}, Issues: issues}
}

// msbuildRegenerationCommand returns the command that rebuilds project,
// recreating its bin/obj output -- dotnet build for .NET SDK-style projects,
// msbuild for VC++, matching the two ProjectType values XMLBuildProjectParser
// ever produces. "" for any other type (there is currently no other MSBuild
// project type, but a switch is more honest about that than a panic).
func msbuildRegenerationCommand(project domain.BuildProject) string {
	switch project.Type {
	case domain.ProjectTypeMSBuildDotNet:
		return fmt.Sprintf(`dotnet build "%s"`, project.ManifestPath)
	case domain.ProjectTypeMSBuildCpp:
		return fmt.Sprintf(`msbuild "%s"`, project.ManifestPath)
	default:
		return ""
	}
}

// projectArtifactCleanupEvidence checks the three safety facts DetectArtifacts
// itself doesn't already prove -- whether the output location is genuinely
// known (not just name-guessed), whether the resource is an NTFS reparse
// point (junction/mount point/symlink), and whether Git tracks any file at
// or under it -- alongside the ProjectOwned fact every project-owned build
// artifact already carries just by being found under projectRoot.
//
// KnownOutputPath is true unconditionally for node_modules: npm/Yarn/pnpm
// never let a project customize where it lives, so its location is known
// regardless of how confident the *regenerability* evidence (the lockfile
// check) is. Every other resource type only counts as KnownOutputPath when
// its Confidence is at least DECLARED-strength -- i.e. DetectArtifacts found
// it via a real declared OutputPath/OutDir-family property, not just a bin/
// obj/dist name that happened to match (see msbuild/artifacts.go).
//
// A check that fails (can't stat the path, no git binary, etc.) is reported
// as an Issue and leaves the corresponding evidence field unverified
// (false), never guessed true -- CleanupEvidence.complete() requires an
// affirmative answer on all four facts before RiskPolicy will consider SAFE.
func projectArtifactCleanupEvidence(ctx context.Context, projectRoot string, resource domain.Resource) (CleanupEvidence, []Issue) {
	evidence := CleanupEvidence{
		ProjectOwned:    true,
		KnownOutputPath: resource.Type == domain.ResourceTypeNodeModules || resource.Confidence >= domain.DefaultConfidence[domain.EvidenceDeclared],
	}
	var issues []Issue

	if info, err := os.Lstat(resource.DisplayPath); err != nil {
		issues = append(issues, cleanupEvidenceIssue(resource.DisplayPath, "check reparse point", err))
	} else if !scanner.IsLinkLike(info) {
		evidence.ReparsePointFree = true
	}

	repoRoot, found, err := gitadapter.FindRepoRoot(projectRoot)
	switch {
	case err != nil:
		issues = append(issues, cleanupEvidenceIssue(resource.DisplayPath, "locate git repository", err))
	case !found:
		// No repository at all above this project: vacuously no tracked
		// files can exist under resource.DisplayPath.
		evidence.GitTrackedOriginalsAbsent = true
	default:
		tracked, err := (gitadapter.TrackedFilesChecker{}).HasTrackedFiles(ctx, repoRoot, resource.DisplayPath)
		if err != nil {
			issues = append(issues, cleanupEvidenceIssue(resource.DisplayPath, "check git tracked files", err))
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
