package output

import (
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	humanize "github.com/dustin/go-humanize"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

// ProjectsView is the rendered result of `libra projects`: every discovered
// project and its activity state. See F-03/3.4 in
// docs/libra_cli_commands_and_schedule.md.
type ProjectsView struct {
	Projects []ProjectLine `json:"projects"`
	// TotalCount is how many projects matched the caller's filters before
	// the default display limit (issue #41) truncated Projects. Equal to
	// len(Projects) when --all was passed or nothing was truncated.
	TotalCount int `json:"total_count"`
}

// ProjectLine is a single project row in a ProjectsView. SizeKnown
// disambiguates LogicalSize's zero value (issue #48): no project detector
// sets LogicalSize until AnalysisOrchestrator.Run measures it, and that
// measurement can itself fail (permission error, etc.), so a bare
// "logical_size_bytes": 0 is ambiguous between "empty" and "unmeasured" for
// any JSON consumer -- mirrors domain.Resource.SizeKnown's existing role.
type ProjectLine struct {
	Name           string               `json:"name"`
	Path           string               `json:"path"`
	Type           domain.ProjectType   `json:"type"`
	Drive          string               `json:"drive,omitempty"`
	LogicalSize    int64                `json:"logical_size_bytes"`
	SizeKnown      bool                 `json:"size_known"`
	LastModifiedAt time.Time            `json:"last_modified_at,omitempty"`
	LastObservedAt time.Time            `json:"last_observed_at"`
	Status         domain.ProjectStatus `json:"status"`
	ResourceCount  int                  `json:"resource_count"`
}

// RenderText implements Renderable.
func (v ProjectsView) RenderText(w io.Writer) error {
	if len(v.Projects) == 0 {
		fmt.Fprintln(w, "No projects found. Run `libra scan` first.")
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tTYPE\tDRIVE\tSIZE\tSTATUS\tRESOURCES\tMODIFIED\tPATH")
	for _, p := range v.Projects {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%d\t%s\t%s\n",
			p.Name, p.Type, p.Drive, formatProjectSize(p.LogicalSize, &p.SizeKnown),
			p.Status, p.ResourceCount, formatTime(p.LastModifiedAt), p.Path)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	// Only note the cap when it actually hid something (issue #41) -- e.g.
	// "Showing 3 of 3 projects" on an unfiltered small scan would just be
	// noise, not information.
	if v.TotalCount > len(v.Projects) {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "Showing %d of %d projects. Use --all to see the rest.\n", len(v.Projects), v.TotalCount)
	}
	return nil
}

// formatProjectSize renders a project's LogicalSize as "—" when it is known
// to be unmeasured (sizeKnown is false), instead of the misleading "0 B" a
// bare humanize.Bytes(0) would print (issue #48). sizeKnown is a *bool,
// not bool, because ExplainView.SizeKnown is nil for a resource-kind view
// (where this ambiguity doesn't apply) -- treated the same as "known" here
// since callers only pass a nil pointer for non-project views that never
// reach this function in practice.
func formatProjectSize(logicalSize int64, sizeKnown *bool) string {
	if sizeKnown != nil && !*sizeKnown {
		return "—"
	}
	return humanize.Bytes(uint64(logicalSize))
}

// formatTime is shared by every view in this package that renders a
// timestamp (ProjectLine here, plus explain/impact views), not just
// ProjectsView -- it lives in this file because ProjectsView was the first
// caller, not because it's projects-specific.
func formatTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format("2006-01-02")
}
