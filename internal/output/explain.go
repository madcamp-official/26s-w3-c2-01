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
	UsedBy         []ExplainUsage      `json:"used_by,omitempty"`
	ExpectedImpact []ExplainImpactLine `json:"expected_impact,omitempty"`
	Recovery       string              `json:"recovery,omitempty"`

	// Project-only fields.
	ProjectType    domain.ProjectType   `json:"project_type,omitempty"`
	Status         domain.ProjectStatus `json:"status,omitempty"`
	LastModifiedAt time.Time            `json:"last_modified_at,omitempty"`
	Requires       []ExplainUsage       `json:"requires,omitempty"`

	// Shared.
	LogicalSize    int64     `json:"logical_size_bytes"`
	LastObservedAt time.Time `json:"last_observed_at"`
	Unverified     []string  `json:"unverified,omitempty"`
}

// ExplainUsage is one dependency edge's evidence, rendered under "Used by"
// (resource target) or "Requires" (project target).
type ExplainUsage struct {
	Name     string                `json:"name"`
	Path     string                `json:"path,omitempty"`
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
	for _, usage := range v.UsedBy {
		fmt.Fprintf(w, "- %s\n", usage.Path)
		renderEvidence(w, usage.Evidence)
	}
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
	fmt.Fprintf(w, "Recovery: %s\n", v.Recovery)

	return v.renderUnverified(w)
}

func (v ExplainView) renderProject(w io.Writer) error {
	fmt.Fprintf(w, "Project: %s (%s)\n", v.Name, v.ProjectType)
	fmt.Fprintf(w, "Path: %s\n", v.Path)
	fmt.Fprintf(w, "Size: %s\n", humanize.Bytes(uint64(v.LogicalSize)))
	fmt.Fprintf(w, "Status: %s\n", v.Status)
	fmt.Fprintf(w, "Last modified: %s\n", formatTime(v.LastModifiedAt))
	fmt.Fprintf(w, "Last observed: %s\n", formatTime(v.LastObservedAt))
	fmt.Fprintln(w)

	fmt.Fprintln(w, "Requires:")
	if len(v.Requires) == 0 {
		fmt.Fprintln(w, "- none found in the last scan")
	}
	for _, usage := range v.Requires {
		fmt.Fprintf(w, "- %s\n", usage.Name)
		renderEvidence(w, usage.Evidence)
	}

	return v.renderUnverified(w)
}

func renderEvidence(w io.Writer, evidence []ExplainEvidenceLine) {
	for _, e := range evidence {
		fmt.Fprintf(w, "  Evidence: %s\n", e.Kind)
		if e.Source != "" {
			fmt.Fprintf(w, "  Source: %s\n", e.Source)
		}
		if e.Property != "" {
			fmt.Fprintf(w, "  Property: %s\n", e.Property)
		}
	}
}

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
