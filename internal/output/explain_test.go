package output

import (
	"bytes"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

func TestExplainViewRenderTextConfidenceEligible(t *testing.T) {
	profile := domain.ConfidenceProfile{
		ModelVersion: 1,
		// CleanupSummary reads the top-level scalar fields, not Assessments
		// (Assessments only drives the printed per-axis breakdown table) --
		// all must clear their thresholds for Eligible to be true.
		Classification: 100, Ownership: 100, Dependency: 100,
		Regenerability: 100, PathSafety: 100, ScanCoverage: 100, Freshness: 100,
		Assessments: []domain.ConfidenceAssessment{
			{Axis: domain.AxisOwnership, Score: 100, Status: domain.ConfidenceKnown},
		},
	}
	summary := profile.CleanupSummary()
	view := ExplainView{Kind: ExplainKindResource, Name: "node_modules", ConfidenceProfile: &profile, ConfidenceSummary: &summary}

	var buf bytes.Buffer
	if err := view.RenderText(&buf); err != nil {
		t.Fatalf("RenderText() error = %v", err)
	}
	out := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte("Cleanup eligibility: eligible for automatic selection by `libra plan`")) {
		t.Fatalf("output missing eligible line:\n%s", out)
	}
	if bytes.Contains(buf.Bytes(), []byte("not eligible")) {
		t.Fatalf("eligible profile must not print the not-eligible line:\n%s", out)
	}
}

func TestExplainViewRenderTextConfidenceNotEligible(t *testing.T) {
	profile := domain.ConfidenceProfile{
		ModelVersion: 1,
		// Only PathSafety is below its threshold (90), so it must be the
		// sole reported limiting axis.
		Classification: 100, Ownership: 100, Dependency: 100,
		Regenerability: 100, PathSafety: 0, ScanCoverage: 100, Freshness: 100,
		Assessments: []domain.ConfidenceAssessment{
			{Axis: domain.AxisPathSafety, Score: 0, Status: domain.ConfidenceUnknown},
		},
	}
	summary := profile.CleanupSummary()
	view := ExplainView{Kind: ExplainKindResource, Name: "node_modules", ConfidenceProfile: &profile, ConfidenceSummary: &summary}

	var buf bytes.Buffer
	if err := view.RenderText(&buf); err != nil {
		t.Fatalf("RenderText() error = %v", err)
	}
	out := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte("Cleanup eligibility: not eligible for automatic selection by `libra plan` (limited by PATH_SAFETY at 0%)")) {
		t.Fatalf("output missing not-eligible line with the correct limiting axis/score:\n%s", out)
	}
}

// TestExplainViewRenderTextSkipsConfidenceBreakdownForLegacyProfile covers a
// resource that predates the claim-based confidence model (ModelVersion 0,
// no per-axis Assessments, e.g. seedWindowsSDKDependency-style test
// fixtures constructed before that model existed): explain must not print
// an empty or fabricated breakdown for it.
func TestExplainViewRenderTextSkipsConfidenceBreakdownForLegacyProfile(t *testing.T) {
	profile := domain.ConfidenceProfile{Classification: 75, Ownership: 75, Dependency: 75, Regenerability: 75, PathSafety: 75, ScanCoverage: 75, Freshness: 75}
	view := ExplainView{Kind: ExplainKindResource, Name: "Windows SDK", ConfidenceProfile: &profile}

	var buf bytes.Buffer
	if err := view.RenderText(&buf); err != nil {
		t.Fatalf("RenderText() error = %v", err)
	}
	if bytes.Contains(buf.Bytes(), []byte("Confidence breakdown:")) {
		t.Fatalf("legacy profile with no Assessments must not print a breakdown:\n%s", buf.String())
	}
}
