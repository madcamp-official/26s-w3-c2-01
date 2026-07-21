package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	condaadapter "github.com/madcamp-official/26s-w3-c2-01/internal/adapter/conda"
	"github.com/madcamp-official/26s-w3-c2-01/internal/adapter/msbuild"
	swiftpmadapter "github.com/madcamp-official/26s-w3-c2-01/internal/adapter/swiftpm"
	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

// condaEnvPropertyName is the ProjectProperty name PythonProjectDetector
// uses to carry a project's declared environment.yml "name" field through to
// CondaDependencyAnalyzer (see project_detector_adapters.go).
const condaEnvPropertyName = "conda-env"

// MSBuildDependencyAnalyzer adapts msbuild.ResolveDependencies to the
// DependencyAnalyzer contract: it converts the app-neutral ProjectProperty
// and ResourceIndex types this package works with into the
// msbuild.DeclaredProperty/[]domain.Resource shapes ResolveDependencies
// expects, then wraps the result back into a DetectionResult. It contains no
// SDK-matching logic of its own -- that lives entirely in
// internal/adapter/msbuild.
type MSBuildDependencyAnalyzer struct {
	// Now returns the collection timestamp recorded on resolved Evidence.
	// Defaults to time.Now; tests may override it for determinism.
	Now func() time.Time
}

func (a MSBuildDependencyAnalyzer) Analyze(ctx context.Context, input ProjectAnalysisInput, index ResourceIndex) DetectionResult[DependencyBundle] {
	declared := make([]msbuild.DeclaredProperty, len(input.Properties))
	for i, property := range input.Properties {
		declared[i] = msbuild.DeclaredProperty{
			Name:      property.Name,
			Value:     property.Value,
			Condition: property.Condition,
		}
	}

	installed := append(
		index.ListByType(domain.ResourceTypeWindowsSDK),
		index.ListByType(domain.ResourceTypeDotNetSDK)...,
	)

	now := a.Now
	if now == nil {
		now = time.Now
	}

	resolved, unverified := msbuild.ResolveDependencies(
		input.Project.ID, input.Project.ManifestPath, declared, installed, now(),
	)

	items := make([]DependencyBundle, len(resolved))
	for i, r := range resolved {
		items[i] = DependencyBundle{Dependency: r.Dependency, Evidence: r.Evidence}
	}
	scopes := make([]UnverifiedScope, len(unverified))
	for i, u := range unverified {
		scopes[i] = UnverifiedScope{Path: u.Source, Phase: PhaseResolveDependencies, Reason: u.Reason}
	}
	return DetectionResult[DependencyBundle]{Items: items, Unverified: scopes}
}

// CondaDependencyAnalyzer matches a Python project's declared conda
// environment name (environment.yml "name", carried as a ProjectProperty
// named condaEnvPropertyName) against the globally registered environments
// CondaResourceDetector observed, building a PROJECT REQUIRES RESOURCE edge
// on a match (docs/libra_integration_contracts.md §19.4/§19.5 결정 5).
//
// A generic environment name (base/env/py39/...) still produces an edge --
// refusing to link at all would silently drop a real relationship -- but at
// EvidenceInferred strength instead of EvidenceDeclared, plus an
// UnverifiedScope explaining why, so downstream commands don't overstate
// confidence in an ambiguous match.
type CondaDependencyAnalyzer struct {
	// Now returns the collection timestamp recorded on resolved Evidence.
	// Defaults to time.Now; tests may override it for determinism.
	Now func() time.Time
}

func (a CondaDependencyAnalyzer) Analyze(ctx context.Context, input ProjectAnalysisInput, index ResourceIndex) DetectionResult[DependencyBundle] {
	var declared *ProjectProperty
	for i := range input.Properties {
		if input.Properties[i].Name == condaEnvPropertyName {
			declared = &input.Properties[i]
			break
		}
	}
	if declared == nil {
		return DetectionResult[DependencyBundle]{}
	}

	var matched *domain.Resource
	for _, candidate := range index.ListByType(domain.ResourceTypeCondaEnv) {
		if strings.EqualFold(candidate.Name, declared.Value) {
			c := candidate
			matched = &c
			break
		}
	}
	if matched == nil {
		return DetectionResult[DependencyBundle]{
			Unverified: []UnverifiedScope{{Path: declared.SourcePath, Phase: PhaseResolveDependencies,
				Reason: fmt.Sprintf("declared conda environment %q is not currently registered with conda", declared.Value)}},
		}
	}

	kind := domain.EvidenceDeclared
	var unverified []UnverifiedScope
	if condaadapter.IsGenericEnvName(declared.Value) {
		kind = domain.EvidenceInferred
		unverified = append(unverified, UnverifiedScope{Path: declared.SourcePath, Phase: PhaseResolveDependencies,
			Reason: fmt.Sprintf("conda environment name %q is too generic to trust as project-specific", declared.Value)})
	}

	now := a.Now
	if now == nil {
		now = time.Now
	}

	dependency := domain.Dependency{
		SourceType: domain.NodeProject,
		SourceID:   input.Project.ID,
		TargetType: domain.NodeResource,
		TargetID:   matched.ID,
		Relation:   domain.RelationRequires,
		Confidence: domain.DefaultConfidence[kind],
	}
	dependency.ID = domain.DependencyID(dependency.SourceType, dependency.SourceID, dependency.Relation, dependency.TargetType, dependency.TargetID)

	evidence := domain.Evidence{
		DependencyID:  dependency.ID,
		Kind:          kind,
		SourcePath:    declared.SourcePath,
		Property:      condaEnvPropertyName,
		RawValue:      declared.Value,
		ResolvedValue: matched.Name,
		CollectedAt:   now(),
	}
	evidence.ID = domain.EvidenceID(evidence.DependencyID, evidence.Kind, evidence.SourcePath, evidence.Property, evidence.RawValue, evidence.ResolvedValue)

	return DetectionResult[DependencyBundle]{
		Items:      []DependencyBundle{{Dependency: dependency, Evidence: []domain.Evidence{evidence}}},
		Unverified: unverified,
	}
}

