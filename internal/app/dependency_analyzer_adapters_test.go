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
