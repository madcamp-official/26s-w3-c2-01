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

// EnvelopeSchemaVersion is the current shape of Envelope. Bump it (and
// document the change in docs/libra_integration_contracts.md §13) whenever
// a field is added, renamed, or removed -- never reuse a version number for
// an incompatible shape, since that's the whole point of shipping one.
const EnvelopeSchemaVersion = 1

// Outcome is the envelope-level summary of how a command's own execution
// went -- distinct from any domain-specific status nested under Data (e.g.
// PlanView.Status's READY/INSUFFICIENT_CANDIDATES), which is why the
// envelope field is named Outcome rather than the contract draft's
// original "status": both concepts existing at different JSON depths
// under the same name invited exactly the kind of `jq .status` confusion
// issue #42 exists to eliminate (see docs/libra_integration_contracts.md
// §13 and #59's follow-up decision).
type Outcome string

const (
	// OutcomeSuccess is every command's default: it did what it was asked
	// with nothing left unresolved. Most commands (pure reads of
	// already-scanned data -- projects/resources/summary/explain/impact/
	// transactions -- and issues, whose own listing operation isn't
	// degraded by what it finds) can only ever report this; there is no
	// partial-execution concept for a straight DB read.
	OutcomeSuccess Outcome = "SUCCESS"
	// OutcomePartial means the command completed but part of its own work
	// was skipped, degraded, or fell short of what was requested (e.g.
	// scan hit permission errors on some paths, clean moved some items but
	// not others, plan couldn't reach --target).
	OutcomePartial Outcome = "PARTIAL"
	// OutcomeFailed means the command's own operation did not succeed at
	// all, but still produced a result worth printing rather than exiting
	// with a bare error (e.g. domain.TransactionFailed) -- distinct from a
	// Go error returned from RunE, which never reaches Print at all.
	OutcomeFailed Outcome = "FAILED"
)

// EnvelopeIssue is one problem or partial-success detail surfaced at the
// envelope level (issue #59/#42), so any --json consumer can check one
// place regardless of which command it ran, instead of learning a
// different field name/shape per command (scan's Warnings, issues'
// Issues, clean's per-item Detail, a transaction's per-item Reason).
// Fields that don't apply to a given source are left at their zero value
// (omitempty) rather than every source being forced into full richness.
type EnvelopeIssue struct {
	Code      string `json:"code"`
	Severity  string `json:"severity,omitempty"`
	Phase     string `json:"phase,omitempty"`
	Adapter   string `json:"adapter,omitempty"`
	Path      string `json:"path,omitempty"`
	Operation string `json:"operation,omitempty"`
	Message   string `json:"message"`
}

// Envelope is the common --json shape every command's Printer wraps its
// Renderable in (docs/libra_integration_contracts.md §13). Command/
// SchemaVersion are the same for every response from a given command;
// Outcome/Issues/Unverified vary per invocation.
type Envelope struct {
	Command       string          `json:"command"`
	SchemaVersion int             `json:"schema_version"`
	Outcome       Outcome         `json:"outcome"`
	Data          Renderable      `json:"data"`
	Issues        []EnvelopeIssue `json:"issues"`
	Unverified    []string        `json:"unverified"`
}

// EnvelopeOptions carries the per-invocation parts of an Envelope that only
// some commands need to set explicitly -- see PrintEnvelope. The zero value
// (empty Outcome) renders as OutcomeSuccess with no issues, which is
// correct for every command that doesn't call PrintEnvelope directly.
type EnvelopeOptions struct {
	Outcome    Outcome
	Issues     []EnvelopeIssue
	Unverified []string
}

// Printer writes a Renderable to Out in its configured Format, labeled with
// the CLI command that produced it (used only for JSON's envelope; Command
// is ignored in Text mode).
type Printer struct {
	Out     io.Writer
	Format  Format
	Command string
}

// New returns a Printer that writes JSON when jsonOutput is true, text
// otherwise. This mirrors the --json persistent flag every command shares.
// command is the bare Cobra command name (e.g. "scan", "clean" -- not the
// full Use string like "clean --plan <id>"), used to populate the JSON
// envelope's "command" field; every cmd/*.go call site passes its own
// literal command name explicitly rather than deriving it from cmd.Use, to
// keep the mapping obvious at each call site instead of parsed out of a
// human-facing usage string.
func New(w io.Writer, jsonOutput bool, command string) *Printer {
	f := Text
	if jsonOutput {
		f = JSON
	}
	return &Printer{Out: w, Format: f, Command: command}
}

// Print renders v to the printer's writer in its configured format, as a
// SUCCESS envelope with no issues in JSON mode. Most commands -- anything
// without its own partial-success concept -- call this.
func (p *Printer) Print(v Renderable) error {
	return p.PrintEnvelope(v, EnvelopeOptions{})
}

// PrintEnvelope renders v like Print, but lets a command that can genuinely
// only partially succeed (scan, issues, clean, purge, restore -- see #59)
// report that explicitly instead of every response silently claiming
// OutcomeSuccess. Issues/Unverified are always encoded as "[]", never
// null, when empty, so a JSON consumer never has to special-case a missing
// array vs an empty one.
func (p *Printer) PrintEnvelope(v Renderable, opts EnvelopeOptions) error {
	if p.Format == JSON {
		outcome := opts.Outcome
		if outcome == "" {
			outcome = OutcomeSuccess
		}
		issues := opts.Issues
		if issues == nil {
			issues = []EnvelopeIssue{}
		}
		unverified := opts.Unverified
		if unverified == nil {
			unverified = []string{}
		}
		env := Envelope{
			Command: p.Command, SchemaVersion: EnvelopeSchemaVersion,
			Outcome: outcome, Data: v, Issues: issues, Unverified: unverified,
		}
		enc := json.NewEncoder(p.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(env)
	}
	return v.RenderText(p.Out)
}

// DecodeEnvelope parses raw --json output and unmarshals its "data" field
// into target (a pointer to the specific View type the caller expects for
// that command, e.g. &ProjectsView{}) -- Envelope.Data can't be decoded
// directly since it's typed as the Renderable interface, which
// encoding/json can't unmarshal into without knowing the concrete type.
// Both this package's own tests and cmd's tests share this helper rather
// than each hand-rolling the same two-step unmarshal, and it doubles as
// the reference implementation for any other Go-based --json consumer.
func DecodeEnvelope(raw []byte, target any) (Envelope, error) {
	var wire struct {
		Command       string          `json:"command"`
		SchemaVersion int             `json:"schema_version"`
		Outcome       Outcome         `json:"outcome"`
		Data          json.RawMessage `json:"data"`
		Issues        []EnvelopeIssue `json:"issues"`
		Unverified    []string        `json:"unverified"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return Envelope{}, err
	}
	if err := json.Unmarshal(wire.Data, target); err != nil {
		return Envelope{}, err
	}
	return Envelope{
		Command: wire.Command, SchemaVersion: wire.SchemaVersion, Outcome: wire.Outcome,
		Issues: wire.Issues, Unverified: wire.Unverified,
	}, nil
}

// yesNo renders a bool as the "yes"/"no" text tables in this package use,
// shared by any view with a yes/no column or line (e.g. regenerable).
func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
