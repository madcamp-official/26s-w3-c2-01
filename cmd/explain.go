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

// explainImpactLabels maps an impact scope to the sentence-style label used
// in `libra explain`'s "Expected impact" section (F-07 in
// docs/libra_cli_commands_and_schedule.md's example output).
var explainImpactLabels = map[domain.ImpactScope]string{
	domain.ImpactScopeRun:   "Existing executable launch",
	domain.ImpactScopeBuild: "Rebuild",
	domain.ImpactScopeDebug: "Visual Studio debugging",
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

		return output.New(cmd.OutOrStdout(), jsonOutput).Print(view)
	},
}

func init() {
	rootCmd.AddCommand(explainCmd)
}

func renderResourceExplanation(cmd *cobra.Command, service *app.ExplainService, resourceID string) (output.ExplainView, error) {
	explanation, err := service.ExplainResource(cmd.Context(), resourceID)
	if err != nil {
		return output.ExplainView{}, fmt.Errorf("explain resource %q: %w", resourceID, err)
	}
	resource := explanation.Resource

	view := output.ExplainView{
		Kind:           output.ExplainKindResource,
		Name:           resource.Name,
		Path:           resource.DisplayPath,
		ResourceType:   resource.Type,
		Version:        resource.Version,
		Regenerable:    &resource.Regenerable,
		Risk:           resource.Risk,
		Confidence:     &resource.Confidence,
		LogicalSize:    resource.LogicalSize,
		LastObservedAt: resource.LastObservedAt,
		Recovery:       output.RecoveryHint(resource.Type),
	}
	for _, usage := range explanation.UsedBy {
		view.UsedBy = append(view.UsedBy, output.ExplainUsage{
			Name: usage.ProjectName, Path: usage.ProjectPath,
			Evidence: toEvidenceLines(usage.Evidence),
		})
	}
	for _, scope := range impactScopes {
		level := domain.ImpactLevelUnknown
		for _, a := range explanation.Impact {
			if a.Scope == scope {
				level = a.Level
			}
		}
		line := output.ExplainImpactLine{Label: explainImpactLabels[scope], Scope: scope, Level: level}
		if scope == domain.ImpactScopeDebug && level == domain.ImpactLevelHigh {
			line.Note = "when rebuild occurs"
		}
		view.ExpectedImpact = append(view.ExpectedImpact, line)
	}
	return view, nil
}

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
		LastModifiedAt: project.LastModifiedAt,
		LastObservedAt: project.LastObservedAt,
	}
	for _, usage := range explanation.Requires {
		view.Requires = append(view.Requires, output.ExplainUsage{
			Name: usage.ResourceName, Evidence: toEvidenceLines(usage.Evidence),
		})
	}
	return view, nil
}

func toEvidenceLines(evidence []domain.Evidence) []output.ExplainEvidenceLine {
	lines := make([]output.ExplainEvidenceLine, 0, len(evidence))
	for _, e := range evidence {
		lines = append(lines, output.ExplainEvidenceLine{Kind: e.Kind, Source: e.SourcePath, Property: e.Property})
	}
	return lines
}
