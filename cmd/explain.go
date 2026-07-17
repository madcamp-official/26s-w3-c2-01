package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// explainCmd represents the explain command.
var explainCmd = &cobra.Command{
	Use:   "explain <resource-id-or-path>",
	Short: "Explain what a project or resource is and why it exists",
	Long: `explain describes a single project or resource: its kind, path,
size, when it was created or last modified, which projects reference it,
the evidence behind that dependency, whether it can be regenerated, the
expected impact of deleting it, how to recover it, its risk level, and the
confidence of the analysis.`,
	Example: `  libra explain windows-sdk:10.0.22621.0
  libra explain "D:\Projects\OldWeb\node_modules"
  libra explain project:"D:\Projects\GameClient"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintf(cmd.OutOrStdout(), "explain %s: not yet implemented\n", args[0])
		return nil
	},
}

func init() {
	rootCmd.AddCommand(explainCmd)
}
