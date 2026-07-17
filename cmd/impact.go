package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// impactCmd represents the impact command.
var impactCmd = &cobra.Command{
	Use:   "impact <resource-id-or-path>",
	Short: "Show what breaks if a resource is removed",
	Long: `impact analyzes what happens to affected projects if a resource is
removed: whether already-built executables can still run, whether the
project rebuilds, whether IDE debugging still works, how to restore the
dependency, and any CI configuration that references it.`,
	Example: `  libra impact windows-sdk:10.0.22621.0
  libra impact "C:\Program Files (x86)\Windows Kits\10\Lib\10.0.22621.0"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintf(cmd.OutOrStdout(), "impact %s: not yet implemented\n", args[0])
		return nil
	},
}

func init() {
	rootCmd.AddCommand(impactCmd)
}
