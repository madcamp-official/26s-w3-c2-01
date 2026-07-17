package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	scanRoot string
	scanFull bool
)

// scanCmd represents the scan command.
var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Discover projects, resources, and build artifacts",
	Long: `scan walks the configured project roots, detects projects
(.sln, .vcxproj, .csproj, package.json, .git), detects known development
resources and build artifacts, computes their logical size, runs dependency
analysis, and stores the results in the local SQLite database.

Permission errors on individual paths are recorded but do not abort the
scan.`,
	Example: `  libra scan
  libra scan --root D:\Projects
  libra scan --full`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(cmd.OutOrStdout(), "scan: not yet implemented")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(scanCmd)

	scanCmd.Flags().StringVar(&scanRoot, "root", "", "scan only this project root instead of all configured roots")
	scanCmd.Flags().BoolVar(&scanFull, "full", false, "force a full rescan instead of an incremental one")
}
