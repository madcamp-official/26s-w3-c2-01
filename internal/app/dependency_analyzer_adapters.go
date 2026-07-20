package app

import (
	"context"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/adapter/msbuild"
	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

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
