package cmd

import (
	"fmt"
	"strings"

	"github.com/madcamp-official/26s-w3-c2-01/internal/output"
	"github.com/madcamp-official/26s-w3-c2-01/internal/store/sqlite"
	"github.com/spf13/cobra"
)

// projects.go reads straight from ProjectRepository/DependencyRepository
// (no application service in between, unlike cmd/scan.go and
// cmd/summary.go) since all it does is list, filter, and count -- see
// docs/libra_review_findings_day4.md §5 for why this is flagged as a
// structural inconsistency worth a team decision rather than fixed here.
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

		projects, err := sqlite.NewProjectRepository(db).List(cmd.Context())
		if err != nil {
			return fmt.Errorf("list projects: %w", err)
		}
		dependencies := sqlite.NewDependencyRepository(db)

		view := output.ProjectsView{}
		for _, project := range projects {
			if projectsType != "" && !strings.EqualFold(string(project.Type), projectsType) {
				continue
			}
			if projectsDrive != "" && !strings.EqualFold(project.Drive, projectsDrive) {
				continue
			}
			if projectsStatus != "" && !strings.EqualFold(string(project.Status), projectsStatus) {
				continue
			}

			resources, err := dependencies.FindResourcesByProject(cmd.Context(), project.ID)
			if err != nil {
				return fmt.Errorf("count resources for project %q: %w", project.ID, err)
			}

			view.Projects = append(view.Projects, output.ProjectLine{
				Name:           project.Name,
				Path:           project.RootPath,
				Type:           project.Type,
				Drive:          project.Drive,
				LogicalSize:    project.LogicalSize,
				LastModifiedAt: project.LastModifiedAt,
				LastObservedAt: project.LastObservedAt,
				Status:         project.Status,
				ResourceCount:  len(resources),
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
