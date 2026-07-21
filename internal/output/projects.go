package output

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	humanize "github.com/dustin/go-humanize"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/pathutil"
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

// RenderText implements Renderable. Rows nest under the project whose Path
// most closely contains theirs (e.g. a Node project inside a Git
// repository's working tree) so a repo and the projects living in it read
// as one tree instead of unrelated rows -- JSON output is untouched: this
// grouping is computed here from v.Projects without reordering or mutating
// it, so the JSON encoder (which never calls RenderText) still emits the
// original flat, DB-ordered array.
func (v ProjectsView) RenderText(w io.Writer) error {
	if len(v.Projects) == 0 {
		fmt.Fprintln(w, "No projects found. Run `libra scan` first.")
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tTYPE\tDRIVE\tSIZE\tSTATUS\tRESOURCES\tMODIFIED\tPATH")
	for _, root := range buildProjectForest(v.Projects) {
		writeProjectTree(tw, root, "", "")
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

// projectTreeNode is one row of the tree RenderText prints, plus the rows
// nested under it. Built fresh per render from v.Projects; never stored on
// ProjectsView itself.
type projectTreeNode struct {
	line     ProjectLine
	children []*projectTreeNode
}

// buildProjectForest groups lines by filesystem containment: a project
// nests under the OTHER project in lines whose Path most closely (deepest)
// contains its own Path. A project with no such enclosing project in lines
// -- either because it truly has none, or because --type/--drive/--status
// filtered its enclosing project out -- surfaces as its own root rather
// than being dropped. Both root order and each parent's child order follow
// lines' original order, so this never re-sorts what the caller passed in.
func buildProjectForest(lines []ProjectLine) []*projectTreeNode {
	normalized := make([]string, len(lines))
	for i, line := range lines {
		if n, err := pathutil.Normalize(line.Path); err == nil {
			normalized[i] = n
		}
	}

	nodes := make([]*projectTreeNode, len(lines))
	for i, line := range lines {
		nodes[i] = &projectTreeNode{line: line}
	}

	var roots []*projectTreeNode
	for i := range lines {
		parent := nearestEnclosingProject(lines, normalized, i)
		if parent == -1 {
			roots = append(roots, nodes[i])
			continue
		}
		nodes[parent].children = append(nodes[parent].children, nodes[i])
	}
	return roots
}

// nearestEnclosingProject returns the index in lines whose Path is the
// deepest proper ancestor of lines[i].Path, or -1 if none contains it.
// "Deepest" is measured by path-separator count on the pre-normalized
// paths, not raw string length, so case/trailing-slash differences that
// pathutil.Normalize would otherwise erase can't skew the comparison.
func nearestEnclosingProject(lines []ProjectLine, normalized []string, i int) int {
	best := -1
	bestDepth := -1
	for j := range lines {
		if i == j || normalized[j] == "" || normalized[j] == normalized[i] {
			continue
		}
		contains, err := pathutil.IsSameOrChild(lines[i].Path, lines[j].Path)
		if err != nil || !contains {
			continue
		}
		depth := strings.Count(normalized[j], string(filepath.Separator))
		if depth > bestDepth {
			best, bestDepth = j, depth
		}
	}
	return best
}

// writeProjectTree prints node's own row under linePrefix, then recurses
// into its children with the standard box-drawing continuation rule: the
// last child at any level gets "└─ " and an unindented continuation
// ("   "), every earlier sibling gets "├─ " and a continuation that keeps
// drawing the vertical bar ("│  ") so deeper descendants still visibly
// belong to this branch. childBase is the accumulated continuation prefix
// -- not linePrefix itself, which still carries node's own connector -- so
// each child's connector is appended fresh instead of stacking on top of
// node's.
func writeProjectTree(w io.Writer, node *projectTreeNode, linePrefix, childBase string) {
	writeProjectLine(w, node.line, linePrefix)
	for i, child := range node.children {
		connector, continuation := "├─ ", "│  "
		if i == len(node.children)-1 {
			connector, continuation = "└─ ", "   "
		}
		writeProjectTree(w, child, childBase+connector, childBase+continuation)
	}
}

func writeProjectLine(w io.Writer, p ProjectLine, namePrefix string) {
	fmt.Fprintf(w, "%s%s\t%s\t%s\t%s\t%s\t%d\t%s\t%s\n",
		namePrefix, p.Name, p.Type, p.Drive, formatProjectSize(p.LogicalSize, &p.SizeKnown),
		p.Status, p.ResourceCount, formatTime(p.LastModifiedAt), p.Path)
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
