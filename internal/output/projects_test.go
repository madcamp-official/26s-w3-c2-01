package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

// TestProjectsViewRenderText_SizeIsNeverRenderedAsBytes covers issue #38: no
// project detector measures BuildProject.LogicalSize (only Resource sizes
// are measured, in internal/app/resource_service.go), so it is always 0 for
// every project -- not just when a project happens to be empty. Rendering
// "0 B" would misreport "not measured" as "measured empty". LogicalSize is
// deliberately set non-zero here to prove RenderText ignores it entirely,
// not merely special-cases 0.
func TestProjectsViewRenderText_SizeIsNeverRenderedAsBytes(t *testing.T) {
	view := ProjectsView{Projects: []ProjectLine{
		{Name: "frontend", Type: domain.ProjectTypeNode, Status: domain.ProjectStatusActive, LogicalSize: 123456},
	}}

	var buf bytes.Buffer
	if err := view.RenderText(&buf); err != nil {
		t.Fatalf("RenderText: %v", err)
	}
	out := buf.String()

	if strings.Contains(out, "123456") || strings.Contains(out, "0 B") {
		t.Errorf("RenderText must not render project LogicalSize as a byte count, got:\n%s", out)
	}
	if !strings.Contains(out, projectSizeDisplay) {
		t.Errorf("RenderText missing the %q placeholder, got:\n%s", projectSizeDisplay, out)
	}
	if !strings.Contains(out, "not measured") {
		t.Errorf("RenderText missing a footnote explaining unmeasured SIZE, got:\n%s", out)
	}
}

// TestProjectsViewJSON_StillCarriesLogicalSize confirms this display fix is
// text-only: the JSON contract (logical_size_bytes) is intentionally left
// unchanged here. Adding a size_known-style field would be a CLI JSON schema
// change requiring team agreement (docs/libra_collaboration_rules.md §9),
// which is out of scope for issue #38's display-only fix.
func TestProjectsViewJSON_StillCarriesLogicalSize(t *testing.T) {
	view := ProjectsView{Projects: []ProjectLine{{Name: "frontend", LogicalSize: 0}}}

	var buf bytes.Buffer
	if err := New(&buf, true).Print(view); err != nil {
		t.Fatalf("Print: %v", err)
	}

	var decoded ProjectsView
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("Unmarshal: %v\noutput: %s", err, buf.String())
	}
	if len(decoded.Projects) != 1 || decoded.Projects[0].LogicalSize != 0 {
		t.Errorf("decoded = %+v, want one project with logical_size_bytes = 0 (JSON contract unchanged)", decoded.Projects)
	}
}
