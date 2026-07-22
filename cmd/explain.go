package cmd

import (
	"errors"
	"fmt"

	"github.com/madcamp-official/26s-w3-c2-01/internal/app"
	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/output"
	"github.com/madcamp-official/26s-w3-c2-01/internal/store/sqlite"
	"github.com/spf13/cobra"
)

// explainImpactLabel returns the sentence-style label for one impact scope
// in `libra explain`'s "Expected impact" section (F-07 in
// docs/libra_cli_commands_and_schedule.md's example output). RUN and BUILD
// are IDE-neutral ("Existing executable launch", "Rebuild"), but DEBUG names
// a specific IDE only for resource types where that IDE is unambiguous --
// a Windows/.NET SDK always implies Visual Studio, a CocoaPods Pods/
// directory or the active Xcode install always implies Xcode -- and falls
// back to the neutral "IDE debugging" for resource types (build-output,
// node_modules, ...) that are shared across ecosystems and don't imply any
// one editor. Previously this was a single fixed "Visual Studio debugging"
// label regardless of resource type, which showed up verbatim even when
// explaining a Node or Xcode resource on macOS.
func explainImpactLabel(scope domain.ImpactScope, resourceType domain.ResourceType) string {
	switch scope {
	case domain.ImpactScopeRun:
		return "Existing executable launch"
	case domain.ImpactScopeBuild:
		return "Rebuild"
	case domain.ImpactScopeDebug:
		switch resourceType {
		case domain.ResourceTypeWindowsSDK, domain.ResourceTypeNetFXSDK, domain.ResourceTypeVisualStudio, domain.ResourceTypeMSBuild, domain.ResourceTypeDotNetSDK:
			return "Visual Studio debugging"
		case domain.ResourceTypeXcodeInstall, domain.ResourceTypePods:
			return "Xcode debugging"
		default:
			return "IDE debugging"
		}
	default:
		return string(scope)
	}
}

