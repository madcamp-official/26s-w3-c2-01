package output

import (
	"bytes"
	"encoding/json"
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
		{Name: "frontend", Type: domain.ProjectTypeNode, Status: domain.ProjectStatusActive, LogicalSize: 123456},
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
