package cmd

import (
	"errors"
	"fmt"

	"github.com/madcamp-official/26s-w3-c2-01/internal/app"
	"github.com/madcamp-official/26s-w3-c2-01/internal/output"
	"github.com/madcamp-official/26s-w3-c2-01/internal/safety"
	"github.com/madcamp-official/26s-w3-c2-01/internal/store/sqlite"
	"github.com/spf13/cobra"
)

var restoreTransactionID string

var restoreCmd = &cobra.Command{
	Use: "restore --transaction <id>", Short: "Restore quarantined items without overwriting existing paths", Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if restoreTransactionID == "" {
			return errors.New("--transaction is required")
		}
		db, err := openDatabase()
		if err != nil {
			return err
		}
		defer db.Close()
		classifier, err := safety.NewSystemPathClassifier()
		if err != nil {
			return err
		}
		service := app.NewCleanupService(sqlite.NewCleanupPlanRepository(db), sqlite.NewResourceRepository(db), sqlite.NewProjectRepository(db), sqlite.NewCleanupTransactionRepository(db), safety.CleanupValidator{Paths: classifier}, safety.QuarantineEngine{})
		transaction, err := service.Restore(cmd.Context(), restoreTransactionID)
		if err != nil {
			return fmt.Errorf("restore transaction: %w", err)
		}
		view := output.CleanupTransactionViewFromDomain(transaction)
		return output.New(cmd.OutOrStdout(), jsonOutput, "restore").PrintEnvelope(view, view.Envelope())
	},
}

func init() {
	rootCmd.AddCommand(restoreCmd)
	restoreCmd.Flags().StringVar(&restoreTransactionID, "transaction", "", "cleanup transaction ID")
}
