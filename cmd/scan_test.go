package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/app"
	"github.com/madcamp-official/26s-w3-c2-01/internal/store/sqlite"
)

func TestScanCommandDetectsAndPersistsNodeProjects(t *testing.T) {
	scanRoot = ""
	cfgPath = ""

	fixture, err := filepath.Abs("../testdata/node")
	if err != nil {
		t.Fatalf("resolve fixture path: %v", err)
	}

	dir := t.TempDir()
	t.Chdir(dir)

	out := &bytes.Buffer{}
	rootCmd.SetOut(out)
	rootCmd.SetErr(out)
	rootCmd.SetArgs([]string{"scan", "--root", fixture})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v; output=%s", err, out)
	}
	if !bytes.Contains(out.Bytes(), []byte("Projects found:  7")) {
		t.Fatalf("unexpected output:\n%s", out)
	}

	db, err := sqlite.Open(filepath.Join(dir, ".libra.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM projects WHERE project_type = 'node'").Scan(&count); err != nil {
		t.Fatalf("count projects: %v", err)
	}
	if count != 7 {
		t.Fatalf("persisted node projects = %d, want 7", count)
	}
}

func TestScanCommandFullFlagIsDeprecatedButHarmless(t *testing.T) {
	scanRoot = ""
	cfgPath = ""

	fixture, err := filepath.Abs("../testdata/node/basic")
	if err != nil {
		t.Fatalf("resolve fixture path: %v", err)
	}
	t.Chdir(t.TempDir())

	out := &bytes.Buffer{}
	rootCmd.SetOut(out)
	rootCmd.SetErr(out)
	rootCmd.SetArgs([]string{"scan", "--root", fixture, "--full"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v; output=%s", err, out)
	}
	if !bytes.Contains(out.Bytes(), []byte("deprecated")) {
		t.Fatalf("output missing deprecation notice:\n%s", out)
	}
	if !bytes.Contains(out.Bytes(), []byte("Scan completed")) {
		t.Fatalf("--full should not prevent the scan from completing:\n%s", out)
	}
}

// TestScanCommandSupportsJSON covers issue #42: scan was the only command
// left ignoring the shared --json flag entirely (it always printed plain
// text, regardless of jsonOutput), unlike every other command's
// output.Renderable + --json pair. This checks the JSON actually decodes
// and carries full per-warning detail (code/path/message), not just a
// count -- the point of exposing Warnings as a real list instead of an int.
func TestScanCommandSupportsJSON(t *testing.T) {
	scanRoot = ""
	cfgPath = ""
	jsonOutput = false
	// rootCmd's --json flag is bound to the package-level jsonOutput var, so
	// passing --json below leaves it at true for every test that runs after
	// this one in the same package unless explicitly reset -- the same class
	// of stale-flag bug fixed for projectsName in issue #41.
	t.Cleanup(func() { jsonOutput = false })
	previousResourceDetectors := resourceDetectors
	resourceDetectors = func() []app.ResourceDetector { return nil }
	t.Cleanup(func() { resourceDetectors = previousResourceDetectors })

	fixture, err := filepath.Abs("../testdata/node")
	if err != nil {
		t.Fatalf("resolve fixture path: %v", err)
	}
	t.Chdir(t.TempDir())

	out := &bytes.Buffer{}
	rootCmd.SetOut(out)
	rootCmd.SetErr(out)
	rootCmd.SetArgs([]string{"scan", "--root", fixture, "--json"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v; output=%s", err, out)
	}

	var view struct {
		RootsScanned   int   `json:"roots_scanned"`
		ProjectsFound  int   `json:"projects_found"`
		ResourcesFound int   `json:"resources_found"`
		FilesInspected int64 `json:"files_inspected"`
		Warnings       []struct {
			Code    string `json:"code"`
			Path    string `json:"path"`
			Message string `json:"message"`
		} `json:"warnings"`
	}
	if err := json.Unmarshal(out.Bytes(), &view); err != nil {
		t.Fatalf("unmarshal scan --json output: %v\n%s", err, out)
	}
	if view.ProjectsFound != 7 {
		t.Fatalf("projects_found = %d, want 7", view.ProjectsFound)
	}
	if len(view.Warnings) != 1 {
		t.Fatalf("warnings = %#v, want exactly 1 (the malformed-package-json fixture)", view.Warnings)
	}
	if view.Warnings[0].Code != "MALFORMED_MANIFEST" || view.Warnings[0].Path == "" || view.Warnings[0].Message == "" {
		t.Fatalf("warnings[0] = %#v, want a populated MALFORMED_MANIFEST issue", view.Warnings[0])
	}
}

// TestScanCommandSummarizesIssuesByDefaultAndShowsAllWithVerbose covers
// issue #37: `scan` used to print only "Warnings: N", never the underlying
// path or cause, leaving the user unable to tell whether a result like
// "Safely reclaimable" reflects a complete scan. This exercises the fix
// entirely through the real scan -> in-memory result.Issues -> stdout path
// (no new persistence).
//
// The fixture builds 4 malformed package.json directories itself, rather
// than reusing testdata/node's single malformed-package-json fixture,
// specifically to control the issue *count* past scanIssueSummaryLimit (3)
// deterministically -- testdata/node's own warning count is platform
// dependent (Windows SDK/.NET/Visual Studio detection only run on Windows;
// see internal/adapter/windowsdk, dotnet, msbuild), so asserting "more than
// 3 warnings" against it would pass on macOS and fail in Windows CI.
func TestScanCommandSummarizesIssuesByDefaultAndShowsAllWithVerbose(t *testing.T) {
	scanRoot = ""
	cfgPath = ""
	verbose = false
	t.Cleanup(func() { verbose = false })

	// Stub out system resource detection (Windows SDK/.NET/Visual Studio):
	// on a non-Windows dev machine these each add their own UNSUPPORTED_
	// PLATFORM issue, which would make the total issue count -- and so which
	// issues fall inside/outside scanIssueSummaryLimit -- depend on the OS
	// running the test. See cmd/resources_test.go for the same pattern.
	previousResourceDetectors := resourceDetectors
	resourceDetectors = func() []app.ResourceDetector { return nil }
	t.Cleanup(func() { resourceDetectors = previousResourceDetectors })
	root := t.TempDir()
	for i := 1; i <= 4; i++ {
		dir := filepath.Join(root, fmt.Sprintf("malformed-%d", i))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte("{ not valid json"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(t.TempDir())

	run := func(args ...string) *bytes.Buffer {
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

	summary := run("scan", "--root", root)
	if !bytes.Contains(summary.Bytes(), []byte("Warnings:        4")) {
		t.Fatalf("scan output missing warning count:\n%s", summary)
	}
	if got := strings.Count(summary.String(), "MALFORMED_MANIFEST"); got != scanIssueSummaryLimit {
		t.Fatalf("default scan output shows %d issue lines, want the %d-issue summary limit:\n%s", got, scanIssueSummaryLimit, summary)
	}
	if !bytes.Contains(summary.Bytes(), []byte("...and 1 more (use --verbose to see all)")) {
		t.Fatalf("default scan output missing the hidden-issue count:\n%s", summary)
	}
	// The path of the surfaced issue must actually be visible -- the whole
	// point of #37 is that "Warnings: N" alone doesn't say where.
	if !bytes.Contains(summary.Bytes(), []byte(filepath.Join(root, "malformed-1"))) {
		t.Fatalf("default scan output missing the offending path:\n%s", summary)
	}

	verbose = true
	full := run("scan", "--root", root)
	if got := strings.Count(full.String(), "MALFORMED_MANIFEST"); got != 4 {
		t.Fatalf("--verbose scan output shows %d issue lines, want all 4:\n%s", got, full)
	}
	if bytes.Contains(full.Bytes(), []byte("more (use --verbose")) {
		t.Fatalf("--verbose scan output should not truncate:\n%s", full)
	}
}

// TestScanCommandPrintsNextActionHint covers issue #41: scan reported what
// it found but never suggested what to do with the result.
func TestScanCommandPrintsNextActionHint(t *testing.T) {
	scanRoot = ""
	cfgPath = ""

	fixture, err := filepath.Abs("../testdata/node/basic")
	if err != nil {
		t.Fatalf("resolve fixture path: %v", err)
	}
	t.Chdir(t.TempDir())

	out := &bytes.Buffer{}
	rootCmd.SetOut(out)
	rootCmd.SetErr(out)
	rootCmd.SetArgs([]string{"scan", "--root", fixture})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v; output=%s", err, out)
	}
	if !bytes.Contains(out.Bytes(), []byte("Next: libra summary")) {
		t.Fatalf("scan output missing next-action hint:\n%s", out)
	}
}

func TestScanCommandRequiresRootsWithoutConfig(t *testing.T) {
	scanRoot = ""
	cfgPath = ""
	t.Chdir(t.TempDir())

	out := &bytes.Buffer{}
	rootCmd.SetOut(out)
	rootCmd.SetErr(out)
	rootCmd.SetArgs([]string{"scan"})
	if err := rootCmd.Execute(); err == nil {
		t.Fatal("Execute() error = nil, want an error about missing project roots")
	}
}
