package output

import (
	"fmt"
	"io"
	"strings"

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

// PlanCandidateLine is one SAFE or REVIEW row.
type PlanCandidateLine struct {
	Size int64  `json:"size_bytes"`
	Path string `json:"path"`
}

// PlanBlockedLine is one BLOCKED row. UsedBy names the projects that
// require it, when a dependency edge makes that known.
type PlanBlockedLine struct {
	Size   int64    `json:"size_bytes"`
	Path   string   `json:"path"`
	UsedBy []string `json:"used_by,omitempty"`
}

// RenderText implements Renderable. The numbered list continues across
// SAFE and REVIEW (matching the §3.8 example: SAFE ends at [4], REVIEW
// starts at [5]) since both are candidates the user might act on manually;
// BLOCKED uses an empty "[ ]" marker instead, since it is never selectable.
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

	index := 1
	fmt.Fprintln(w)
	fmt.Fprintln(w, "SAFE")
	if len(v.Safe) == 0 {
		fmt.Fprintln(w, "(none)")
	}
	for _, line := range v.Safe {
		fmt.Fprintf(w, "[%d] %s %s\n", index, humanize.Bytes(uint64(line.Size)), line.Path)
		index++
	}

	if len(v.Review) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "REVIEW")
		for _, line := range v.Review {
			fmt.Fprintf(w, "[%d] %s %s\n", index, humanize.Bytes(uint64(line.Size)), line.Path)
			index++
		}
	}

	if len(v.Blocked) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "BLOCKED")
		for _, line := range v.Blocked {
			fmt.Fprintf(w, "[ ] %s %s\n", humanize.Bytes(uint64(line.Size)), line.Path)
			if len(line.UsedBy) > 0 {
				fmt.Fprintf(w, "    Used by %s\n", strings.Join(line.UsedBy, ", "))
			}
		}
	}

	return nil
}