// explainCmd represents the explain command.
var explainCmd = &cobra.Command{
	Use:   "explain <resource-id-or-path>",
	Short: "Explain what a project or resource is and why it exists",
	Long: `explain describes a single project or resource: its kind, path,
size, when it was created or last modified, which projects reference it,
the evidence behind that dependency, whether it can be regenerated, the
expected impact of deleting it, how to recover it, its risk level, and the
confidence of the analysis.`,
	Example: `  libra explain windows-sdk:10.0.22621.0
  libra explain "D:\Projects\OldWeb\node_modules"
  libra explain project:"D:\Projects\GameClient"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := openDatabase()
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer db.Close()

		resourceRepo := sqlite.NewResourceRepository(db)
		projectRepo := sqlite.NewProjectRepository(db)
		dependencyRepo := sqlite.NewDependencyRepository(db)

		resolved, err := resolveTarget(cmd.Context(), resourceRepo, projectRepo, args[0])
		if err != nil {
			if errors.Is(err, ErrTargetNotFound) || errors.Is(err, ErrTargetAmbiguous) {
				return err
			}
			return fmt.Errorf("resolve target %q: %w", args[0], err)
		}

		service := app.NewExplainService(resourceRepo, projectRepo, dependencyRepo)

		var view output.ExplainView
		if resolved.Kind == targetKindProject {
			view, err = renderProjectExplanation(cmd, service, resolved.Project.ID)
		} else {
			view, err = renderResourceExplanation(cmd, service, resolved.Resource.ID)
		}
		if err != nil {
			return err
		}

		return output.New(cmd.OutOrStdout(), jsonOutput, "explain").Print(view)
	},
}

func init() {
	rootCmd.AddCommand(explainCmd)
}

// renderResourceExplanation builds the resource-shaped half of ExplainView.
// It walks the fixed impactScopes list (shared with cmd/impact.go) rather
// than explanation.Impact directly, so every scope always renders a line --
// a scope app.ImpactService didn't judge (RUN/DEBUG today) still shows up
// as UNKNOWN instead of silently disappearing from the output.
func renderResourceExplanation(cmd *cobra.Command, service *app.ExplainService, resourceID string) (output.ExplainView, error) {
	explanation, err := service.ExplainResource(cmd.Context(), resourceID)
	if err != nil {
		return output.ExplainView{}, fmt.Errorf("explain resource %q: %w", resourceID, err)
	}
	resource := explanation.Resource
	confidenceSummary := resource.ConfidenceProfile.CleanupSummary()

	view := output.ExplainView{
		Kind:              output.ExplainKindResource,
		Name:              resource.Name,
		Path:              resource.DisplayPath,
		ResourceType:      resource.Type,
		Version:           resource.Version,
		Regenerable:       &resource.Regenerable,
		Risk:              resource.Risk,
		Confidence:        &resource.Confidence,
		ConfidenceProfile: &resource.ConfidenceProfile,
		ConfidenceSummary: &confidenceSummary,
		RiskReasons:       resource.RiskReasons,
		LogicalSize:       resource.LogicalSize,
		LastObservedAt:    resource.LastObservedAt,
		Recovery:          output.RecoveryHint(resource.Type),
		Unverified:        explainUnverifiedFromConfidence(resource.ConfidenceProfile),
	}
	for _, usage := range explanation.UsedBy {
		view.UsedBy = append(view.UsedBy, output.ExplainUsage{
			Name: usage.ProjectName, Path: usage.ProjectPath,
			Relation: usage.Relation, Evidence: toEvidenceLines(usage.Evidence),
		})
	}
	for _, scope := range impactScopes {
		level := domain.ImpactLevelUnknown
		for _, a := range explanation.Impact {
			if a.Scope == scope {
				level = a.Level
			}
		}
		line := output.ExplainImpactLine{Label: explainImpactLabel(scope, resource.Type), Scope: scope, Level: level}
		if scope == domain.ImpactScopeDebug && level == domain.ImpactLevelHigh {
			line.Note = "when rebuild occurs"
		}
		view.ExpectedImpact = append(view.ExpectedImpact, line)
	}
	return view, nil
}

// renderProjectExplanation builds the project-shaped half of ExplainView.
// Unlike the resource case, there's no fixed set of scopes to render here --
// a project's "Uses" list is just whatever dependency edges exist for it, in
// whatever order ExplainProject's own query returns them.
func renderProjectExplanation(cmd *cobra.Command, service *app.ExplainService, projectID string) (output.ExplainView, error) {
	explanation, err := service.ExplainProject(cmd.Context(), projectID)
	if err != nil {
		return output.ExplainView{}, fmt.Errorf("explain project %q: %w", projectID, err)
	}
	project := explanation.Project

	view := output.ExplainView{
		Kind:           output.ExplainKindProject,
		Name:           project.Name,
		Path:           project.RootPath,
		ProjectType:    project.Type,
		Status:         project.Status,
		LogicalSize:    project.LogicalSize,
		SizeKnown:      &project.SizeKnown,
		LastModifiedAt: &project.LastModifiedAt,
		LastObservedAt: project.LastObservedAt,
	}
	for _, usage := range explanation.Requires {
		view.Requires = append(view.Requires, output.ExplainUsage{
			Name: usage.ResourceName, Relation: usage.Relation, Evidence: toEvidenceLines(usage.Evidence),
		})
	}
	return view, nil
}

// explainUnverifiedFromConfidence turns each ConfidenceProfile axis whose
// Status isn't KNOWN into one "분석하지 못한 범위" line. It reads directly off
// whatever the claim-based confidence model (internal/app/confidence_claims.go)
// already computed and persisted -- no separate UnverifiedScope wiring,
// which stays scan-run-scoped and unpersisted (docs/libra_integration_contracts.md
// §13). Dependency/ScanCoverage get a distinct message because their
// non-KNOWN status isn't resource-specific -- every resource gets the same
// conservative baseline today, pending per-resource UnverifiedScope
// attribution (§20.2) -- while the rest (Ownership/Regenerability/PathSafety/
// Freshness/Classification) vary per resource and get their LimitingClaim
// surfaced when there is one. Returns nil for a legacy resource that was
// never re-scanned since the claim-based model landed (no per-axis
// Assessments to read, ModelVersion 0) rather than guessing.
func explainUnverifiedFromConfidence(profile domain.ConfidenceProfile) []string {
	var lines []string
	for _, a := range profile.Assessments {
		if a.Status == domain.ConfidenceKnown {
			continue
		}
		if a.Axis == domain.AxisDependency || a.Axis == domain.AxisScanCoverage {
			lines = append(lines, fmt.Sprintf("%s uses a conservative baseline (%d%%) until per-resource scope tracking is connected", a.Axis, a.Score))
			continue
		}
		line := fmt.Sprintf("%s confidence is %s (%d%%)", a.Axis, a.Status, a.Score)
		if a.LimitingClaim != "" {
			line += fmt.Sprintf(" -- limited by %s", a.LimitingClaim)
		}
		lines = append(lines, line)
	}
	return lines
}

// toEvidenceLines drops Evidence fields explain has no use for (ID,
// DependencyID, CollectedAt, ResolvedValue) rather than reusing
// domain.Evidence directly as the JSON shape, so the CLI's public
// output schema doesn't silently change if that struct grows a field.
func toEvidenceLines(evidence []domain.Evidence) []output.ExplainEvidenceLine {
	lines := make([]output.ExplainEvidenceLine, 0, len(evidence))
	for _, e := range evidence {
		lines = append(lines, output.ExplainEvidenceLine{Kind: e.Kind, Source: e.SourcePath, Property: e.Property})
	}
	return lines
}
