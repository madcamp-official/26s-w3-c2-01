package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/madcamp-official/26s-w3-c2-01/internal/app"
	"github.com/madcamp-official/26s-w3-c2-01/internal/config"
	"github.com/madcamp-official/26s-w3-c2-01/internal/output"
	"github.com/madcamp-official/26s-w3-c2-01/internal/store/sqlite"
	"github.com/spf13/cobra"
)

var purgeTransactionID string
var purgeExecute bool

var purgeCmd = &cobra.Command{
	Use: "purge --transaction <id>", Short: "Permanently remove expired quarantine items", Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if purgeTransactionID == "" {
			return errors.New("--transaction is required")
		}
		cfg := config.Default()
		if _, err := os.Stat(configFilePath()); err == nil {
			cfg, err = config.Load(configFilePath())
			if err != nil {
				return err
			}
		}
		if purgeExecute && !assumeYes {
			fmt.Fprint(cmd.ErrOrStderr(), "Permanently delete quarantined items? This cannot be restored. [y/N] ")
			answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
			if strings.ToLower(strings.TrimSpace(answer)) != "y" {
				return errors.New("purge cancelled")
			}
		}
		db, err := openDatabase()
		if err != nil {
			return err
		}
		defer db.Close()
		result, err := app.NewPurgeService(sqlite.NewCleanupTransactionRepository(db)).Purge(cmd.Context(), purgeTransactionID, cfg.Cleanup.QuarantineDays, purgeExecute)
		if err != nil {
			return fmt.Errorf("purge transaction: %w", err)
		}
		view := output.PurgeView{TransactionID: result.Transaction.ID, DryRun: result.DryRun, Status: result.Transaction.Status, Candidates: result.Candidates}
		return output.New(cmd.OutOrStdout(), jsonOutput, "purge").PrintEnvelope(view, view.Envelope())
	},
}

func init() {
	rootCmd.AddCommand(purgeCmd)
	purgeCmd.Flags().StringVar(&purgeTransactionID, "transaction", "", "quarantined transaction ID")
	purgeCmd.Flags().BoolVar(&purgeExecute, "execute", false, "permanently delete validated quarantine items")
}
