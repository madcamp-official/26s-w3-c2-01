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

var (
	projectsType   string
	projectsDrive  string
	projectsStatus string
)

// projectsCmd represents the projects command.
var projectsCmd = &cobra.Command{
	Use:   "projects",
	Short: "List discovered projects and their activity status",
	Long: `projects lists every project libra has discovered from the last
scan: its name, path, type, drive, logical size, last modified and
observed times, activity status, and how many resources it depends on.`,
	Example: `  libra projects
  libra projects --type node
  libra projects --drive D:
  libra projects --status stale`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := openDatabase()
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer db.Close()

		service := app.NewProjectListService(sqlite.NewProjectRepository(db), sqlite.NewDependencyRepository(db))
		listings, err := service.List(cmd.Context(), func(project domain.BuildProject) bool {
			if projectsType != "" && !strings.EqualFold(string(project.Type), projectsType) {
				return false
			}
			if projectsDrive != "" && !strings.EqualFold(project.Drive, projectsDrive) {
				return false
			}
			if projectsStatus != "" && !strings.EqualFold(string(project.Status), projectsStatus) {
				return false
			}
			return true
		})
		if err != nil {
			return fmt.Errorf("list projects: %w", err)
		}

		view := output.ProjectsView{}
		for _, listing := range listings {
			project := listing.Project
			view.Projects = append(view.Projects, output.ProjectLine{
				Name:           project.Name,
				Path:           project.RootPath,
				Type:           project.Type,
				Drive:          project.Drive,
				LogicalSize:    project.LogicalSize,
				LastModifiedAt: project.LastModifiedAt,
				LastObservedAt: project.LastObservedAt,
				Status:         project.Status,
				ResourceCount:  listing.ResourceCount,
			})
		}

		return output.New(cmd.OutOrStdout(), jsonOutput).Print(view)
	},
}

func init() {
	rootCmd.AddCommand(projectsCmd)

	projectsCmd.Flags().StringVar(&projectsType, "type", "", "filter by project type (e.g. node)")
	projectsCmd.Flags().StringVar(&projectsDrive, "drive", "", "filter by drive (e.g. D:)")
	projectsCmd.Flags().StringVar(&projectsStatus, "status", "", "filter by activity status (e.g. stale)")
}
