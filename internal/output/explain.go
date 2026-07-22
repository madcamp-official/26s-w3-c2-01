package output

import (
	"fmt"
	"io"
	"time"

	humanize "github.com/dustin/go-humanize"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

// ExplainKind distinguishes which of ExplainView's two shapes is populated:
// libra explain accepts both a resource and a project target (F-07/3.6 in
// docs/libra_cli_commands_and_schedule.md).
type ExplainKind string

const (
	ExplainKindResource ExplainKind = "resource"
	ExplainKindProject  ExplainKind = "project"
)

// ExplainView is the rendered result of `libra explain <target>`.
type ExplainView struct {
	Kind ExplainKind `json:"kind"`
	Name string      `json:"name"`
	Path string      `json:"path"`

	// Resource-only fields.
	ResourceType   domain.ResourceType `json:"resource_type,omitempty"`
	Version        string              `json:"version,omitempty"`
	Regenerable    *bool               `json:"regenerable,omitempty"`
	Risk           domain.RiskLevel    `json:"risk,omitempty"`
	Confidence     *int                `json:"confidence,omitempty"`
	RiskReasons    []domain.RiskReason `json:"risk_reasons,omitempty"`
	UsedBy         []ExplainUsage      `json:"used_by,omitempty"`
	ExpectedImpact []ExplainImpactLine `json:"expected_impact,omitempty"`
	Recovery       string              `json:"recovery,omitempty"`

	// Project-only fields.
	ProjectType domain.ProjectType   `json:"project_type,omitempty"`
	Status      domain.ProjectStatus `json:"status,omitempty"`
	// LastModifiedAt is a *time.Time, not time.Time, because JSON's
	// "omitempty" has no effect on a non-pointer struct field -- a bare
	// time.Time here would still marshal the Go zero value
	// ("0001-01-01T00:00:00Z") into a resource-kind view's JSON even though
	// this field is never set for one, unlike the text renderer, which
	// never prints "Last modified" for a resource at all (see
	// renderResource). nil means "not a project view", not "date unknown".
	LastModifiedAt *time.Time     `json:"last_modified_at,omitempty"`
	Requires       []ExplainUsage `json:"requires,omitempty"`
	// SizeKnown is nil for a resource-kind view (LogicalSize is always
	// measured for a resource, per domain.Resource.SizeKnown's own
	// semantics being out of scope here); non-nil for a project-kind view,
	// where false means LogicalSize is 0 because measurement failed, not
	// because the project is actually empty (issue #48).
	SizeKnown *bool `json:"size_known,omitempty"`

	// Shared.
	LogicalSize    int64     `json:"logical_size_bytes"`
	LastObservedAt time.Time `json:"last_observed_at"`
	Unverified     []string  `json:"unverified,omitempty"`
}

// ExplainUsage is one dependency edge's evidence, rendered under "Used by"
// (resource target) or "Uses" (project target), grouped into "Owns"/
// "Requires" relation subsections by renderUsageGroup.
type ExplainUsage struct {
	Name     string                `json:"name"`
	Path     string                `json:"path,omitempty"`
	Relation domain.RelationType   `json:"relation"`
	Evidence []ExplainEvidenceLine `json:"evidence,omitempty"`
}

// ExplainEvidenceLine is one piece of evidence backing a dependency edge.
type ExplainEvidenceLine struct {
	Kind     domain.EvidenceKind `json:"kind"`
	Source   string              `json:"source,omitempty"`
	Property string              `json:"property,omitempty"`
}

// ExplainImpactLine is one scope's judged impact, labeled for the explain
// command's "Expected impact" section.
type ExplainImpactLine struct {
	Label string             `json:"label"`
	Scope domain.ImpactScope `json:"scope"`
	Level domain.ImpactLevel `json:"level"`
	Note  string             `json:"note,omitempty"`
}

// RenderText implements Renderable.
func (v ExplainView) RenderText(w io.Writer) error {
	if v.Kind == ExplainKindProject {
		return v.renderProject(w)
	}
	return v.renderResource(w)
}

func (v ExplainView) renderResource(w io.Writer) error {
	label := v.Name
	if v.Version != "" {
		label = fmt.Sprintf("%s %s", v.Name, v.Version)
	}
	fmt.Fprintf(w, "Resource: %s\n", label)
	fmt.Fprintf(w, "Path: %s\n", v.Path)
	fmt.Fprintf(w, "Size: %s\n", humanize.Bytes(uint64(v.LogicalSize)))
	fmt.Fprintln(w)

	fmt.Fprintln(w, "Used by:")
	if len(v.UsedBy) == 0 {
		fmt.Fprintln(w, "- none found in the last scan")
	}
	renderUsageGroup(w, "  Owns:", v.UsedBy, domain.RelationOwns)
	renderUsageGroup(w, "  Requires:", v.UsedBy, domain.RelationRequires)
	fmt.Fprintln(w)

	fmt.Fprintln(w, "Expected impact:")
	for _, line := range v.ExpectedImpact {
		if line.Note != "" {
			fmt.Fprintf(w, "- %s: %s (%s)\n", line.Label, line.Level, line.Note)
		} else {
			fmt.Fprintf(w, "- %s: %s\n", line.Label, line.Level)
		}
	}
	fmt.Fprintln(w)

	if v.Regenerable != nil {
		fmt.Fprintf(w, "Regenerable: %s\n", yesNo(*v.Regenerable))
	}
	fmt.Fprintf(w, "Risk: %s\n", v.Risk)
	if v.Confidence != nil {
		fmt.Fprintf(w, "Confidence: %d%%\n", *v.Confidence)
	}
	renderRiskReasons(w, v.RiskReasons)
	fmt.Fprintf(w, "Recovery: %s\n", v.Recovery)

	return v.renderUnverified(w)
}

func (v ExplainView) renderProject(w io.Writer) error {
	fmt.Fprintf(w, "Project: %s (%s)\n", v.Name, v.ProjectType)
	fmt.Fprintf(w, "Path: %s\n", v.Path)
	fmt.Fprintf(w, "Size: %s\n", formatProjectSize(v.LogicalSize, v.SizeKnown))
	fmt.Fprintf(w, "Status: %s\n", v.Status)
	var lastModifiedAt time.Time
	if v.LastModifiedAt != nil {
		lastModifiedAt = *v.LastModifiedAt
	}
	fmt.Fprintf(w, "Last modified: %s\n", formatTime(lastModifiedAt))
	fmt.Fprintf(w, "Last observed: %s\n", formatTime(v.LastObservedAt))
	fmt.Fprintln(w)

	fmt.Fprintln(w, "Uses:")
	if len(v.Requires) == 0 {
		fmt.Fprintln(w, "- none found in the last scan")
	}
	renderUsageGroup(w, "  Owns:", v.Requires, domain.RelationOwns)
	renderUsageGroup(w, "  Requires:", v.Requires, domain.RelationRequires)

	return v.renderUnverified(w)
}

// renderUsageGroup prints one relation's subsection of "Used by" (e.g.
// "  Owns:") followed by its usages, or nothing at all if none of usages
// match relation -- a resource only ever shows the relation groups that
// actually apply to it.
func renderUsageGroup(w io.Writer, header string, usages []ExplainUsage, relation domain.RelationType) {
	var matched []ExplainUsage
	for _, usage := range usages {
		if usage.Relation == relation {
			matched = append(matched, usage)
		}
	}
	if len(matched) == 0 {
		return
	}
	fmt.Fprintln(w, header)
	for _, usage := range matched {
		fmt.Fprintf(w, "  - %s\n", usageLabel(usage))
		renderEvidence(w, usage.Evidence, "      ")
	}
}

// usageLabel prefers a usage's Path -- set on "Used by" entries, one per
// owning/requiring project -- and falls back to Name, set on "Uses" entries
// (one per resource, which has no path of its own distinct from the
// project's).
func usageLabel(usage ExplainUsage) string {
	if usage.Path != "" {
		return usage.Path
	}
	return usage.Name
}

// renderEvidence is shared by renderResource ("Used by") and renderProject
// ("Uses"): both sections list dependency edges with the same
// Evidence/Source/Property sub-lines, just under a different heading, at a
// caller-supplied indent.
func renderEvidence(w io.Writer, evidence []ExplainEvidenceLine, indent string) {
	for _, e := range evidence {
		fmt.Fprintf(w, "%sEvidence: %s\n", indent, e.Kind)
		if e.Source != "" {
			fmt.Fprintf(w, "%sSource: %s\n", indent, e.Source)
		}
		if e.Property != "" {
			fmt.Fprintf(w, "%sProperty: %s\n", indent, e.Property)
		}
	}
}

// renderUnverified is a no-op when there's nothing to say -- ExplainView
// currently never actually populates Unverified (no caller wires
// UnverifiedScope data into it yet), so this only ever prints the "no
// output" branch today. Kept as a real field/method pair, not deleted,
// since F-07 requires a "분석하지 못한 범위" section once that data exists.
func (v ExplainView) renderUnverified(w io.Writer) error {
	if len(v.Unverified) == 0 {
		return nil
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Unverified:")
	for _, item := range v.Unverified {
		fmt.Fprintf(w, "- %s\n", item)
	}
	return nil
}
