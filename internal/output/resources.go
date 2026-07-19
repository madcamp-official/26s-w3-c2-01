package output

import (
	"fmt"
	"io"
	"text/tabwriter"

	humanize "github.com/dustin/go-humanize"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

// ResourcesView is the rendered result of `libra resources`: every
// discovered SDK, tool, cache, or build artifact. See F-04/3.5 in
// docs/libra_cli_commands_and_schedule.md.
type ResourcesView struct {
	Resources []ResourceLine `json:"resources"`
}

// ResourceLine is a single resource row in a ResourcesView.
type ResourceLine struct {
	Name         string              `json:"name"`
	Type         domain.ResourceType `json:"type"`
	Version      string              `json:"version,omitempty"`
	Path         string              `json:"path"`
	LogicalSize  int64               `json:"logical_size_bytes"`
	ProjectCount int                 `json:"project_count"`
	Regenerable  bool                `json:"regenerable"`
	Risk         domain.RiskLevel    `json:"risk"`
	Confidence   int                 `json:"confidence"`
}

// RenderText implements Renderable.
func (v ResourcesView) RenderText(w io.Writer) error {
	if len(v.Resources) == 0 {
		fmt.Fprintln(w, "No resources found. Run `libra scan` first.")
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tTYPE\tVERSION\tSIZE\tPROJECTS\tREGEN\tRISK\tCONFIDENCE\tPATH")
	for _, r := range v.Resources {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d\t%s\t%s\t%d%%\t%s\n",
			r.Name, r.Type, emptyDash(r.Version), humanize.Bytes(uint64(r.LogicalSize)),
			r.ProjectCount, yesNo(r.Regenerable), r.Risk, r.Confidence, r.Path)
	}
	return tw.Flush()
}

// emptyDash renders "-" for an empty version string instead of a blank
// table cell, since not every resource type is versioned (e.g.
// node_modules/build-output have no meaningful Version) and a blank cell
// reads as a rendering bug rather than "not applicable."
func emptyDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
