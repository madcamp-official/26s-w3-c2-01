package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/madcamp-official/26s-w3-c2-01/internal/config"
	"github.com/spf13/cobra"
)

// init.go is the only command that creates .libra.yaml when it's missing;
// every other command just assumes a config exists (or silently falls back
// to config.Default(), e.g. cmd/scan.go's resolveScanOptions) rather than
// requiring `libra init` to have run first.
//
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
		path := configFilePath()
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			if err := config.Save(path, config.Default()); err != nil {
				return fmt.Errorf("write config: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created config file: %s\n", path)
		} else if err != nil {
			return fmt.Errorf("check config %q: %w", path, err)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Config file already exists: %s\n", path)
		}

		db, err := openDatabase()
		if err != nil {
			return fmt.Errorf("initialize database: %w", err)
		}
		defer db.Close()

		fmt.Fprintf(cmd.OutOrStdout(), "Database ready: %s\n", dbFilePath())
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
