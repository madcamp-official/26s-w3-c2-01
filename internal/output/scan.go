package output

import (
	"fmt"
	"io"
)

// ScanView is the rendered result of `libra scan` (issue #42).
type ScanView struct {
	RootsScanned   int         `json:"roots_scanned"`
	ProjectsFound  int         `json:"projects_found"`
	ResourcesFound int         `json:"resources_found"`
	FilesInspected int64       `json:"files_inspected"`
	Warnings       []ScanIssue `json:"warnings"`
}

// ScanIssue is one recoverable issue the scan collected without aborting.
// The JSON view carries full detail (not just a count) so automation (export,
// AI input, a future `libra issues`) can act on it; RenderText below only
// prints the total count, matching scan's prior text behavior.
type ScanIssue struct {
	Code      string `json:"code"`
	Phase     string `json:"phase"`
	Severity  string `json:"severity"`
	Path      string `json:"path,omitempty"`
	Operation string `json:"operation,omitempty"`
	Message   string `json:"message"`
}

// RenderText implements Renderable. Text stays byte-for-byte identical to
// scan's pre-#42 hardcoded output, so nothing observing it in text mode
// (docs, golden tests, a script grepping stdout) needs to change.
func (v ScanView) RenderText(w io.Writer) error {
	fmt.Fprintln(w, "Scan completed")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Roots scanned:   %d\n", v.RootsScanned)
	fmt.Fprintf(w, "Projects found:  %d\n", v.ProjectsFound)
	fmt.Fprintf(w, "Resources found: %d\n", v.ResourcesFound)
	fmt.Fprintf(w, "Files inspected: %d\n", v.FilesInspected)
	fmt.Fprintf(w, "Warnings:        %d\n", len(v.Warnings))
	return nil
}

// Envelope maps ScanView onto the shared JSON envelope (issue #59): a scan
// that hit any recoverable issue is PARTIAL, not SUCCESS -- it still
// produced a usable result, but not a complete one, the same distinction
// `summary`'s Coverage line already draws for a scan record after the
// fact. Warnings become envelope Issues directly; ScanIssue and
// EnvelopeIssue share the same field set by design (scan was the shape
// EnvelopeIssue was modeled on), so this is a plain copy.
func (v ScanView) Envelope() EnvelopeOptions {
	opts := EnvelopeOptions{Outcome: OutcomeSuccess}
	if len(v.Warnings) > 0 {
		opts.Outcome = OutcomePartial
	}
	for _, w := range v.Warnings {
		opts.Issues = append(opts.Issues, EnvelopeIssue{
			Code: w.Code, Severity: w.Severity, Phase: w.Phase,
			Path: w.Path, Operation: w.Operation, Message: w.Message,
		})
	}
	return opts
}
