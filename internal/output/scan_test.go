package output

import (
	"bytes"
	"encoding/json"
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
	if err := New(&buf, true).Print(view); err != nil {
		t.Fatalf("Print: %v", err)
	}

	var decoded ScanView
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("Unmarshal: %v\noutput: %s", err, buf.String())
	}
	if decoded.RootsScanned != 2 || decoded.ProjectsFound != 3 {
		t.Errorf("decoded = %+v, want RootsScanned=2 ProjectsFound=3", decoded)
	}
	if len(decoded.Warnings) != 1 || decoded.Warnings[0].Path != "/blocked" {
		t.Errorf("decoded.Warnings = %+v, want one warning with Path /blocked", decoded.Warnings)
	}
}
