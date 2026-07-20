package output

import (
	"fmt"
	"io"

	humanize "github.com/dustin/go-humanize"
)

// CleanItemStatus classifies one plan item as `libra clean` compares it
// against the resource's current state before ever touching a file.
type CleanItemStatus string

const (
	// CleanItemWouldMove means the resource still matches the database
	// snapshot in dry-run; --execute performs the stronger filesystem checks.
	CleanItemWouldMove CleanItemStatus = "WOULD_MOVE"
	// CleanItemChanged means the resource still exists but its size or
	// risk has drifted since planning, so it would be skipped rather
	// than moved on stale information.
	CleanItemChanged CleanItemStatus = "CHANGED"
	// CleanItemMissing means the resource the plan snapshot points at is
	// no longer known at all (e.g. a re-scan no longer found it).
	CleanItemMissing CleanItemStatus = "MISSING"
)

// CleanView is the dry-run result of `libra clean --plan <id>`.
type CleanView struct {
	PlanID string          `json:"plan_id"`
	DryRun bool            `json:"dry_run"`
	Items  []CleanItemLine `json:"items"`
}

// CleanItemLine is one previewed plan item.
type CleanItemLine struct {
	Path         string          `json:"path"`
	ExpectedSize int64           `json:"expected_size_bytes"`
	Status       CleanItemStatus `json:"status"`
	Detail       string          `json:"detail,omitempty"`
}

// RenderText implements Renderable.
func (v CleanView) RenderText(w io.Writer) error {
	fmt.Fprintf(w, "Plan ID: %s\n", v.PlanID)
	fmt.Fprintln(w, "Mode: dry-run (no files will be moved)")
	fmt.Fprintln(w)

	if len(v.Items) == 0 {
		fmt.Fprintln(w, "No SAFE items in this plan.")
		return nil
	}

	for _, item := range v.Items {
		fmt.Fprintf(w, "[%s] %s %s\n", item.Status, humanize.Bytes(uint64(item.ExpectedSize)), item.Path)
		if item.Detail != "" {
			fmt.Fprintf(w, "    %s\n", item.Detail)
		}
	}
	return nil
}
