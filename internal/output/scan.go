package output

import (
	"fmt"
	"io"
)

// ScanView is the rendered result of `libra scan` (issue #42). scan was the
// last command still writing plain text directly with fmt.Fprintln and no
// --json support at all -- every other command already goes through
// Printer/Renderable (see docs/libra_integration_contracts.md §13's "남은
// 작업" 2, common JSON envelope migration). This gives scan the same
// text/--json pair every other command has; it does not introduce the
// shared envelope/schema_version itself, which is a separate, larger,
// cross-command decision left for a follow-up.
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
