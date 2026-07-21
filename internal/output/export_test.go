package output

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/app"
	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

func TestExportWriters(t *testing.T) {
	report := app.ExportReport{GeneratedAt: time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC), Scan: app.ScanRecord{ID: "scan-1", Status: app.ScanStatusCompleted}, Projects: []domain.BuildProject{{Name: "App", RootPath: "/app"}}, Resources: []domain.Resource{{Name: "modules", Risk: domain.RiskReview}}}
	var jsonOutput bytes.Buffer
	if err := WriteExportJSON(&jsonOutput, report); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(jsonOutput.String(), `"generated_at"`) || !strings.Contains(jsonOutput.String(), `"projects"`) {
		t.Fatalf("JSON = %s", &jsonOutput)
	}
	var markdown bytes.Buffer
	if err := WriteExportMarkdown(&markdown, report); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# Libra analysis report", "## Projects", "## Resources", "App"} {
		if !strings.Contains(markdown.String(), want) {
			t.Fatalf("markdown missing %q:\n%s", want, &markdown)
		}
	}
}
