package output

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/madcamp-official/26s-w3-c2-01/internal/app"
)

// ExportView adapts the portable export report to the interactive CLI
// renderer. `export --format json` remains the raw, portable report format;
// the global `--json` flag wraps this same data in the shared CLI envelope.
type ExportView app.ExportReport

func (v ExportView) RenderText(w io.Writer) error {
	return WriteExportMarkdown(w, app.ExportReport(v))
}

func WriteExportJSON(w io.Writer, report app.ExportReport) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

func WriteExportMarkdown(w io.Writer, report app.ExportReport) error {
	fmt.Fprintln(w, "# Libra analysis report")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- Generated: %s\n", report.GeneratedAt.Format("2006-01-02 15:04:05Z07:00"))
	fmt.Fprintf(w, "- Scan: `%s` (%s)\n", report.Scan.ID, report.Scan.Status)
	fmt.Fprintf(w, "- Projects: %d\n- Resources: %d\n- Issues: %d\n- Transactions: %d\n", len(report.Projects), len(report.Resources), len(report.Issues), len(report.Transactions))

	fmt.Fprintln(w)
	fmt.Fprintln(w, "## Projects")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| Name | Type | Size | Path |")
	fmt.Fprintln(w, "|---|---|---:|---|")
	for _, project := range report.Projects {
		fmt.Fprintf(w, "| %s | %s | %d | `%s` |\n", escapeMarkdown(project.Name), project.Type, project.LogicalSize, project.RootPath)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "## Resources")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| Name | Type | Risk | Confidence | Size | Path |")
	fmt.Fprintln(w, "|---|---|---|---:|---:|---|")
	for _, resource := range report.Resources {
		fmt.Fprintf(w, "| %s | %s | %s | %d | %d | `%s` |\n", escapeMarkdown(resource.Name), resource.Type, resource.Risk, resource.Confidence, resource.LogicalSize, resource.DisplayPath)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "## Issues")
	fmt.Fprintln(w)
	if len(report.Issues) == 0 {
		fmt.Fprintln(w, "No issues recorded.")
	}
	for _, issue := range report.Issues {
		fmt.Fprintf(w, "- **%s / %s** %s — %s\n", issue.Severity, issue.Code, escapeMarkdown(issue.Path), escapeMarkdown(issue.Message))
	}
	return nil
}

func escapeMarkdown(value string) string {
	result := ""
	for _, r := range value {
		if r == '|' {
			result += "\\|"
		} else {
			result += string(r)
		}
	}
	return result
}
