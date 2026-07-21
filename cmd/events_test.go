package cmd

import (
	"bytes"

	"github.com/madcamp-official/26s-w3-c2-01/internal/eventlog"
	"github.com/madcamp-official/26s-w3-c2-01/internal/output"
	"path/filepath"
	"testing"
	"time"
)

func TestEventsCommandFiltersAndLimitsNewestEvents(t *testing.T) {
	dir := t.TempDir()
	cfgPath = filepath.Join(dir, ".libra.yaml")
	jsonOutput = true
	eventsKind = ""
	eventsSince = ""
	eventsLimit = 50
	t.Cleanup(func() { cfgPath = ""; jsonOutput = false; eventsKind = ""; eventsSince = ""; eventsLimit = 50 })

	initOut := &bytes.Buffer{}
	rootCmd.SetOut(initOut)
	rootCmd.SetErr(initOut)
	rootCmd.SetArgs([]string{"init"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute(init) error = %v; output=%s", err, initOut)
	}

	now := time.Now().UTC()
	for _, event := range []eventlog.Event{{At: now.Add(-2 * time.Hour), Kind: "DAEMON_STARTED"}, {At: now.Add(-time.Hour), Kind: "RESOURCE_DIRTY"}, {At: now, Kind: "RESOURCE_DIRTY", Error: "scan failed"}} {
		if err := eventlog.Append(daemonEventPath(), event); err != nil {
			t.Fatal(err)
		}
	}
	out := &bytes.Buffer{}
	rootCmd.SetOut(out)
	rootCmd.SetErr(out)
	rootCmd.SetArgs([]string{"events", "--kind", "resource_dirty", "--limit", "1", "--json"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var view output.EventsView
	envelope, err := output.DecodeEnvelope(out.Bytes(), &view)
	if err != nil {
		t.Fatal(err)
	}
	if envelope.Command != "events" || envelope.Outcome != output.OutcomeSuccess {
		t.Fatalf("envelope = %#v", envelope)
	}
	if len(view.Events) != 1 || view.Events[0].Error != "scan failed" {
		t.Fatalf("events = %#v", view.Events)
	}
}
func TestParseEventsSinceDuration(t *testing.T) {
	now := time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
	got, err := parseEventsSince("24h", now)
	if err != nil || !got.Equal(now.Add(-24*time.Hour)) {
		t.Fatalf("got %v, %v", got, err)
	}
}
