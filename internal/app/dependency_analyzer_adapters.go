package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	condaadapter "github.com/madcamp-official/26s-w3-c2-01/internal/adapter/conda"
	"github.com/madcamp-official/26s-w3-c2-01/internal/adapter/msbuild"
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
		Claim:         domain.ClaimRequiredDependency,
		Polarity:      domain.EvidenceSupports,
		Method:        "conda-environment-resolution",
		SourceFamily:  declared.SourcePath,
		SourcePath:    declared.SourcePath,
		Property:      condaEnvPropertyName,
		RawValue:      declared.Value,
		ResolvedValue: matched.Name,
		CollectedAt:   now(),
	}
	evidence.ID = domain.EvidenceID(evidence.DependencyID, evidence.Kind, evidence.Claim, evidence.Polarity, evidence.SourcePath, evidence.Property, evidence.RawValue, evidence.ResolvedValue)

	return DetectionResult[DependencyBundle]{
		Items:      []DependencyBundle{{Dependency: dependency, Evidence: []domain.Evidence{evidence}}},
		Unverified: unverified,
	}
}

// XcodeDependencyAnalyzer connects a detected Xcode project (.xcodeproj) to
// the active Xcode.app resource (ResourceTypeXcodeInstall) with a REQUIRES
// edge -- the same "if the toolchain that builds this disappears, so does
// the project's ability to build" relationship MSBuildDependencyAnalyzer
// expresses for Windows SDK/.NET SDK. Building an .xcodeproj genuinely needs
// full Xcode (`xcodebuild`), which is exactly what InstallLister reports (it
// excludes a Command-Line-Tools-only install), so that edge is real.
//
// SwiftPM (Package.swift) is deliberately NOT given this edge. A Swift
// package builds with `swift build` under *any* Swift toolchain -- Command
// Line Tools, a standalone toolchain, or Linux's -- not only this Xcode.app,
// so an unconditional REQUIRES-Xcode edge would make `impact
// xcode-install:<version>` falsely report SwiftPM projects as breaking when
// Xcode is removed. swift-tools-version, moreover, declares a Swift *tools
// compatibility level*, not a dependency on a specific Xcode install. Until
// a Swift-toolchain resource is modeled, SwiftPM records an UnverifiedScope
// (the relationship exists but isn't analyzed) instead of a wrong edge.
//
// The edge for .xcodeproj is always EvidenceInferred: a plain .xcodeproj
// carries no declared Xcode version, so this is "this project type requires
// some Xcode to build, and exactly one active install was found," not a
// version match.
type XcodeDependencyAnalyzer struct {
	// Now returns the collection timestamp recorded on resolved Evidence.
	// Defaults to time.Now; tests may override it for determinism.
	Now func() time.Time
}

func (a XcodeDependencyAnalyzer) Analyze(ctx context.Context, input ProjectAnalysisInput, index ResourceIndex) DetectionResult[DependencyBundle] {
	switch input.Project.Type {
	case domain.ProjectTypeXcode:
		// Proceed to build the REQUIRES edge below.
	case domain.ProjectTypeSwiftPM:
		return DetectionResult[DependencyBundle]{
			Unverified: []UnverifiedScope{{Path: input.Project.ManifestPath, Phase: PhaseResolveDependencies,
				Reason: "SwiftPM build toolchain is not modeled; a Package.swift builds with Command Line Tools, a standalone Swift toolchain, or Linux's Swift -- not only this Xcode.app -- so no REQUIRES edge to Xcode is asserted"}},
		}
	default:
		return DetectionResult[DependencyBundle]{}
	}

	installs := index.ListByType(domain.ResourceTypeXcodeInstall)
	if len(installs) == 0 {
		return DetectionResult[DependencyBundle]{
			Unverified: []UnverifiedScope{{Path: input.Project.ManifestPath, Phase: PhaseResolveDependencies,
				Reason: "no active Xcode installation detected on this machine to match against"}},
		}
	}
	// InstallLister only ever reports the one active Xcode.app, so there is
	// exactly one candidate to match, unlike Windows SDK/.NET SDK's multiple
	// side-by-side versions.
	target := installs[0]

	now := a.Now
	if now == nil {
		now = time.Now
	}

	dependency := domain.Dependency{
		SourceType: domain.NodeProject, SourceID: input.Project.ID,
		TargetType: domain.NodeResource, TargetID: target.ID,
		Relation:   domain.RelationRequires,
		Confidence: domain.DefaultConfidence[domain.EvidenceInferred],
	}
	dependency.ID = domain.DependencyID(dependency.SourceType, dependency.SourceID, dependency.Relation, dependency.TargetType, dependency.TargetID)

	evidence := domain.Evidence{
		DependencyID: dependency.ID, Kind: domain.EvidenceInferred,
		Claim: domain.ClaimRequiredDependency, Polarity: domain.EvidenceSupports,
		Method: "xcode-install-inference", SourceFamily: input.Project.ManifestPath,
		SourcePath: input.Project.ManifestPath,
		Property:   "xcode-install", ResolvedValue: target.Version,
		CollectedAt: now(),
	}
	evidence.ID = domain.EvidenceID(evidence.DependencyID, evidence.Kind, evidence.Claim, evidence.Polarity, evidence.SourcePath, evidence.Property, evidence.RawValue, evidence.ResolvedValue)

	return DetectionResult[DependencyBundle]{Items: []DependencyBundle{{Dependency: dependency, Evidence: []domain.Evidence{evidence}}}}
}
