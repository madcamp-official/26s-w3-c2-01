package cmd

import (
	"bytes"
	"encoding/json"
	"path/filepath"
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
