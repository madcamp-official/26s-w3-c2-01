package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestSummaryViewRenderText(t *testing.T) {
	view := SummaryView{
		Drive: "C:",
		ResourcesByType: []SummaryLine{
			{Label: "Windows SDKs", Bytes: 12459999232},
		},
		SafeReclaimable: 10416967680,
		NeedsReview:     13316730880,
		Blocked:         63146360832,
	}

	var buf bytes.Buffer
	if err := view.RenderText(&buf); err != nil {
		t.Fatalf("RenderText: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"C: drive developer storage", "Windows SDKs", "Safely reclaimable", "Needs review", "Blocked"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q, got:\n%s", want, out)
		}
	}
}

func TestPrinterJSON(t *testing.T) {
	view := SummaryView{
		ResourcesByType: []SummaryLine{{Label: "Windows SDKs", Bytes: 100}},
		SafeReclaimable: 1,
	}

	var buf bytes.Buffer
	if err := New(&buf, true).Print(view); err != nil {
		t.Fatalf("Print: %v", err)
	}

	var decoded SummaryView
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("Unmarshal: %v\noutput: %s", err, buf.String())
	}
	if decoded.SafeReclaimable != view.SafeReclaimable {
		t.Errorf("SafeReclaimable = %d, want %d", decoded.SafeReclaimable, view.SafeReclaimable)
	}
	if len(decoded.ResourcesByType) != 1 || decoded.ResourcesByType[0].Label != "Windows SDKs" {
		t.Errorf("ResourcesByType = %+v, want one line for Windows SDKs", decoded.ResourcesByType)
	}
}

func TestPrinterText(t *testing.T) {
	view := SummaryView{ResourcesByType: []SummaryLine{{Label: "X", Bytes: 1}}}

	var buf bytes.Buffer
	if err := New(&buf, false).Print(view); err != nil {
		t.Fatalf("Print: %v", err)
	}
	if strings.HasPrefix(strings.TrimSpace(buf.String()), "{") {
		t.Errorf("expected text output, got what looks like JSON:\n%s", buf.String())
	}
}
