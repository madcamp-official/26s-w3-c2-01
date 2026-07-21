package cmd

import (
	"bytes"
	"path/filepath"
	"testing"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/app"
	"github.com/madcamp-official/26s-w3-c2-01/internal/output"
	"github.com/madcamp-official/26s-w3-c2-01/internal/store/sqlite"
)

func TestIssuesCommandUsesLatestScanAndFilters(t *testing.T) {
	resetIssueFlags()
	t.Cleanup(resetIssueFlags)
	dir := t.TempDir()
	t.Chdir(dir)
	runIssuesCommand(t, "init")
	db, err := openDatabase()
	if err != nil {
		t.Fatal(err)
	}
	scans := sqlite.NewScanRepository(db)
	base := time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
	for i, id := range []string{"scan-old", "scan-new"} {
		if err := scans.Save(t.Context(), app.ScanRecord{ID: id, StartedAt: base.Add(time.Duration(i) * time.Hour), Roots: []string{"/projects"}, Status: app.ScanStatusCompletedWithErrors}); err != nil {
			t.Fatal(err)
		}
	}
	repository := sqlite.NewScanIssueRepository(db)
	if err := repository.Replace(t.Context(), "scan-new", []app.Issue{
		{Code: app.IssueAccessDenied, Phase: app.PhaseDiscoverFiles, Path: "/private", Operation: "read", Severity: app.IssueWarning, Message: "denied"},
		{Code: app.IssueAdapterFailed, Phase: app.PhaseResolveDependencies, Severity: app.IssueError, Message: "failed"},
	}); err != nil {
		t.Fatal(err)
	}
	db.Close()

	out := runIssuesCommand(t, "issues", "--severity", "warning")
	if !bytes.Contains(out.Bytes(), []byte("scan-new")) || !bytes.Contains(out.Bytes(), []byte("ACCESS_DENIED")) || bytes.Contains(out.Bytes(), []byte("ADAPTER_FAILED")) {
		t.Fatalf("filtered output:\n%s", out)
	}

	issuesSeverity = ""
	out = runIssuesCommand(t, "issues", "--scan", "scan-old", "--json")
	var view struct {
		ScanID string `json:"scan_id"`
		Issues []any  `json:"issues"`
	}
	if _, err := output.DecodeEnvelope(out.Bytes(), &view); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, out)
	}
	if view.ScanID != "scan-old" || len(view.Issues) != 0 {
		t.Fatalf("JSON view = %#v", view)
	}
}

func TestScanPersistsIssuesForLaterCommand(t *testing.T) {
	resetIssueFlags()
	t.Cleanup(resetIssueFlags)
	scanRoot = ""
	cfgPath = ""
	previousResourceDetectors := resourceDetectors
	resourceDetectors = func() []app.ResourceDetector { return nil }
	t.Cleanup(func() { resourceDetectors = previousResourceDetectors })
	fixture, err := filepath.Abs("../testdata/node")
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	runIssuesCommand(t, "init")
	runIssuesCommand(t, "scan", "--root", fixture)
	out := runIssuesCommand(t, "issues", "--code", "MALFORMED_MANIFEST")
	if !bytes.Contains(out.Bytes(), []byte("MALFORMED_MANIFEST")) || !bytes.Contains(out.Bytes(), []byte("package.json")) {
		t.Fatalf("issues output:\n%s", out)
	}
}

func runIssuesCommand(t *testing.T, args ...string) *bytes.Buffer {
	t.Helper()
	out := &bytes.Buffer{}
	rootCmd.SetOut(out)
	rootCmd.SetErr(out)
	rootCmd.SetArgs(args)
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute(%v) error = %v; output=%s", args, err, out)
	}
	return out
}

func resetIssueFlags() {
	issuesScanID = ""
	issuesCode = ""
	issuesSeverity = ""
	jsonOutput = false
	cfgPath = ""
}
