package output

import (
	"fmt"
	"io"
	"text/tabwriter"

	humanize "github.com/dustin/go-humanize"
)

// SummaryView is the rendered result of `libra summary`: developer storage
// usage broken down by resource type, plus totals by risk level. See F-06 in
// docs/libra_cli_commands_and_schedule.md.
type SummaryView struct {
	Drive           string        `json:"drive,omitempty"`
	ResourcesByType []SummaryLine `json:"resources_by_type"`
	SafeReclaimable int64         `json:"safe_reclaimable_bytes"`
	NeedsReview     int64         `json:"needs_review_bytes"`
	Blocked         int64         `json:"blocked_bytes"`
}

// SummaryLine is a single labeled byte total in a SummaryView.
type SummaryLine struct {
	Label string `json:"label"`
	Bytes int64  `json:"bytes"`
}

// RenderText implements Renderable.
func (s SummaryView) RenderText(w io.Writer) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)

	title := "Developer storage"
	if s.Drive != "" {
		title = fmt.Sprintf("%s drive developer storage", s.Drive)
	}
	fmt.Fprintf(tw, "%s\n\n", title)

	for _, line := range s.ResourcesByType {
		fmt.Fprintf(tw, "%s\t%s\n", line.Label, humanize.Bytes(uint64(line.Bytes)))
	}

	fmt.Fprintf(tw, "\t\n")
	fmt.Fprintf(tw, "Safely reclaimable\t%s\n", humanize.Bytes(uint64(s.SafeReclaimable)))
	fmt.Fprintf(tw, "Needs review\t%s\n", humanize.Bytes(uint64(s.NeedsReview)))
	fmt.Fprintf(tw, "Blocked\t%s\n", humanize.Bytes(uint64(s.Blocked)))

	return tw.Flush()
}
