package output

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

// TestProjectsViewRenderText_RendersLogicalSizeAsBytes covers issue #38's
// fix: AnalysisOrchestrator now measures BuildProject.LogicalSize via
// scanner.MeasureResource, so RenderText should humanize it like every other
// size column (ResourcesView) instead of hiding it behind a placeholder.
func TestProjectsViewRenderText_RendersLogicalSizeAsBytes(t *testing.T) {
	view := ProjectsView{Projects: []ProjectLine{
		{Name: "frontend", Type: domain.ProjectTypeNode, Status: domain.ProjectStatusActive, LogicalSize: 123456, SizeKnown: true},
	}}

	var buf bytes.Buffer
	if err := view.RenderText(&buf); err != nil {
		t.Fatalf("RenderText: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "124 kB") {
		t.Errorf("RenderText must render project LogicalSize as a humanized byte count, got:\n%s", out)
	}
}

// TestProjectsViewJSON_StillCarriesLogicalSize confirms logical_size_bytes
// round-trips through the JSON contract unchanged.
func TestProjectsViewJSON_StillCarriesLogicalSize(t *testing.T) {
	view := ProjectsView{Projects: []ProjectLine{{Name: "frontend", LogicalSize: 0}}}

	var buf bytes.Buffer
	if err := New(&buf, true, "projects").Print(view); err != nil {
		t.Fatalf("Print: %v", err)
	}

	var decoded ProjectsView
	if _, err := DecodeEnvelope(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("DecodeEnvelope: %v\noutput: %s", err, buf.String())
	}
	if len(decoded.Projects) != 1 || decoded.Projects[0].LogicalSize != 0 {
		t.Errorf("decoded = %+v, want one project with logical_size_bytes = 0 (JSON contract unchanged)", decoded.Projects)
	}
}

// TestProjectsViewRenderText_UnmeasuredSizeShowsPlaceholderNotZeroBytes is a
// regression test for issue #48: LogicalSize == 0 is ambiguous between "an
// empty project" and "measurement failed" (e.g. a permission error during
// AnalysisOrchestrator.Run's scanner.MeasureResource call). SizeKnown
// disambiguates the two; text output must only show "0 B" for the former.
func TestProjectsViewRenderText_UnmeasuredSizeShowsPlaceholderNotZeroBytes(t *testing.T) {
	view := ProjectsView{Projects: []ProjectLine{
		{Name: "frontend", Type: domain.ProjectTypeNode, Status: domain.ProjectStatusActive, LogicalSize: 0, SizeKnown: false},
	}}

	var buf bytes.Buffer
	if err := view.RenderText(&buf); err != nil {
		t.Fatalf("RenderText: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "—") {
		t.Errorf("RenderText must show the unmeasured-size placeholder when SizeKnown is false, got:\n%s", out)
	}
	if strings.Contains(out, "0 B") {
		t.Errorf("RenderText must not render \"0 B\" when SizeKnown is false, got:\n%s", out)
	}
}

// TestProjectsViewJSON_CarriesSizeKnown confirms size_known round-trips
// through the JSON contract (issue #48): automation consuming this JSON
// needs it to tell "measured as 0 bytes" apart from "not measured".
func TestProjectsViewJSON_CarriesSizeKnown(t *testing.T) {
	view := ProjectsView{Projects: []ProjectLine{{Name: "frontend", LogicalSize: 0, SizeKnown: false}}}

	var buf bytes.Buffer
	if err := New(&buf, true, "projects").Print(view); err != nil {
		t.Fatalf("Print: %v", err)
	}

	if !strings.Contains(buf.String(), `"size_known": false`) {
		t.Errorf("JSON output missing explicit size_known:false, got:\n%s", buf.String())
	}

	var decoded ProjectsView
	if _, err := DecodeEnvelope(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("DecodeEnvelope: %v\noutput: %s", err, buf.String())
	}
	if len(decoded.Projects) != 1 || decoded.Projects[0].SizeKnown != false {
		t.Errorf("decoded = %+v, want one project with size_known = false", decoded.Projects)
	}
}

// TestProjectsViewRenderText_NestsChildUnderEnclosingProject covers the
// tree layout `libra projects` now defaults to: a project living inside
// another project's working tree (e.g. a Node project inside a Git repo)
// renders as a "└─ "-prefixed row under its enclosing project instead of
// an unrelated flat row.
func TestProjectsViewRenderText_NestsChildUnderEnclosingProject(t *testing.T) {
	root := t.TempDir()
	repoPath := filepath.Join(root, "week1")
	frontendPath := filepath.Join(repoPath, "frontend")

	view := ProjectsView{Projects: []ProjectLine{
		{Name: "week1", Path: repoPath, Type: domain.ProjectTypeGit, Status: domain.ProjectStatusActive},
		{Name: "frontend", Path: frontendPath, Type: domain.ProjectTypeNode, Status: domain.ProjectStatusActive},
	}}

	var buf bytes.Buffer
	if err := view.RenderText(&buf); err != nil {
		t.Fatalf("RenderText: %v", err)
	}

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("RenderText produced %d lines, want a header + 2 rows:\n%s", len(lines), buf.String())
	}
	if strings.HasPrefix(lines[1], "└─") || !strings.HasPrefix(lines[1], "week1") {
		t.Errorf("root row = %q, want unprefixed \"week1...\"", lines[1])
	}
	if !strings.HasPrefix(lines[2], "└─ frontend") {
		t.Errorf("child row = %q, want \"└─ frontend...\" nested under week1", lines[2])
	}
}

// TestProjectsViewRenderText_MultipleChildrenUseBranchThenCorner covers the
// box-drawing rule: every child but the last uses "├─ " (keeping the
// vertical bar alive for siblings still to come); only the last uses
// "└─ ".
func TestProjectsViewRenderText_MultipleChildrenUseBranchThenCorner(t *testing.T) {
	root := t.TempDir()
	repoPath := filepath.Join(root, "monorepo")
	apiPath := filepath.Join(repoPath, "api")
	webPath := filepath.Join(repoPath, "web")

	view := ProjectsView{Projects: []ProjectLine{
		{Name: "monorepo", Path: repoPath, Type: domain.ProjectTypeGit, Status: domain.ProjectStatusActive},
		{Name: "api", Path: apiPath, Type: domain.ProjectTypeNode, Status: domain.ProjectStatusActive},
		{Name: "web", Path: webPath, Type: domain.ProjectTypeNode, Status: domain.ProjectStatusActive},
	}}

	var buf bytes.Buffer
	if err := view.RenderText(&buf); err != nil {
		t.Fatalf("RenderText: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "├─ api") {
		t.Errorf("first of two children must use the branch connector \"├─ \", got:\n%s", out)
	}
	if !strings.Contains(out, "└─ web") {
		t.Errorf("last child must use the corner connector \"└─ \", got:\n%s", out)
	}
}

// TestProjectsViewRenderText_UnrelatedProjectsStayFlat is a regression test
// against over-eager nesting: two projects that don't contain one another
// (different top-level roots) must both render as unprefixed root rows.
func TestProjectsViewRenderText_UnrelatedProjectsStayFlat(t *testing.T) {
	root := t.TempDir()
	view := ProjectsView{Projects: []ProjectLine{
		{Name: "week1", Path: filepath.Join(root, "week1"), Type: domain.ProjectTypeGit, Status: domain.ProjectStatusActive},
		{Name: "week2", Path: filepath.Join(root, "week2"), Type: domain.ProjectTypeGit, Status: domain.ProjectStatusActive},
	}}

	var buf bytes.Buffer
	if err := view.RenderText(&buf); err != nil {
		t.Fatalf("RenderText: %v", err)
	}
	out := buf.String()

	if strings.Contains(out, "└─") || strings.Contains(out, "├─") {
		t.Errorf("unrelated top-level projects must not be nested under one another, got:\n%s", out)
	}
}

// TestProjectsViewRenderText_TreeGroupingLeavesJSONFlatAndUnordered
// confirms the tree grouping RenderText computes is purely a text-rendering
// concern: JSON output still encodes v.Projects exactly as given (same
// order, no nesting), since json.Marshal never calls RenderText.
func TestProjectsViewRenderText_TreeGroupingLeavesJSONFlatAndUnordered(t *testing.T) {
	root := t.TempDir()
	repoPath := filepath.Join(root, "week1")
	frontendPath := filepath.Join(repoPath, "frontend")
	view := ProjectsView{Projects: []ProjectLine{
		{Name: "frontend", Path: frontendPath, Type: domain.ProjectTypeNode},
		{Name: "week1", Path: repoPath, Type: domain.ProjectTypeGit},
	}}

	var buf bytes.Buffer
	if err := New(&buf, true, "projects").Print(view); err != nil {
		t.Fatalf("Print: %v", err)
	}

	var decoded ProjectsView
	if _, err := DecodeEnvelope(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("DecodeEnvelope: %v\noutput: %s", err, buf.String())
	}
	if len(decoded.Projects) != 2 || decoded.Projects[0].Name != "frontend" || decoded.Projects[1].Name != "week1" {
		t.Errorf("decoded = %+v, want the original flat [frontend, week1] order preserved", decoded.Projects)
	}
}
