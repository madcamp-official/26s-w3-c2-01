package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/madcamp-official/26s-w3-c2-01/internal/config"
	"github.com/madcamp-official/26s-w3-c2-01/internal/output"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{Use: "config", Short: "Inspect and update configuration"}
var configShowCmd = &cobra.Command{Use: "show", Short: "Show the effective configuration", Args: cobra.NoArgs, RunE: showConfig}
var configValidateCmd = &cobra.Command{Use: "validate", Short: "Validate the configuration file", Args: cobra.NoArgs, RunE: validateConfig}
var configSetCmd = &cobra.Command{Use: "set <key> <value>", Short: "Update a supported configuration value", Args: cobra.ExactArgs(2), RunE: setConfig}

// configSetDelete backs `config set exclude <name> -d`, which removes a
// single entry from a list-valued key instead of replacing the whole list --
// see cmd/root.go's jsonOutput for the same package-level flag-var pattern.
var configSetDelete bool

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configShowCmd, configValidateCmd, configSetCmd)
	configSetCmd.Flags().BoolVarP(&configSetDelete, "delete", "d", false, "remove <value> from the exclude list instead of replacing it (safety excludes cannot be removed)")
}

func showConfig(cmd *cobra.Command, _ []string) error {
	cfg, err := config.Load(configFilePath())
	if err != nil {
		return err
	}
	return output.New(cmd.OutOrStdout(), jsonOutput, "config show").Print(output.ConfigView{Path: configFilePath(), Config: cfg})
}

func validateConfig(cmd *cobra.Command, _ []string) error {
	if _, err := config.Load(configFilePath()); err != nil {
		return err
	}
	return output.New(cmd.OutOrStdout(), jsonOutput, "config validate").Print(output.ConfigValidationView{Path: configFilePath(), Valid: true})
}

func setConfig(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(configFilePath())
	if err != nil {
		return err
	}
	key, value := strings.ToLower(args[0]), args[1]
	if configSetDelete {
		if key != "exclude" {
			return fmt.Errorf("--delete is only supported for the exclude key")
		}
		updated, err := config.RemoveExclude(cfg.Exclude, value)
		if err != nil {
			return err
		}
		cfg.Exclude = updated
		if err := config.Save(configFilePath(), cfg); err != nil {
			return err
		}
		return output.New(cmd.OutOrStdout(), jsonOutput, "config set").Print(output.ConfigUpdateView{Path: configFilePath(), Key: key, Value: value})
	}
	positiveInt := func() (int, error) {
		n, err := strconv.Atoi(value)
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("%s must be a positive integer", key)
		}
		return n, nil
	}
	switch key {
	case "project_roots":
		cfg.ProjectRoots = splitConfigList(value)
	case "exclude":
		cfg.Exclude = config.EnsureSafetyExcludes(splitConfigList(value))
	case "scan.max_depth":
		cfg.Scan.MaxDepth, err = positiveInt()
	case "scan.stale_days":
		cfg.Scan.StaleDays, err = positiveInt()
	case "scan.follow_reparse_points":
		cfg.Scan.FollowReparsePoints, err = strconv.ParseBool(value)
	case "cleanup.quarantine_days":
		cfg.Cleanup.QuarantineDays, err = positiveInt()
	default:
		return fmt.Errorf("unsupported config key %q", key)
	}
	if err != nil {
		return fmt.Errorf("set %s: %w", key, err)
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	if err := config.Save(configFilePath(), cfg); err != nil {
		return err
	}
	return output.New(cmd.OutOrStdout(), jsonOutput, "config set").Print(output.ConfigUpdateView{Path: configFilePath(), Key: key, Value: value})
}

func splitConfigList(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if item := strings.TrimSpace(part); item != "" {
			out = append(out, item)
		}
	}
	return out
}
