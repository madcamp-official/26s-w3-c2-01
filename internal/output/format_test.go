package output

import (
	"encoding/json"
	"io"
	"strings"
	"testing"
)

// minimalView is the smallest possible Renderable, used to test the
// envelope machinery in isolation from any real command's shape.
type minimalView struct {
	Value string `json:"value"`
}

func (v minimalView) RenderText(w io.Writer) error {
	_, err := w.Write([]byte(v.Value))
	return err
}

// TestPrintEncodesSharedEnvelope is a regression test for issue #59: --json
// output used to be the bare view with no common shape across commands.
// Every field the contract (docs/libra_integration_contracts.md §13)
// requires must now be present, and Issues/Unverified must be "[]", never
// null, so a JSON consumer never has to special-case a missing array.
func TestPrintEncodesSharedEnvelope(t *testing.T) {
	var buf strings.Builder
	if err := New(&buf, true, "widgets").Print(minimalView{Value: "hi"}); err != nil {
		t.Fatalf("Print: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(buf.String()), &raw); err != nil {
		t.Fatalf("Unmarshal: %v\noutput: %s", err, buf.String())
	}
	for _, field := range []string{"command", "schema_version", "outcome", "data", "issues", "unverified"} {
		if _, ok := raw[field]; !ok {
			t.Errorf("envelope missing %q field, got:\n%s", field, buf.String())
		}
	}
	if !strings.Contains(buf.String(), `"issues": []`) {
		t.Errorf("envelope issues must be an empty array, not null/omitted, got:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), `"unverified": []`) {
		t.Errorf("envelope unverified must be an empty array, not null/omitted, got:\n%s", buf.String())
	}
}

// TestPrintDefaultsToSuccessOutcome confirms the plain Print path (used by
// every command with no partial-success concept of its own) always reports
// OutcomeSuccess -- PrintEnvelope with an explicit Outcome is opt-in.
func TestPrintDefaultsToSuccessOutcome(t *testing.T) {
	var buf strings.Builder
	if err := New(&buf, true, "widgets").Print(minimalView{Value: "hi"}); err != nil {
		t.Fatalf("Print: %v", err)
	}
	var env struct {
		Outcome Outcome `json:"outcome"`
	}
	if err := json.Unmarshal([]byte(buf.String()), &env); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if env.Outcome != OutcomeSuccess {
		t.Errorf("outcome = %q, want %q", env.Outcome, OutcomeSuccess)
	}
}

// TestPrintEnvelopeCarriesExplicitOutcomeAndIssues confirms PrintEnvelope's
// opt-in path actually reaches the wire, for the commands that call it
// (scan/issues/clean/purge/restore -- see #59).
func TestPrintEnvelopeCarriesExplicitOutcomeAndIssues(t *testing.T) {
	var buf strings.Builder
	p := New(&buf, true, "widgets")
	err := p.PrintEnvelope(minimalView{Value: "hi"}, EnvelopeOptions{
		Outcome: OutcomePartial,
		Issues:  []EnvelopeIssue{{Code: "SOME_CODE", Message: "something happened"}},
	})
	if err != nil {
		t.Fatalf("PrintEnvelope: %v", err)
	}

	var decoded minimalView
	env, err := DecodeEnvelope([]byte(buf.String()), &decoded)
	if err != nil {
		t.Fatalf("DecodeEnvelope: %v\noutput: %s", err, buf.String())
	}
	if env.Command != "widgets" {
		t.Errorf("command = %q, want %q", env.Command, "widgets")
	}
	if env.SchemaVersion != EnvelopeSchemaVersion {
		t.Errorf("schema_version = %d, want %d", env.SchemaVersion, EnvelopeSchemaVersion)
	}
	if env.Outcome != OutcomePartial {
		t.Errorf("outcome = %q, want %q", env.Outcome, OutcomePartial)
	}
	if len(env.Issues) != 1 || env.Issues[0].Code != "SOME_CODE" {
		t.Errorf("issues = %#v, want one SOME_CODE issue", env.Issues)
	}
	if decoded.Value != "hi" {
		t.Errorf("decoded data = %#v, want Value=hi", decoded)
	}
}

// TestPrintTextModeIgnoresEnvelope confirms text mode is unaffected by any
// of this -- RenderText is called directly, no envelope wrapping.
func TestPrintTextModeIgnoresEnvelope(t *testing.T) {
	var buf strings.Builder
	if err := New(&buf, false, "widgets").Print(minimalView{Value: "hi"}); err != nil {
		t.Fatalf("Print: %v", err)
	}
	if buf.String() != "hi" {
		t.Errorf("text output = %q, want exactly the RenderText result %q", buf.String(), "hi")
	}
}
