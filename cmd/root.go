package cmd

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
	Use:           "libra",
	Short:         "Analyze and manage local developer storage",
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
