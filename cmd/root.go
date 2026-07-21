package cmd

import "github.com/spf13/cobra"

// root.go wires the Cobra command tree's entrypoint; every other cmd/*.go
// file registers itself onto rootCmd from its own init(). Execute() is the
// only symbol main.go calls into this package.
//
// Global flag values shared by every subcommand. Populated by rootCmd's
// persistent flags; subcommands read these instead of redefining them.
var (
	cfgPath    string
	jsonOutput bool
	verbose    bool
	noColor    bool
	// assumeYes backs --yes, registered locally by clean.go and purge.go
	// (the only commands that prompt for confirmation) rather than here.
	assumeYes bool
)

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
	resultExitCode = ExitSuccess
	return rootCmd.Execute()
}

func init() {
	flags := rootCmd.PersistentFlags()
	flags.StringVar(&cfgPath, "config", "", "path to libra config file (default: .libra.yaml)")
	flags.BoolVar(&jsonOutput, "json", false, "output machine-readable JSON instead of text")
	flags.BoolVar(&verbose, "verbose", false, "print additional diagnostic detail")
	flags.BoolVar(&noColor, "no-color", false, "disable ANSI color in text output")
}
