package cmd

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/madcamp-official/26s-w3-c2-01/internal/app"
	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/output"
	"github.com/madcamp-official/26s-w3-c2-01/internal/store/sqlite"
	"github.com/spf13/cobra"
)

// summary.go is the one command that goes through an application service
// (app.SummaryService) purely for aggregation, not because summary has
// side effects -- contrast with cmd/projects.go/cmd/resources.go, which
// skip that layer for the same kind of read-only work.
var (
	summaryDrive string
	summaryType  string
)

// resourceTypeLabels maps domain.ResourceType to the display label used in
// `libra summary` (F-06 in docs/libra_cli_commands_and_schedule.md).
var resourceTypeLabels = map[domain.ResourceType]string{
	domain.ResourceTypeWindowsSDK:   "Windows SDKs",
	domain.ResourceTypeNetFXSDK:     ".NET Framework SDKs",
	domain.ResourceTypeVisualStudio: "Visual Studio tools",
	domain.ResourceTypeMSBuild:      "MSBuild",
	domain.ResourceTypeDotNetSDK:    ".NET SDKs",
	domain.ResourceTypeAndroidSDK:   "Android SDK",
	domain.ResourceTypeNodeModules:  "Node project artifacts",
	domain.ResourceTypeBuildOutput:  "Build outputs",
	domain.ResourceTypeGlobalCache:  "Global package caches",
	domain.ResourceTypeDockerCache:  "Docker cache",
	domain.ResourceTypeDockerVolume: "Docker volumes",
}

func resourceTypeLabel(t domain.ResourceType) string {
	if label, ok := resourceTypeLabels[t]; ok {
		return label
	}
	return string(t)
}

// summaryCmd represents the summary command.
var summaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Summarize developer storage usage and reclaimable space",
	Long: `summary reports project and resource counts, storage usage broken
down by resource type and drive, and how much space is safely reclaimable,
needs review, or is blocked from cleanup.`,
	Example: `  libra summary
  libra summary --drive C:
  libra summary --type node-modules`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := openDatabase()
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer db.Close()

		service := app.NewSummaryService(sqlite.NewProjectRepository(db), sqlite.NewResourceRepository(db))
		summary, err := service.Summarize(cmd.Context(), func(resource domain.Resource) bool {
			if summaryType != "" && !strings.EqualFold(string(resource.Type), summaryType) {
				return false
			}
			if summaryDrive != "" && !strings.EqualFold(filepath.VolumeName(resource.DisplayPath), summaryDrive) {
				return false
			}
			return true
		})
		if err != nil {
			return fmt.Errorf("summarize: %w", err)
		}

		view := output.SummaryView{
			Drive:           summaryDrive,
			ProjectCount:    summary.ProjectCount,
			ResourceCount:   summary.ResourceCount,
			SafeReclaimable: summary.SafeReclaimable,
			NeedsReview:     summary.NeedsReview,
			Blocked:         summary.Blocked,
		}

		// Scan freshness (issue #41): omitted, not an error, when no scan has
		// run yet -- ScanRepository.FindLatest's ErrNoScans is exactly the
		// "ran `libra summary` before `libra scan`" case, which the rest of
		// this command already tolerates (an empty Summary, all zeros).
		scan, err := sqlite.NewScanRepository(db).FindLatest(cmd.Context())
		switch {
		case errors.Is(err, app.ErrNoScans):
		case err != nil:
			return fmt.Errorf("find latest scan: %w", err)
		default:
			view.LastScanAt = scan.StartedAt
			view.LastScanRoots = scan.Roots
			view.FilesInspected = scan.FileCount
			if scan.FinishedAt != nil {
				view.LastScanDurationMS = scan.FinishedAt.Sub(scan.StartedAt).Milliseconds()
			}
			switch {
			case scan.FinishedAt == nil:
				// AnalysisOrchestrator.Run saves a Status: RUNNING record
				// before doing any work, then only updates it to a terminal
				// status (with FinishedAt set) on success or failure -- see
				// internal/app/analysis_orchestrator.go's Run/fail. A
				// record still RUNNING here means the process that ran the
				// scan died or was killed mid-scan without either path
				// running, so ErrorCount (still its zero value) says
				// nothing about how much of the scan actually completed --
				// reporting "Complete" would be a lie.
				view.Coverage = "Incomplete · scan did not finish"
			case scan.ErrorCount > 0:
				view.Coverage = fmt.Sprintf("Partial · %d warning(s)", scan.ErrorCount)
			default:
				view.Coverage = "Complete"
			}
		}
		for _, line := range summary.ResourcesByType {
			view.ResourcesByType = append(view.ResourcesByType, output.SummaryLine{
				Label: resourceTypeLabel(line.Type),
				Bytes: line.Bytes,
			})
		}
		return output.New(cmd.OutOrStdout(), jsonOutput).Print(view)
	},
}

func init() {
	rootCmd.AddCommand(summaryCmd)

	summaryCmd.Flags().StringVar(&summaryDrive, "drive", "", "limit the summary to this drive (e.g. C:)")
	summaryCmd.Flags().StringVar(&summaryType, "type", "", "limit the summary to this resource type (e.g. node-modules)")
}
