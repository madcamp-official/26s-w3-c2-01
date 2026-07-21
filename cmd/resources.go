package cmd

import (
	"fmt"
	"strings"

	"github.com/madcamp-official/26s-w3-c2-01/internal/app"
	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
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

		service := app.NewResourceListService(sqlite.NewResourceRepository(db), sqlite.NewDependencyRepository(db))
		listings, err := service.List(cmd.Context(), func(resource domain.Resource) bool {
			if resourcesType != "" && !strings.EqualFold(string(resource.Type), resourcesType) {
				return false
			}
			if resourcesRisk != "" && !strings.EqualFold(string(resource.Risk), resourcesRisk) {
				return false
			}
			return true
		})
		if err != nil {
			return fmt.Errorf("list resources: %w", err)
		}

		view := output.ResourcesView{}
		for _, listing := range listings {
			resource := listing.Resource
			view.Resources = append(view.Resources, output.ResourceLine{
				Name:         resource.Name,
				Type:         resource.Type,
				Version:      resource.Version,
				Path:         resource.DisplayPath,
				LogicalSize:  resource.LogicalSize,
				ProjectCount: listing.ProjectCount,
				Regenerable:  resource.Regenerable,
				Risk:         resource.Risk,
				Confidence:   resource.Confidence,
				Reason:       resource.Reason,
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
