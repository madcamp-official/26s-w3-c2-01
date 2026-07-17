package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// initCmd represents the init command.
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a libra config file and local database",
	Long: `init sets up libra for first use: it writes a config file with the
project roots to scan, paths to exclude, the quarantine directory, and
default risk/stale thresholds, then creates the local SQLite database.

Known dangerous system paths (C:\Windows, C:\Program Files, ...) are
excluded automatically.`,
	Example: `  libra init
  libra init --config .libra.yaml`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(cmd.OutOrStdout(), "init: not yet implemented")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
