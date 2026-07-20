package cmd

import (
	"errors"
	"fmt"

	humanize "github.com/dustin/go-humanize"

	"github.com/madcamp-official/26s-w3-c2-01/internal/app"
	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/output"
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

--execute (actually moving files into quarantine) is not implemented yet:
it depends on transaction/quarantine storage that is still in progress.
See the Day 5 status note in docs/libra_cli_commands_and_schedule.md.`,
	Example: `  libra clean --plan plan-20260717-001`,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if cleanPlanID == "" {
			return errors.New("--plan is required")
		}
		if cleanExecute {
			return errors.New(`clean --execute is not implemented yet: quarantine and transaction storage are still in progress (see the Day 5 status note in docs/libra_cli_commands_and_schedule.md); run "libra clean --plan <id>" without --execute for a dry-run preview`)
		}

		db, err := openDatabase()
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer db.Close()

		plan, err := sqlite.NewCleanupPlanRepository(db).FindByID(cmd.Context(), cleanPlanID)
		if err != nil {
			return fmt.Errorf("find plan %q: %w", cleanPlanID, err)
		}

		resources := sqlite.NewResourceRepository(db)
		view := output.CleanView{PlanID: plan.ID, DryRun: true}
		for _, item := range plan.Items {
			line, err := previewCleanItem(cmd, resources, item)
			if err != nil {
				return err
			}
			view.Items = append(view.Items, line)
		}

		return output.New(cmd.OutOrStdout(), jsonOutput).Print(view)
	},
}

func init() {
	rootCmd.AddCommand(cleanCmd)

	cleanCmd.Flags().StringVar(&cleanPlanID, "plan", "", "ID of a plan created by \"libra plan\" (required)")
	cleanCmd.Flags().BoolVar(&cleanExecute, "execute", false, "not implemented yet: would actually quarantine SAFE items")
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
