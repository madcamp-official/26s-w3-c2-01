package cmd

import (
	"github.com/madcamp-official/26s-w3-c2-01/internal/output"
	"github.com/madcamp-official/26s-w3-c2-01/internal/store/sqlite"
	"github.com/spf13/cobra"
)

var transactionsCmd = &cobra.Command{
	Use: "transactions", Short: "List cleanup and restore transactions", Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		db, err := openDatabase()
		if err != nil {
			return err
		}
		defer db.Close()
		transactions, err := sqlite.NewCleanupTransactionRepository(db).List(cmd.Context())
		if err != nil {
			return err
		}
		view := output.CleanupTransactionsView{}
		for _, transaction := range transactions {
			view.Transactions = append(view.Transactions, output.CleanupTransactionViewFromDomain(transaction))
		}
		return output.New(cmd.OutOrStdout(), jsonOutput, "transactions").Print(view)
	},
}

func init() { rootCmd.AddCommand(transactionsCmd) }
