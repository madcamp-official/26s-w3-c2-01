package output

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	humanize "github.com/dustin/go-humanize"
)

// SummaryView is the rendered result of `libra summary`: developer storage
// usage broken down by resource type, plus totals by risk level. See F-06 in
// docs/libra_cli_commands_and_schedule.md.
type SummaryView struct {
	Drive           string        `json:"drive,omitempty"`
	ProjectCount    int           `json:"project_count"`
	ResourceCount   int           `json:"resource_count"`
	ResourcesByType []SummaryLine `json:"resources_by_type"`
	SafeReclaimable int64         `json:"safe_reclaimable_bytes"`
	NeedsReview     int64         `json:"needs_review_bytes"`
	Blocked         int64         `json:"blocked_bytes"`

	// Scanned is explicit (rather than inferring "no scan yet" from
	// LastScanAt.IsZero()) because encoding/json's omitempty does not omit
	// a zero-value time.Time: without this field, a fresh database with no
	// scan would still marshal last_scan_at as "0001-01-01T00:00:00Z",
	// indistinguishable from a real timestamp to a JSON consumer.
	Scanned       bool      `json:"scanned"`
	LastScanAt    time.Time `json:"last_scan_at,omitempty"`
	LastScanRoots []string  `json:"last_scan_roots,omitempty"`
	// LastScanDurationMS is explicitly milliseconds, not a raw
	// time.Duration -- json.Marshal would otherwise serialize a
	// time.Duration as its integer nanosecond count while a "_ms"-tagged
	// field silently claimed milliseconds, exactly the kind of
	// mismatched-unit JSON field issue #48 flags elsewhere in this project.
	LastScanDurationMS int64 `json:"last_scan_duration_ms,omitempty"`
	FilesInspected     int64 `json:"files_inspected,omitempty"`
	// Coverage is "Complete" or "Partial · N warning(s)" -- a human summary
	// of ScanRecord.ErrorCount, not a machine-parsed field. Empty when
	// Scanned is false.
	Coverage string `json:"coverage,omitempty"`
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

	if s.Scanned {
		fmt.Fprintf(tw, "Last scan\t%s\n", s.LastScanAt.Local().Format("2006-01-02 15:04:05"))
		if len(s.LastScanRoots) > 0 {
			fmt.Fprintf(tw, "Roots\t%s\n", strings.Join(s.LastScanRoots, ", "))
		}
		if s.LastScanDurationMS > 0 {
			fmt.Fprintf(tw, "Duration\t%s\n", time.Duration(s.LastScanDurationMS*int64(time.Millisecond)).Round(time.Millisecond))
		}
		fmt.Fprintf(tw, "Coverage\t%s\n", s.Coverage)
		fmt.Fprintf(tw, "Files inspected\t%s\n", humanize.Comma(s.FilesInspected))
		fmt.Fprintf(tw, "\t\n")
	} else {
		fmt.Fprintf(tw, "Coverage\tNot scanned yet -- run `libra scan` first\n")
		fmt.Fprintf(tw, "\t\n")
	}

	fmt.Fprintf(tw, "Projects\t%d\n", s.ProjectCount)
	fmt.Fprintf(tw, "Resources\t%d\n", s.ResourceCount)
	fmt.Fprintf(tw, "\t\n")

	for _, line := range s.ResourcesByType {
		fmt.Fprintf(tw, "%s\t%s\n", line.Label, humanize.Bytes(uint64(line.Bytes)))
	}

	fmt.Fprintf(tw, "\t\n")
	fmt.Fprintf(tw, "Safely reclaimable\t%s\n", humanize.Bytes(uint64(s.SafeReclaimable)))
	fmt.Fprintf(tw, "Needs review\t%s\n", humanize.Bytes(uint64(s.NeedsReview)))
	fmt.Fprintf(tw, "Blocked\t%s\n", humanize.Bytes(uint64(s.Blocked)))

	return tw.Flush()
}
