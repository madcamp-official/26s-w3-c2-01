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
