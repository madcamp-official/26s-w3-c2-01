package output

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	humanize "github.com/dustin/go-humanize"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

// PlanView is the rendered result of `libra plan`: the SAFE candidates
// selected into the stored plan, plus the REVIEW/BLOCKED candidates that
// were considered but never auto-selected, shown so the user can see why.
// See §3.8 in docs/libra_cli_commands_and_schedule.md.
type PlanView struct {
	PlanID   string                   `json:"plan_id"`
	Target   int64                    `json:"target_bytes"`
	Selected int64                    `json:"selected_bytes"`
	Status   domain.CleanupPlanStatus `json:"status"`
	Safe     []PlanCandidateLine      `json:"safe"`
	Review   []PlanCandidateLine      `json:"review,omitempty"`
	Blocked  []PlanBlockedLine        `json:"blocked,omitempty"`
}

// PlanCandidateLine is one SAFE or REVIEW row. RiskReasons explain why RiskPolicy
// classified it at this risk level (issue #40) --
// without it, `plan` was just a list of paths with no indication of why a
// SAFE candidate was trusted or a REVIEW one wasn't.
type PlanCandidateLine struct {
	Size        int64               `json:"size_bytes"`
	Path        string              `json:"path"`
	RiskReasons []domain.RiskReason `json:"risk_reasons,omitempty"`
}

// PlanBlockedLine is one BLOCKED row. UsedBy names the projects that
// require it, when a dependency edge makes that known.
type PlanBlockedLine struct {
	Size        int64               `json:"size_bytes"`
	Path        string              `json:"path"`
	RiskReasons []domain.RiskReason `json:"risk_reasons,omitempty"`
	UsedBy      []string            `json:"used_by,omitempty"`
}

// Envelope maps PlanView onto the shared JSON envelope (issue #59):
// INSUFFICIENT_CANDIDATES means plan couldn't reach --target, which is
// exactly the "completed but fell short" case Outcome exists to flag.
// Issues deliberately aren't wired here: SAFE/REVIEW/BLOCKED risk reasons
// are decision evidence about candidates, not failures in the plan command's
// execution. Duplicating them as envelope issues would blur that distinction.
func (v PlanView) Envelope() EnvelopeOptions {
	if v.Status == domain.CleanupPlanInsufficientCandidates {
		return EnvelopeOptions{Outcome: OutcomePartial}
	}
	return EnvelopeOptions{Outcome: OutcomeSuccess}
}

// RenderText implements Renderable. Each tier is grouped by its shared risk
// reason rather than printed as one flat numbered list: a reason repeated
// across many candidates (e.g. 14 REVIEW items all "not fully verified")
// prints once with a subtotal instead of once per line, and both groups and
// the lines within them are ordered by size descending so the largest
// reclaim opportunity is never buried in the middle of the list. The old
// per-line "[N]"/"[ ]" markers are gone: nothing in this codebase lets a
// user act on an individual plan line (clean --plan operates on the whole
// plan), so numbering only implied a selection that doesn't exist.
func (v PlanView) RenderText(w io.Writer) error {
	fmt.Fprintf(w, "Plan ID: %s\n", v.PlanID)
	if v.Target > 0 {
		fmt.Fprintf(w, "Target: %s\n", humanize.Bytes(uint64(v.Target)))
	} else {
		fmt.Fprintln(w, "Target: unlimited")
	}
	fmt.Fprintf(w, "Selected: %s\n", humanize.Bytes(uint64(v.Selected)))
	if v.Status == domain.CleanupPlanInsufficientCandidates {
		fmt.Fprintln(w, "Status: INSUFFICIENT_CANDIDATES (not enough SAFE candidates to reach target)")
	}

	fmt.Fprintln(w)
	renderPlanSummary(w, v)

	fmt.Fprintln(w)
	fmt.Fprintln(w, "SAFE")
	if len(v.Safe) == 0 {
		fmt.Fprintln(w, "(none)")
	} else {
		renderCandidateLines(w, v.Safe)
	}

	if len(v.Review) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "REVIEW")
		renderCandidateLines(w, v.Review)
	}

	if len(v.Blocked) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "BLOCKED")
		renderBlockedLines(w, v.Blocked)
	}

	return nil
}

