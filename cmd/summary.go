package cmd

import (
	"fmt"

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
		fmt.Fprintln(cmd.OutOrStdout(), "summary: not yet implemented")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(summaryCmd)

	summaryCmd.Flags().StringVar(&summaryDrive, "drive", "", "limit the summary to this drive (e.g. C:)")
	summaryCmd.Flags().StringVar(&summaryType, "type", "", "limit the summary to this resource type (e.g. sdk)")
}
