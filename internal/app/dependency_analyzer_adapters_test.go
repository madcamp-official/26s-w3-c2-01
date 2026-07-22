package app

import (
	"context"
	"testing"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

func TestMSBuildDependencyAnalyzerResolvesWindowsSDKDependency(t *testing.T) {
	sdk := domain.Resource{ID: "resource-sdk", Type: domain.ResourceTypeWindowsSDK, Version: "10.0.22621.0"}
	index := newMemoryResourceIndex([]domain.Resource{sdk})
	collectedAt := time.Date(2026, 7, 20, 9, 0, 0, 0, time.UTC)

	input := ProjectAnalysisInput{
		Project: domain.BuildProject{ID: "project-1", ManifestPath: `D:\Projects\Game\Game.vcxproj`},
		Properties: []ProjectProperty{
			{OwnerManifestPath: `D:\Projects\Game\Game.vcxproj`, SourcePath: `D:\Projects\Game\Game.vcxproj`,
				Name: "WindowsTargetPlatformVersion", Value: "10.0"},
		},
	}

	analyzer := MSBuildDependencyAnalyzer{Now: func() time.Time { return collectedAt }}
	got := analyzer.Analyze(context.Background(), input, index)

	if len(got.Items) != 1 {
		t.Fatalf("Analyze() items = %#v, want one resolved dependency", got.Items)
	}
	bundle := got.Items[0]
	if bundle.Dependency.SourceID != "project-1" || bundle.Dependency.TargetID != "resource-sdk" {
		t.Errorf("dependency = %#v, want project-1 -> resource-sdk", bundle.Dependency)
	}
	if len(bundle.Evidence) != 1 || !bundle.Evidence[0].CollectedAt.Equal(collectedAt) {
		t.Errorf("evidence = %#v, want CollectedAt = %v", bundle.Evidence, collectedAt)
	}
}

func TestMSBuildDependencyAnalyzerReportsUnverifiedConditionalProperty(t *testing.T) {
	index := newMemoryResourceIndex(nil)
	input := ProjectAnalysisInput{
		Project: domain.BuildProject{ID: "project-1", ManifestPath: `D:\Projects\Game\Game.vcxproj`},
		Properties: []ProjectProperty{
			{Name: "WindowsTargetPlatformVersion", Value: "10.0", Condition: "'$(Configuration)' == 'Debug'"},
		},
	}

	got := (MSBuildDependencyAnalyzer{}).Analyze(context.Background(), input, index)

	if len(got.Items) != 0 || len(got.Unverified) != 1 {
		t.Fatalf("Analyze() = %#v, want zero items and one unverified scope", got)
	}
	if got.Unverified[0].Path != `D:\Projects\Game\Game.vcxproj` || got.Unverified[0].Phase != PhaseResolveDependencies {
		t.Errorf("unverified = %#v, want manifest path and RESOLVE_DEPENDENCIES phase", got.Unverified[0])
	}
}

func TestMSBuildDependencyAnalyzerSatisfiesDependencyAnalyzer(t *testing.T) {
	var _ DependencyAnalyzer = MSBuildDependencyAnalyzer{}
}

func TestCondaDependencyAnalyzerMatchesDeclaredEnvironment(t *testing.T) {
	env := domain.Resource{ID: "resource-conda-env", Type: domain.ResourceTypeCondaEnv, Name: "myproject"}
	index := newMemoryResourceIndex([]domain.Resource{env})
	collectedAt := time.Date(2026, 7, 21, 9, 0, 0, 0, time.UTC)

	input := ProjectAnalysisInput{
		Project: domain.BuildProject{ID: "project-1", ManifestPath: `/repo/pyproject.toml`},
		Properties: []ProjectProperty{
			{OwnerManifestPath: `/repo/pyproject.toml`, SourcePath: `/repo/environment.yml`,
				Name: condaEnvPropertyName, Value: "myproject"},
		},
	}

	analyzer := CondaDependencyAnalyzer{Now: func() time.Time { return collectedAt }}
	got := analyzer.Analyze(context.Background(), input, index)

	if len(got.Items) != 1 {
		t.Fatalf("Analyze() items = %#v, want one resolved dependency", got.Items)
	}
	bundle := got.Items[0]
	if bundle.Dependency.SourceID != "project-1" || bundle.Dependency.TargetID != "resource-conda-env" {
		t.Errorf("dependency = %#v, want project-1 -> resource-conda-env", bundle.Dependency)
	}
	if bundle.Dependency.Relation != domain.RelationRequires {
		t.Errorf("Relation = %q, want REQUIRES (결정 4)", bundle.Dependency.Relation)
	}
	if bundle.Dependency.Confidence != domain.DefaultConfidence[domain.EvidenceDeclared] {
		t.Errorf("Confidence = %d, want DECLARED-strength for a specific project name", bundle.Dependency.Confidence)
	}
	if len(got.Unverified) != 0 {
		t.Errorf("Unverified = %#v, want none for a specific, matched name", got.Unverified)
	}
}

func TestCondaDependencyAnalyzerDegradesGenericEnvName(t *testing.T) {
	env := domain.Resource{ID: "resource-conda-env", Type: domain.ResourceTypeCondaEnv, Name: "base"}
	index := newMemoryResourceIndex([]domain.Resource{env})

	input := ProjectAnalysisInput{
		Project: domain.BuildProject{ID: "project-1"},
		Properties: []ProjectProperty{
			{Name: condaEnvPropertyName, Value: "base"},
		},
	}

	got := (CondaDependencyAnalyzer{}).Analyze(context.Background(), input, index)

	if len(got.Items) != 1 {
		t.Fatalf("Analyze() items = %#v, want one dependency even for a generic name (결정 5)", got.Items)
	}
	if got.Items[0].Dependency.Confidence != domain.DefaultConfidence[domain.EvidenceInferred] {
		t.Errorf("Confidence = %d, want INFERRED-strength for a generic env name", got.Items[0].Dependency.Confidence)
	}
	if len(got.Unverified) != 1 {
		t.Errorf("Unverified = %#v, want one scope explaining the generic-name downgrade", got.Unverified)
	}
}

func TestCondaDependencyAnalyzerNoMatchReportsUnverified(t *testing.T) {
	index := newMemoryResourceIndex(nil)
	input := ProjectAnalysisInput{
		Project: domain.BuildProject{ID: "project-1"},
		Properties: []ProjectProperty{
			{SourcePath: `/repo/environment.yml`, Name: condaEnvPropertyName, Value: "myproject"},
		},
	}

	got := (CondaDependencyAnalyzer{}).Analyze(context.Background(), input, index)

	if len(got.Items) != 0 || len(got.Unverified) != 1 {
		t.Fatalf("Analyze() = %#v, want zero items and one unverified scope", got)
	}
}

func TestCondaDependencyAnalyzerNoDeclaration(t *testing.T) {
	index := newMemoryResourceIndex(nil)
	input := ProjectAnalysisInput{Project: domain.BuildProject{ID: "project-1"}}

	got := (CondaDependencyAnalyzer{}).Analyze(context.Background(), input, index)

	if len(got.Items) != 0 || len(got.Unverified) != 0 {
		t.Fatalf("Analyze() = %#v, want an empty result when no conda-env property is declared", got)
	}
}

func TestCondaDependencyAnalyzerSatisfiesDependencyAnalyzer(t *testing.T) {
	var _ DependencyAnalyzer = CondaDependencyAnalyzer{}
}

func TestXcodeDependencyAnalyzerDoesNotRequireXcodeForSwiftPM(t *testing.T) {
	// Even with an active Xcode installed AND a declared swift-tools-version,
	// a SwiftPM project must NOT get a REQUIRES-Xcode edge: swift build runs
	// under any Swift toolchain. It records an UnverifiedScope instead so the
	// unmodeled toolchain relationship is not silently dropped.
	install := domain.Resource{ID: "resource-xcode", Type: domain.ResourceTypeXcodeInstall, Version: "15.4"}
	index := newMemoryResourceIndex([]domain.Resource{install})

	input := ProjectAnalysisInput{
		Project: domain.BuildProject{ID: "project-1", Type: domain.ProjectTypeSwiftPM, ManifestPath: "/repo/Package.swift"},
		Properties: []ProjectProperty{
			{OwnerManifestPath: "/repo/Package.swift", SourcePath: "/repo/Package.swift", Name: "swift-tools-version", Value: "5.9"},
		},
	}

	got := (XcodeDependencyAnalyzer{}).Analyze(context.Background(), input, index)
	if len(got.Items) != 0 {
		t.Fatalf("Analyze() items = %#v, want no dependency edge for a SwiftPM project", got.Items)
	}
	if len(got.Unverified) != 1 {
		t.Fatalf("Analyze() unverified = %#v, want one scope noting the unmodeled Swift toolchain", got.Unverified)
	}
}

func TestXcodeDependencyAnalyzerInfersPlainXcodeProject(t *testing.T) {
	install := domain.Resource{ID: "resource-xcode", Type: domain.ResourceTypeXcodeInstall, Version: "15.4"}
	index := newMemoryResourceIndex([]domain.Resource{install})

	input := ProjectAnalysisInput{Project: domain.BuildProject{ID: "project-1", Type: domain.ProjectTypeXcode, ManifestPath: "/repo/App.xcodeproj/project.pbxproj"}}

	got := (XcodeDependencyAnalyzer{}).Analyze(context.Background(), input, index)
	if len(got.Items) != 1 || got.Items[0].Evidence[0].Kind != domain.EvidenceInferred {
		t.Fatalf("Analyze() = %#v, want one INFERRED dependency (no declared version marker for plain .xcodeproj)", got)
	}
}

func TestXcodeDependencyAnalyzerReportsUnverifiedWhenNoXcodeInstalled(t *testing.T) {
	index := newMemoryResourceIndex(nil)
	input := ProjectAnalysisInput{Project: domain.BuildProject{ID: "project-1", Type: domain.ProjectTypeXcode, ManifestPath: "/repo/App.xcodeproj/project.pbxproj"}}

	got := (XcodeDependencyAnalyzer{}).Analyze(context.Background(), input, index)
	if len(got.Items) != 0 || len(got.Unverified) != 1 {
		t.Fatalf("Analyze() = %#v, want no dependency and one UnverifiedScope", got)
	}
}

func TestXcodeDependencyAnalyzerIgnoresOtherProjectTypes(t *testing.T) {
	index := newMemoryResourceIndex([]domain.Resource{{ID: "resource-xcode", Type: domain.ResourceTypeXcodeInstall}})
	input := ProjectAnalysisInput{Project: domain.BuildProject{ID: "project-1", Type: domain.ProjectTypeNode}}

	got := (XcodeDependencyAnalyzer{}).Analyze(context.Background(), input, index)
	if len(got.Items) != 0 || len(got.Unverified) != 0 {
		t.Fatalf("Analyze() = %#v, want an empty result for a non-Xcode/SwiftPM project", got)
	}
}

func TestXcodeDependencyAnalyzerSatisfiesDependencyAnalyzer(t *testing.T) {
	var _ DependencyAnalyzer = XcodeDependencyAnalyzer{}
}
