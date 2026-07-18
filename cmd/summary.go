package cmd

import (
	"github.com/madcamp-official/26s-w3-c2-01/internal/output"
	"github.com/spf13/cobra"
)

var (
	summaryDrive string
	summaryType  string
)

// summaryCmd represents the summary command.
var summaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Summarize developer storage usage and reclaimable space",
	Long: `summary reports project and resource counts, storage usage broken
down by resource type and drive, and how much space is safely reclaimable,
needs review, or is blocked from cleanup.`,
	Example: `  libra summary
  libra summary --drive C:
  libra summary --type sdk`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Mock numbers until scan/store wiring lands (Day2); shape matches
		// the F-06 example in docs/libra_cli_commands_and_schedule.md.
		view := output.SummaryView{
			Drive: summaryDrive,
			ResourcesByType: []output.SummaryLine{
				{Label: "Windows SDKs", Bytes: 12459999232},
				{Label: "Visual Studio tools", Bytes: 25986469478},
				{Label: ".NET SDKs", Bytes: 5798727680},
				{Label: "Node project artifacts", Bytes: 19434323968},
				{Label: "MSBuild outputs", Bytes: 8162838528},
			},
			SafeReclaimable: 10416967680,
			NeedsReview:     13316730880,
			Blocked:         63146360832,
		}
		return output.New(cmd.OutOrStdout(), jsonOutput).Print(view)
	},
}

func init() {
	rootCmd.AddCommand(summaryCmd)

	summaryCmd.Flags().StringVar(&summaryDrive, "drive", "", "limit the summary to this drive (e.g. C:)")
	summaryCmd.Flags().StringVar(&summaryType, "type", "", "limit the summary to this resource type (e.g. sdk)")
}
