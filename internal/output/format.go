// Package output renders libra's analysis results as human-readable text or
// machine-readable JSON, so every command can share one --json contract
// instead of formatting output ad hoc.
package output

import (
	"encoding/json"
	"io"
)

// Format selects how a Printer renders a Renderable.
type Format int

const (
	Text Format = iota
	JSON
)

// Renderable is a result type a Printer can display. Implementations must
// also be safe to encode with encoding/json (exported fields, json tags)
// since the same value is used for both Text and JSON output.
type Renderable interface {
	RenderText(w io.Writer) error
}

// Printer writes a Renderable to Out in its configured Format.
type Printer struct {
	Out    io.Writer
	Format Format
}

// New returns a Printer that writes JSON when jsonOutput is true, text
// otherwise. This mirrors the --json persistent flag every command shares.
func New(w io.Writer, jsonOutput bool) *Printer {
	f := Text
	if jsonOutput {
		f = JSON
	}
	return &Printer{Out: w, Format: f}
}

// Print renders v to the printer's writer in its configured format.
func (p *Printer) Print(v Renderable) error {
	if p.Format == JSON {
		enc := json.NewEncoder(p.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	}
	return v.RenderText(p.Out)
}

// yesNo renders a bool as the "yes"/"no" text tables in this package use,
// shared by any view with a yes/no column or line (e.g. regenerable).
func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// projectSizeDisplay/projectSizeFootnote (issue #38): no project detector
// currently sets BuildProject.LogicalSize -- internal/app/resource_service.go
// only measures Resource sizes, never a project's -- so the field is always
// its zero value for every project, unconditionally, not just when a project
// happens to be empty. Rendering that zero as "0 B" reads as a measurement
// ("this project is empty") when it is really "not measured at all", so
// ProjectsView and ExplainView's project case both show this placeholder
// instead of humanize.Bytes(LogicalSize). Resource sizes (Resource.LogicalSize
// via scanner.MeasureResource) are real measurements and are unaffected --
// only the project-level SIZE column/line uses this.
const projectSizeDisplay = "—"

const projectSizeFootnote = "Project SIZE is not measured in this scan mode; see resource sizes (`libra resources`) instead."
