package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/madcamp-official/26s-w3-c2-01/internal/app"
	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/output"
	"github.com/madcamp-official/26s-w3-c2-01/internal/pathutil"
	"github.com/madcamp-official/26s-w3-c2-01/internal/store/sqlite"
	"github.com/spf13/cobra"
)

// projectsDefaultLimit caps unfiltered `libra projects` output (issue #41):
// a scan across a real machine can report hundreds of rows, most of which a
// user has no reason to look at right now. --all disables the cap entirely.
const projectsDefaultLimit = 20

var (
	projectsType   string
	projectsDrive  string
	projectsStatus string
	projectsName   string
	projectsUnder  string
	projectsSort   string
	projectsAll    bool
)

// projectsCmd represents the projects command.
var projectsCmd = &cobra.Command{
	Use:   "projects",
	Short: "List discovered projects and their activity status",
	Long: `projects lists projects libra has discovered from the last scan:
name, path, type, drive, logical size, last modified and observed times,
activity status, and how many resources it depends on. Output is capped at
the top 20 by default (use --all to see every match).`,
	Example: `  libra projects
  libra projects --all
  libra projects --sort size
  libra projects --sort modified
  libra projects --name frontend
  libra projects --under D:\Work
  libra projects --type node
  libra projects --drive D:
  libra projects --status stale`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateProjectsSortFlag(projectsSort); err != nil {
			return err
		}
		var underNormalized string
		if projectsUnder != "" {
			normalized, err := pathutil.Normalize(projectsUnder)
			if err != nil {
				return fmt.Errorf("normalize --under %q: %w", projectsUnder, err)
			}
			underNormalized = normalized
		}

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
			if projectsName != "" && !strings.Contains(strings.ToLower(project.Name), strings.ToLower(projectsName)) {
				return false
			}
			if underNormalized != "" {
				within, err := pathutil.IsSameOrChild(project.NormalizedRootPath, underNormalized)
				if err != nil || !within {
					return false
				}
			}
			return true
		})
		if err != nil {
			return fmt.Errorf("list projects: %w", err)
		}

		switch projectsSort {
		case "modified":
			sort.SliceStable(listings, func(i, j int) bool {
				return listings[i].Project.LastModifiedAt.After(listings[j].Project.LastModifiedAt)
			})
		case "size":
			sort.SliceStable(listings, func(i, j int) bool {
				return listings[i].Project.LogicalSize > listings[j].Project.LogicalSize
			})
		}

		total := len(listings)
		if !projectsAll && total > projectsDefaultLimit {
			listings = listings[:projectsDefaultLimit]
		}

		view := output.ProjectsView{TotalCount: total}
		for _, listing := range listings {
			project := listing.Project
			view.Projects = append(view.Projects, output.ProjectLine{
				Name:           project.Name,
				Path:           project.RootPath,
				Type:           project.Type,
				Drive:          project.Drive,
				LogicalSize:    project.LogicalSize,
				SizeKnown:      project.SizeKnown,
				LastModifiedAt: project.LastModifiedAt,
				LastObservedAt: project.LastObservedAt,
				Status:         project.Status,
				ResourceCount:  listing.ResourceCount,
			})
		}

		return output.New(cmd.OutOrStdout(), jsonOutput, "projects").Print(view)
	},
}

func init() {
	rootCmd.AddCommand(projectsCmd)

	projectsCmd.Flags().StringVar(&projectsType, "type", "", "filter by project type (e.g. node)")
	projectsCmd.Flags().StringVar(&projectsDrive, "drive", "", "filter by drive (e.g. D:)")
	projectsCmd.Flags().StringVar(&projectsStatus, "status", "", "filter by activity status (e.g. stale)")
	projectsCmd.Flags().StringVar(&projectsName, "name", "", "filter by project name (case-insensitive substring match)")
	projectsCmd.Flags().StringVar(&projectsUnder, "under", "", "only show projects rooted at or under this path")
	projectsCmd.Flags().StringVar(&projectsSort, "sort", "", "sort by: size, modified (default: scan order)")
	projectsCmd.Flags().BoolVar(&projectsAll, "all", false, fmt.Sprintf("show every matching project (default: top %d)", projectsDefaultLimit))
}

// validateProjectsSortFlag rejects an unrecognized --sort value up front
// instead of silently falling back to unsorted order.
func validateProjectsSortFlag(raw string) error {
	switch raw {
	case "", "size", "modified":
		return nil
	default:
		return fmt.Errorf("invalid --sort %q: must be one of: size, modified", raw)
	}
}