// renderPlanSummary prints one line per tier RenderText will actually show
// below, so a reader can gauge scale (how much is safe to ignore vs. how
// much needs a manual look) without scrolling through every line first. A
// tier RenderText omits entirely -- REVIEW/BLOCKED filtered down to nothing
// by --risk -- is left out here too, rather than printing a "0 items" that
// would look like a real finding instead of a filter side effect.
func renderPlanSummary(w io.Writer, v PlanView) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintf(tw, "SAFE\t%s\t%s\tauto-selected\n", itemCount(len(v.Safe)), humanize.Bytes(uint64(sumCandidateSize(v.Safe))))
	if len(v.Review) > 0 {
		fmt.Fprintf(tw, "REVIEW\t%s\t%s\tneeds your review\n", itemCount(len(v.Review)), humanize.Bytes(uint64(sumCandidateSize(v.Review))))
	}
	if len(v.Blocked) > 0 {
		fmt.Fprintf(tw, "BLOCKED\t%s\t%s\tnot eligible\n", itemCount(len(v.Blocked)), humanize.Bytes(uint64(sumBlockedSize(v.Blocked))))
	}
	tw.Flush()
}

func itemCount(n int) string {
	if n == 1 {
		return "1 item"
	}
	return fmt.Sprintf("%d items", n)
}

func sumCandidateSize(lines []PlanCandidateLine) int64 {
	var total int64
	for _, line := range lines {
		total += line.Size
	}
	return total
}

func sumBlockedSize(lines []PlanBlockedLine) int64 {
	var total int64
	for _, line := range lines {
		total += line.Size
	}
	return total
}

// reasonGroup buckets same-typed lines that share the exact same risk
// reason text, in the order that reason first appeared in the input.
type reasonGroup[T any] struct {
	reason string
	lines  []T
	total  int64
}

// groupBySharedReason groups lines sharing identical reason text together,
// largest group first by total size, with each group's own lines sorted
// largest-first. reasonOf/sizeOf let this one implementation serve both
// PlanCandidateLine and PlanBlockedLine without either type needing methods
// solely for this grouping.
func groupBySharedReason[T any](lines []T, reasonOf func(T) string, sizeOf func(T) int64) []reasonGroup[T] {
	index := make(map[string]int, len(lines))
	var groups []reasonGroup[T]
	for _, line := range lines {
		key := reasonOf(line)
		i, ok := index[key]
		if !ok {
			i = len(groups)
			index[key] = i
			groups = append(groups, reasonGroup[T]{reason: key})
		}
		groups[i].lines = append(groups[i].lines, line)
		groups[i].total += sizeOf(line)
	}
	for i := range groups {
		lines := groups[i].lines
		sort.SliceStable(lines, func(a, b int) bool { return sizeOf(lines[a]) > sizeOf(lines[b]) })
	}
	sort.SliceStable(groups, func(a, b int) bool { return groups[a].total > groups[b].total })
	return groups
}

// renderCandidateLines prints SAFE/REVIEW lines grouped by reason; see
// RenderText's doc comment for why.
func renderCandidateLines(w io.Writer, lines []PlanCandidateLine) {
	groups := groupBySharedReason(lines,
		func(l PlanCandidateLine) string { return riskReasonMessages(l.RiskReasons) },
		func(l PlanCandidateLine) int64 { return l.Size },
	)
	for i, group := range groups {
		if i > 0 {
			fmt.Fprintln(w)
		}
		if group.reason != "" {
			fmt.Fprintf(w, "Reason: %s (%s, %s)\n", group.reason, itemCount(len(group.lines)), humanize.Bytes(uint64(group.total)))
		}
		for _, line := range group.lines {
			fmt.Fprintf(w, "- %s  %s\n", humanize.Bytes(uint64(line.Size)), line.Path)
		}
	}
}

// renderBlockedLines is renderCandidateLines' BLOCKED counterpart -- the
// same reason-grouping, plus an indented "Used by" line per item when the
// dependency graph names the project requiring it.
func renderBlockedLines(w io.Writer, lines []PlanBlockedLine) {
	groups := groupBySharedReason(lines,
		func(l PlanBlockedLine) string { return riskReasonMessages(l.RiskReasons) },
		func(l PlanBlockedLine) int64 { return l.Size },
	)
	for i, group := range groups {
		if i > 0 {
			fmt.Fprintln(w)
		}
		if group.reason != "" {
			fmt.Fprintf(w, "Reason: %s (%s, %s)\n", group.reason, itemCount(len(group.lines)), humanize.Bytes(uint64(group.total)))
		}
		for _, line := range group.lines {
			fmt.Fprintf(w, "- %s  %s\n", humanize.Bytes(uint64(line.Size)), line.Path)
			if len(line.UsedBy) > 0 {
				fmt.Fprintf(w, "    Used by %s\n", strings.Join(line.UsedBy, ", "))
			}
		}
	}
}
