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

// impactCmd represents the impact command.
var impactCmd = &cobra.Command{
	Use:   "impact <resource-id-or-path>",
	Short: "Show what breaks if a resource is removed",
	Long: `impact analyzes what happens to affected projects if a resource is
removed: whether already-built executables can still run, whether the
project rebuilds, whether IDE debugging still works, how to restore the
dependency, and any CI configuration that references it.`,
	Example: `  libra impact windows-sdk:10.0.22621.0
  libra impact "C:\Program Files (x86)\Windows Kits\10\Lib\10.0.22621.0"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := openDatabase()
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer db.Close()

		resourceRepo := sqlite.NewResourceRepository(db)
		projectRepo := sqlite.NewProjectRepository(db)

		resolved, err := resolveTarget(cmd.Context(), resourceRepo, projectRepo, args[0])
		if err != nil {
			if errors.Is(err, ErrTargetNotFound) || errors.Is(err, ErrTargetAmbiguous) {
				return err
			}
			return fmt.Errorf("resolve target %q: %w", args[0], err)
		}
		if resolved.Kind != targetKindResource {
			return fmt.Errorf("impact target must be a resource, got project %q", resolved.Project.Name)
		}
		resource := resolved.Resource

		// Two queries against the same graph, deliberately not one:
		// FindProjectsByResource gives the raw edges (which projects, so we
		// can look up their name/path to render), while ImpactService.Assess
		// gives the *judged* level per project+scope -- it's app-layer
		// policy, not something this command re-derives from the edges
		// itself. Keeping cmd a thin renderer over both, rather than
		// reimplementing Assess's judgment inline, is what lets this command
		// pick up ImpactService's logic unchanged as it grows more scopes.
		dependencies := sqlite.NewDependencyRepository(db)
		edges, err := dependencies.FindProjectsByResource(cmd.Context(), resource.ID)
		if err != nil {
			return fmt.Errorf("find projects depending on %q: %w", resource.ID, err)
		}

		assessments, err := app.NewImpactService(dependencies).Assess(cmd.Context(), resource.ID)
		if err != nil {
			return fmt.Errorf("assess impact of %q: %w", resource.ID, err)
		}
		levelByProjectScope := make(map[string]map[domain.ImpactScope]domain.ImpactLevel, len(assessments))
		for _, a := range assessments {
			byScope := levelByProjectScope[a.ProjectID]
			if byScope == nil {
				byScope = make(map[domain.ImpactScope]domain.ImpactLevel)
				levelByProjectScope[a.ProjectID] = byScope
			}
			byScope[a.Scope] = a.Level
		}

		view := output.ImpactView{}
		for _, edge := range edges {
			project, err := projectRepo.FindByID(cmd.Context(), edge.SourceID)
			if err != nil {
				return fmt.Errorf("find affected project %q: %w", edge.SourceID, err)
			}

			projectView := output.ImpactProjectView{
				ProjectName: project.Name,
				ProjectPath: project.RootPath,
				Recovery:    output.RecoveryHint(resource.Type),
			}
			// Every scope in impactScopes always renders a line (see that
			// var's doc comment in cmd/target.go): default to UNKNOWN, and
			// only overwrite it for a scope ImpactService actually returned
			// an assessment for.
			for _, scope := range impactScopes {
				level := domain.ImpactLevelUnknown
				if l, ok := levelByProjectScope[project.ID][scope]; ok {
					level = l
				}
				projectView.Scopes = append(projectView.Scopes, output.ImpactScopeLine{
					Scope:  scope,
					Level:  level,
					Phrase: output.ImpactPhrase(scope, level),
				})
			}
			view.Projects = append(view.Projects, projectView)
		}

		return output.New(cmd.OutOrStdout(), jsonOutput).Print(view)
	},
}

func init() {
	rootCmd.AddCommand(impactCmd)
}
