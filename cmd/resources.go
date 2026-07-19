package cmd

import (
	"fmt"
	"strings"

	"github.com/madcamp-official/26s-w3-c2-01/internal/output"
	"github.com/madcamp-official/26s-w3-c2-01/internal/store/sqlite"
	"github.com/spf13/cobra"
)

// resourcesType/resourcesRisk are bound to --type/--risk by init() below and
// read directly by resourcesCmd.RunE, the same package-level-flag-variable
// pattern every other cmd/*.go command in this package uses (see
// cmd/root.go's jsonOutput etc.).
var (
	resourcesType string
	resourcesRisk string
)

// resourcesCmd represents the resources command.
var resourcesCmd = &cobra.Command{
	Use:   "resources",
	Short: "List discovered SDKs, tools, caches, and build artifacts",
	Long: `resources lists every development resource libra has discovered
from the last scan: its name, type, version, path, logical size, how many
projects depend on it, whether it can be regenerated, its risk level, and
the confidence of the analysis.`,
	Example: `  libra resources
  libra resources --type windows-sdk
  libra resources --type build-output
  libra resources --risk review`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := openDatabase()
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer db.Close()

		// List() returns every resource; filtering happens here in cmd rather
		// than as a repository query, matching cmd/projects.go's --type/
		// --drive/--status filters -- keeps ResourceRepository's contract
		// small (internal/app/resource_repository.go) and filter logic in one
		// place (this file) instead of one query variant per flag.
		resources, err := sqlite.NewResourceRepository(db).List(cmd.Context())
		if err != nil {
			return fmt.Errorf("list resources: %w", err)
		}
		dependencies := sqlite.NewDependencyRepository(db)

		view := output.ResourcesView{}
		for _, resource := range resources {
			if resourcesType != "" && string(resource.Type) != resourcesType {
				continue
			}
			if resourcesRisk != "" && !strings.EqualFold(string(resource.Risk), resourcesRisk) {
				continue
			}

			// One dependency-graph query per surviving resource (not one
			// query total) so unfiltered-out resources never pay for a count
			// the user didn't ask to see. Same N+1-by-design tradeoff
			// cmd/projects.go makes for its own resource-count column.
			projects, err := dependencies.FindProjectsByResource(cmd.Context(), resource.ID)
			if err != nil {
				return fmt.Errorf("count projects for resource %q: %w", resource.ID, err)
			}

			view.Resources = append(view.Resources, output.ResourceLine{
				Name:         resource.Name,
				Type:         resource.Type,
				Version:      resource.Version,
				Path:         resource.DisplayPath,
				LogicalSize:  resource.LogicalSize,
				ProjectCount: len(projects),
				Regenerable:  resource.Regenerable,
				Risk:         resource.Risk,
				Confidence:   resource.Confidence,
			})
		}

		return output.New(cmd.OutOrStdout(), jsonOutput).Print(view)
	},
}

func init() {
	rootCmd.AddCommand(resourcesCmd)

	resourcesCmd.Flags().StringVar(&resourcesType, "type", "", "filter by resource type (e.g. windows-sdk)")
	resourcesCmd.Flags().StringVar(&resourcesRisk, "risk", "", "filter by risk level (e.g. review)")
}
