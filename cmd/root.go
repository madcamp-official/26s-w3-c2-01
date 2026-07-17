package cmd

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
	Use:   "libra",
	Short: "Analyze and manage local developer storage",
	Long: `libra analyzes the dependency relationships between local development
projects and the SDKs, tools, caches, and build artifacts on disk. It explains
what is taking up space and what breaks if you delete it.

libra is read-only by default: scan, summary, explain, and impact never
modify the filesystem. Cleanup commands require an explicit action and
default to --dry-run.`,
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
