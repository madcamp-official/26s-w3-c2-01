package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestScanViewRenderText(t *testing.T) {
	view := ScanView{
		RootsScanned: 1, ProjectsFound: 7, ResourcesFound: 5, FilesInspected: 17,
		Warnings: []ScanIssue{{Code: "MALFORMED_MANIFEST", Message: "bad json"}},
	}

	var buf bytes.Buffer
	if err := view.RenderText(&buf); err != nil {
		t.Fatalf("RenderText: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"Scan completed",
		"Roots scanned:   1",
		"Projects found:  7",
		"Resources found: 5",
		"Files inspected: 17",
		"Warnings:        1",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q, got:\n%s", want, out)
		}
	}
}

func TestScanViewJSON(t *testing.T) {
	view := ScanView{
		RootsScanned: 2, ProjectsFound: 3,
		Warnings: []ScanIssue{{Code: "ACCESS_DENIED", Path: "/blocked", Message: "permission denied"}},
	}

	var buf bytes.Buffer
	if err := New(&buf, true, "scan").Print(view); err != nil {
		t.Fatalf("Print: %v", err)
	}

	var decoded ScanView
	if _, err := DecodeEnvelope(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("DecodeEnvelope: %v\noutput: %s", err, buf.String())
	}
	if decoded.RootsScanned != 2 || decoded.ProjectsFound != 3 {
		t.Errorf("decoded = %+v, want RootsScanned=2 ProjectsFound=3", decoded)
	}
	if len(decoded.Warnings) != 1 || decoded.Warnings[0].Path != "/blocked" {
		t.Errorf("decoded.Warnings = %+v, want one warning with Path /blocked", decoded.Warnings)
	}
}

// TestScanViewEnvelopeReflectsWarnings is a regression test for issue #59:
// a scan that hit recoverable issues must report OutcomePartial, not
// OutcomeSuccess, and those issues must actually reach the envelope's
// Issues list (not just Warnings, which most --json consumers won't know
// to look for on scan specifically).
func TestScanViewEnvelopeReflectsWarnings(t *testing.T) {
	clean := ScanView{RootsScanned: 1}
	if got := clean.Envelope(); got.Outcome != OutcomeSuccess || len(got.Issues) != 0 {
		t.Errorf("Envelope() for a warning-free scan = %+v, want OutcomeSuccess and no issues", got)
	}

	withWarning := ScanView{
		RootsScanned: 1,
		Warnings:     []ScanIssue{{Code: "ACCESS_DENIED", Phase: "DISCOVER_FILES", Path: "/blocked", Message: "denied"}},
	}
	got := withWarning.Envelope()
	if got.Outcome != OutcomePartial {
		t.Errorf("Outcome = %q, want %q for a scan with warnings", got.Outcome, OutcomePartial)
	}
	if len(got.Issues) != 1 || got.Issues[0].Code != "ACCESS_DENIED" || got.Issues[0].Path != "/blocked" {
		t.Errorf("Issues = %+v, want one ACCESS_DENIED issue for /blocked", got.Issues)
	}
}
