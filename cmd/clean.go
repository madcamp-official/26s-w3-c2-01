package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	humanize "github.com/dustin/go-humanize"

	"github.com/madcamp-official/26s-w3-c2-01/internal/app"
	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/output"
	"github.com/madcamp-official/26s-w3-c2-01/internal/safety"
	"github.com/madcamp-official/26s-w3-c2-01/internal/store/sqlite"
	"github.com/spf13/cobra"
)

// cleanPlanID/cleanExecute are bound to --plan/--execute by init() below,
// the same package-level-flag-variable pattern every other cmd/*.go
// command in this package uses (see cmd/root.go's jsonOutput etc).
var (
	cleanPlanID  string
	cleanExecute bool
)

// cleanCmd represents the clean command.
var cleanCmd = &cobra.Command{
	Use:   "clean --plan <id>",
	Short: "Preview a saved cleanup plan",
	Long: `clean reads a plan created by "libra plan" and prints a dry-run
preview of what each SAFE item in it would do -- it never touches the
filesystem. Each item is re-checked against the resource's current state,
so drift since planning (a changed size or risk level, or a resource that
no longer exists) is reported instead of silently ignored.

Use --execute to move verified directories to a same-volume quarantine.
Without --yes, execution asks for interactive confirmation.`,
	Example: `  libra clean --plan plan-20260717-001
  libra clean --plan plan-20260717-001 --execute --yes`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if cleanPlanID == "" {
			return errors.New("--plan is required")
		}
		db, err := openDatabase()
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer db.Close()

		plans := sqlite.NewCleanupPlanRepository(db)
		plan, err := plans.FindByID(cmd.Context(), cleanPlanID)
		if err != nil {
			return fmt.Errorf("find plan %q: %w", cleanPlanID, err)
		}

		resources := sqlite.NewResourceRepository(db)
		if cleanExecute {
			if !assumeYes {
				fmt.Fprint(cmd.ErrOrStderr(), "Move verified plan items to quarantine? [y/N] ")
				answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
				if strings.ToLower(strings.TrimSpace(answer)) != "y" {
					return errors.New("cleanup cancelled")
				}
			}
			classifier, err := safety.NewSystemPathClassifier()
			if err != nil {
				return err
			}
			service := app.NewCleanupService(plans, resources, sqlite.NewProjectRepository(db), sqlite.NewCleanupTransactionRepository(db), safety.CleanupValidator{Paths: classifier}, safety.QuarantineEngine{})
			transaction, err := service.Execute(cmd.Context(), cleanPlanID)
			if err != nil {
				return fmt.Errorf("execute cleanup: %w", err)
			}
			transactionView := output.CleanupTransactionViewFromDomain(transaction)
			return output.New(cmd.OutOrStdout(), jsonOutput, "clean").PrintEnvelope(transactionView, transactionView.Envelope())
		}
		view := output.CleanView{PlanID: plan.ID, DryRun: true}
		for _, item := range plan.Items {
			line, err := previewCleanItem(cmd, resources, item)
			if err != nil {
				return err
			}
			view.Items = append(view.Items, line)
		}

		return output.New(cmd.OutOrStdout(), jsonOutput, "clean").PrintEnvelope(view, view.Envelope())
	},
}

func init() {
	rootCmd.AddCommand(cleanCmd)

	cleanCmd.Flags().StringVar(&cleanPlanID, "plan", "", "ID of a plan created by \"libra plan\" (required)")
	cleanCmd.Flags().BoolVar(&cleanExecute, "execute", false, "move revalidated SAFE items into quarantine")
}

// previewCleanItem re-checks one plan item's snapshot against the
// resource's current state as of the last scan, without modifying
// anything on disk or in the database.
func previewCleanItem(cmd *cobra.Command, resources app.ResourceRepository, item domain.CleanupPlanItem) (output.CleanItemLine, error) {
	line := output.CleanItemLine{Path: item.NormalizedPath, ExpectedSize: item.ExpectedSize}

	current, err := resources.FindByID(cmd.Context(), item.ResourceID)
	if errors.Is(err, sqlite.ErrResourceNotFound) {
		line.Status = output.CleanItemMissing
		line.Detail = "resource no longer found as of the last scan"
		return line, nil
	}
	if err != nil {
		return output.CleanItemLine{}, fmt.Errorf("find resource %q: %w", item.ResourceID, err)
	}
	if current.Risk != item.RiskAtPlanning {
		line.Status = output.CleanItemChanged
		line.Detail = fmt.Sprintf("risk changed from %s to %s since planning", item.RiskAtPlanning, current.Risk)
		return line, nil
	}
	if current.ReclaimableSize != item.ExpectedSize {
		line.Status = output.CleanItemChanged
		line.Detail = fmt.Sprintf("size changed from %s to %s since planning",
			humanize.Bytes(uint64(item.ExpectedSize)), humanize.Bytes(uint64(current.ReclaimableSize)))
		return line, nil
	}
	line.Status = output.CleanItemWouldMove
	return line, nil
}
