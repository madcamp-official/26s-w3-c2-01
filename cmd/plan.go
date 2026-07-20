package cmd

import (
	"fmt"
	"strings"

	humanize "github.com/dustin/go-humanize"

	"github.com/madcamp-official/26s-w3-c2-01/internal/app"
	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/output"
	"github.com/madcamp-official/26s-w3-c2-01/internal/pathutil"
	"github.com/madcamp-official/26s-w3-c2-01/internal/store/sqlite"
	"github.com/spf13/cobra"
)

// planTarget/planRisk/planProject are bound to --target/--risk/--project by
// init() below, the same package-level-flag-variable pattern every other
// cmd/*.go command in this package uses (see cmd/root.go's jsonOutput etc).
var (
	planTarget  string
	planRisk    string
	planProject string
)

// planCmd represents the plan command.
var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Build a cleanup plan that reclaims a target amount of space",
	Long: `plan selects SAFE cleanup candidates -- the reclaimable resources
libra is most confident about -- in order of confidence and then size, until
the requested --target is met (or every SAFE candidate is selected, if
--target is omitted), and saves the selection as a cleanup plan. REVIEW and
BLOCKED candidates are never auto-selected; they are only shown so you can
see what was considered and why it was left out. Run "libra clean --plan
<id>" against the printed plan ID to preview it.`,
	Example: `  libra plan
  libra plan --target 10GB
  libra plan --risk safe
  libra plan --project D:\Projects\OldWeb`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		target, err := parsePlanTarget(planTarget)
		if err != nil {
			return err
		}
		if err := validatePlanRiskFilter(planRisk); err != nil {
			return err
		}
		var projectRoot string
		if planProject != "" {
			normalized, err := pathutil.Normalize(planProject)
			if err != nil {
				return fmt.Errorf("normalize --project %q: %w", planProject, err)
			}
			projectRoot = normalized
		}

		db, err := openDatabase()
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer db.Close()

		resources := sqlite.NewResourceRepository(db)
		projects := sqlite.NewProjectRepository(db)
		scans := sqlite.NewScanRepository(db)
		dependencies := sqlite.NewDependencyRepository(db)

		result, err := app.NewPlanService(resources, projects, scans).Build(cmd.Context(), app.PlanOptions{
			TargetBytes:           target,
			ProjectRootNormalized: projectRoot,
		})
		if err != nil {
			return fmt.Errorf("build plan: %w", err)
		}
		if err := sqlite.NewCleanupPlanRepository(db).Create(cmd.Context(), result.Plan); err != nil {
			return fmt.Errorf("save plan: %w", err)
		}

		view, err := buildPlanView(cmd, result, dependencies, projects)
		if err != nil {
			return err
		}
		return output.New(cmd.OutOrStdout(), jsonOutput).Print(view)
	},
}

func init() {
	rootCmd.AddCommand(planCmd)

	planCmd.Flags().StringVar(&planTarget, "target", "", "amount of space to reclaim, e.g. 10GB (unlimited if omitted)")
	planCmd.Flags().StringVar(&planRisk, "risk", "", "only display this risk tier: safe|review|blocked (selection is always SAFE-only)")
	planCmd.Flags().StringVar(&planProject, "project", "", "restrict candidates to resources under this project path")
}

// parsePlanTarget converts a human size like "10GB" into bytes. An empty
// string means unlimited (0), matching PlanOptions.TargetBytes' zero value.
func parsePlanTarget(raw string) (int64, error) {
	if raw == "" {
		return 0, nil
	}
	parsed, err := humanize.ParseBytes(raw)
	if err != nil {
		return 0, fmt.Errorf("parse --target %q: %w", raw, err)
	}
	return int64(parsed), nil
}

func validatePlanRiskFilter(raw string) error {
	switch strings.ToLower(raw) {
	case "", string(domain.RiskSafe), string(domain.RiskReview), string(domain.RiskBlocked):
		return nil
	default:
		return fmt.Errorf("invalid --risk %q: must be one of safe, review, blocked", raw)
	}
}

// buildPlanView assembles output.PlanView from a PlanResult, applying the
// --risk display filter and resolving BLOCKED "used by" project names from
// the dependency graph (only for BLOCKED, mirroring cmd/resources.go's
// N+1-by-design tradeoff of only paying for a lookup a filter didn't drop).
func buildPlanView(cmd *cobra.Command, result app.PlanResult, dependencies app.DependencyRepository, projects app.ProjectRepository) (output.PlanView, error) {
	view := output.PlanView{
		PlanID:   result.Plan.ID,
		Target:   result.Plan.TargetBytes,
		Selected: result.Plan.SelectedBytes,
		Status:   result.Plan.Status,
	}

	showRisk := func(level domain.RiskLevel) bool {
		return planRisk == "" || strings.EqualFold(planRisk, string(level))
	}

	if showRisk(domain.RiskSafe) {
		for _, item := range result.Plan.Items {
			view.Safe = append(view.Safe, output.PlanCandidateLine{Size: item.ExpectedSize, Path: item.NormalizedPath})
		}
	}
	if showRisk(domain.RiskReview) {
		for _, r := range result.Review {
			// LogicalSize, not ReclaimableSize: ReclaimableSize is forced to
			// 0 for anything that isn't SAFE (see
			// internal/app/resource_service.go's risk switch), so using it
			// here would print "0 B" for every REVIEW candidate regardless
			// of its real size. SAFE lines below correctly use ExpectedSize
			// (== ReclaimableSize == LogicalSize for a SAFE resource).
			view.Review = append(view.Review, output.PlanCandidateLine{Size: r.LogicalSize, Path: r.NormalizedPath})
		}
	}
	if showRisk(domain.RiskBlocked) {
		for _, r := range result.Blocked {
			// Same reasoning as REVIEW above: BLOCKED resources have
			// ReclaimableSize hard-forced to 0, so the real LogicalSize is
			// what a user needs to judge whether it's worth reviewing.
			line := output.PlanBlockedLine{Size: r.LogicalSize, Path: r.NormalizedPath}
			edges, err := dependencies.FindProjectsByResource(cmd.Context(), r.ID)
			if err != nil {
				return output.PlanView{}, fmt.Errorf("find projects depending on %q: %w", r.ID, err)
			}
			for _, edge := range edges {
				project, err := projects.FindByID(cmd.Context(), edge.SourceID)
				if err != nil {
					return output.PlanView{}, fmt.Errorf("find project %q: %w", edge.SourceID, err)
				}
				line.UsedBy = append(line.UsedBy, project.Name)
			}
			view.Blocked = append(view.Blocked, line)
		}
	}

	return view, nil
}