// XcodeDependencyAnalyzer connects a detected Xcode or SwiftPM project to
// the single installed Xcode.app resource (CondaResourceDetector-style
// system resource, ResourceTypeXcodeInstall) -- the same "if the toolchain
// that builds this disappears, so does the project's ability to build"
// relationship MSBuildDependencyAnalyzer expresses for Windows SDK/.NET SDK.
//
// Unlike MSBuild, Xcode publishes no per-project "declared minimum Xcode
// version" MSBuild's WindowsTargetPlatformVersion resolves against -- the
// closest real signal is SwiftPM's swift-tools-version comment (carried as
// a ProjectProperty by SwiftPMProjectDetector), which is DECLARED evidence
// but not resolved against the installed Xcode's actual Swift version
// (xcodebuild -version reports the Xcode version, not the Swift tools
// version it ships). Plain Xcode projects have no such marker at all, so
// the edge there is EvidenceInferred: "this project type requires some
// Xcode to build, and exactly one was found," not a version match.
type XcodeDependencyAnalyzer struct {
	// Now returns the collection timestamp recorded on resolved Evidence.
	// Defaults to time.Now; tests may override it for determinism.
	Now func() time.Time
}

func (a XcodeDependencyAnalyzer) Analyze(ctx context.Context, input ProjectAnalysisInput, index ResourceIndex) DetectionResult[DependencyBundle] {
	if input.Project.Type != domain.ProjectTypeXcode && input.Project.Type != domain.ProjectTypeSwiftPM {
		return DetectionResult[DependencyBundle]{}
	}
	installs := index.ListByType(domain.ResourceTypeXcodeInstall)
	if len(installs) == 0 {
		return DetectionResult[DependencyBundle]{
			Unverified: []UnverifiedScope{{Path: input.Project.ManifestPath, Phase: PhaseResolveDependencies,
				Reason: "no Xcode installation detected on this machine to match against"}},
		}
	}
	// xcode.InstallLister only ever reports the one active Xcode.app, so
	// there is exactly one candidate to match, unlike Windows SDK/.NET SDK's
	// multiple side-by-side versions.
	target := installs[0]

	kind := domain.EvidenceInferred
	sourcePath := input.Project.ManifestPath
	var declaredValue string
	for _, property := range input.Properties {
		if property.Name == swiftpmadapter.ToolsVersionPropertyName {
			kind = domain.EvidenceDeclared
			declaredValue = property.Value
			sourcePath = property.SourcePath
			break
		}
	}

	now := a.Now
	if now == nil {
		now = time.Now
	}

	dependency := domain.Dependency{
		SourceType: domain.NodeProject, SourceID: input.Project.ID,
		TargetType: domain.NodeResource, TargetID: target.ID,
		Relation:   domain.RelationRequires,
		Confidence: domain.DefaultConfidence[kind],
	}
	dependency.ID = domain.DependencyID(dependency.SourceType, dependency.SourceID, dependency.Relation, dependency.TargetType, dependency.TargetID)

	evidence := domain.Evidence{
		DependencyID: dependency.ID, Kind: kind, SourcePath: sourcePath,
		Property: "xcode-install", RawValue: declaredValue, ResolvedValue: target.Version,
		CollectedAt: now(),
	}
	evidence.ID = domain.EvidenceID(evidence.DependencyID, evidence.Kind, evidence.SourcePath, evidence.Property, evidence.RawValue, evidence.ResolvedValue)

	return DetectionResult[DependencyBundle]{Items: []DependencyBundle{{Dependency: dependency, Evidence: []domain.Evidence{evidence}}}}
}
