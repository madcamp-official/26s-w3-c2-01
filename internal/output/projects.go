package output

import (
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

// ProjectsView is the rendered result of `libra projects`: every discovered
// project and its activity state. See F-03/3.4 in
// docs/libra_cli_commands_and_schedule.md.
type ProjectsView struct {
	Projects []ProjectLine `json:"projects"`
}

// ProjectLine is a single project row in a ProjectsView.
type ProjectLine struct {
	Name  string             `json:"name"`
	Path  string             `json:"path"`
	Type  domain.ProjectType `json:"type"`
	Drive string             `json:"drive,omitempty"`
	// LogicalSize is always 0 today (issue #38): no project detector
	// measures it. RenderText deliberately does not render this field --
	// see projectSizeDisplay in format.go. Kept in the JSON shape unchanged
	// pending a team decision on a size_known-style field (a CLI JSON schema
	// change under docs/libra_collaboration_rules.md §9, out of scope here).
	LogicalSize    int64                `json:"logical_size_bytes"`
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
			p.Name, p.Type, p.Drive, projectSizeDisplay,
			p.Status, p.ResourceCount, formatTime(p.LastModifiedAt), p.Path)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, projectSizeFootnote)
	return nil
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
