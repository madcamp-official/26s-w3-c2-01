package output

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/madcamp-official/26s-w3-c2-01/internal/app"
)

type IssuesView struct {
	ScanID string      `json:"scan_id"`
	Issues []IssueLine `json:"issues"`
}

type IssueLine struct {
	Code      app.IssueCode     `json:"code"`
	Phase     app.AnalysisPhase `json:"phase"`
	Adapter   string            `json:"adapter,omitempty"`
	Path      string            `json:"path,omitempty"`
	Operation string            `json:"operation,omitempty"`
	Severity  app.IssueSeverity `json:"severity"`
	Message   string            `json:"message"`
}

func (v IssuesView) RenderText(w io.Writer) error {
	if len(v.Issues) == 0 {
		fmt.Fprintf(w, "No issues found for scan %s.\n", v.ScanID)
		return nil
	}

	fmt.Fprintf(w, "Issues for scan %s\n\n", v.ScanID)
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "SEVERITY\tCODE\tPHASE\tADAPTER\tOPERATION\tPATH\tMESSAGE")
	for _, issue := range v.Issues {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			issue.Severity, issue.Code, issue.Phase, emptyDash(issue.Adapter),
			emptyDash(issue.Operation), emptyDash(issue.Path), issue.Message)
	}
	return tw.Flush()
}
