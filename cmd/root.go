package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

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

// Version is the libra release version, stamped at build time via
// -ldflags "-X github.com/madcamp-official/26s-w3-c2-01/cmd.Version=<version>"
// (see scripts/windows/build-installer.ps1 and scripts/macos/build.sh).
// Unstamped builds (`go run .`, plain `go build`) report "dev".
var Version = "dev"

var rootCmd = &cobra.Command{
	Use:     "libra",
	Short:   "Analyze and manage local developer storage",
	Version: Version,
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

// requireInit gates every command except `init` (and cobra's own bare-root
// help/completion machinery) behind an existing config file, so a project
// that hasn't run `libra init` yet gets a guidance message instead of a
// command silently falling back to config.Default() or failing deep inside
// database code.
func requireInit(cmd *cobra.Command, args []string) error {
	switch topLevelCommand(cmd) {
	case rootCmd, initCmd, helpCmd(cmd), completionCmd(cmd):
		return nil
	}
	path := configFilePath()
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("libra is not initialized here\nrun %q first (looked for %s)", "libra init", path)
	} else if err != nil {
		return fmt.Errorf("check config %q: %w", path, err)
	}
	return nil
}

// topLevelCommand walks up from cmd to the direct child of rootCmd it
// descends from (or rootCmd itself when cmd is the root), since
// PersistentPreRunE only runs once for the invoked command, not once per
// level of a nested command like `daemon start` or `config show`.
func topLevelCommand(cmd *cobra.Command) *cobra.Command {
	for cmd.Parent() != nil && cmd.Parent() != rootCmd {
		cmd = cmd.Parent()
	}
	return cmd
}

// helpCmd and completionCmd return the auto-generated cobra commands cobra
// attaches to the root (`libra help`, `libra completion ...`) so requireInit
// can exempt them by identity rather than by matching on name.
func helpCmd(cmd *cobra.Command) *cobra.Command {
	for _, c := range rootCmd.Commands() {
		if c.Name() == "help" {
			return c
		}
	}
	return nil
}

func completionCmd(cmd *cobra.Command) *cobra.Command {
	for _, c := range rootCmd.Commands() {
		if c.Name() == "completion" {
			return c
		}
	}
	return nil
}

// Execute runs the root command.
func Execute() error {
	resultExitCode = ExitSuccess
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentPreRunE = requireInit

	flags := rootCmd.PersistentFlags()
	flags.StringVar(&cfgPath, "config", "", "path to libra config file (default: .libra.yaml)")
	flags.BoolVar(&jsonOutput, "json", false, "output machine-readable JSON instead of text")
	flags.BoolVar(&verbose, "verbose", false, "print additional diagnostic detail")
	flags.BoolVar(&noColor, "no-color", false, "disable ANSI color in text output")
}
