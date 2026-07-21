package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProjectsCommandListsAndFiltersScannedProjects(t *testing.T) {
	scanRoot = ""
	cfgPath = ""
	projectsType = ""
	projectsDrive = ""
	projectsStatus = ""

	fixture, err := filepath.Abs("../testdata/node")
	if err != nil {
		t.Fatalf("resolve fixture path: %v", err)
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

	run("init")
	run("scan", "--root", fixture)

	all := run("projects")
	for _, want := range []string{"sample-app", "workspace-root", "app-a", "app-b", "lib"} {
		if !bytes.Contains(all.Bytes(), []byte(want)) {
			t.Fatalf("projects output missing %q:\n%s", want, all)
		}
	}

	filtered := run("projects", "--type", "msbuild-cpp")
	if !bytes.Contains(filtered.Bytes(), []byte("No projects found")) {
		t.Fatalf("projects --type msbuild-cpp = %s, want no matches", filtered)
	}

	// --type must match case-insensitively like --drive/--status already do
	// (finding #8 in docs/libra_review_findings_day4.md): stored project
	// types are lowercase ("node"), so a differently-cased query is the
	// realistic way a user would type it.
	caseInsensitive := run("projects", "--type", "Node")
	if !bytes.Contains(caseInsensitive.Bytes(), []byte("sample-app")) {
		t.Fatalf("projects --type Node = %s, want sample-app (case-insensitive match)", caseInsensitive)
	}

	// issue #38: AnalysisOrchestrator now measures BuildProject.LogicalSize
	// (see internal/app/analysis_orchestrator.go), so the SIZE column must
	// show a real humanized value instead of the old "—" placeholder.
	if bytes.Contains(all.Bytes(), []byte("—")) {
		t.Fatalf("projects output must not render the unmeasured-size placeholder:\n%s", all)
	}
}

// writeManyNodeProjects builds n minimal, independent Node projects
// (root/project-000, root/project-001, ...) under a fresh temp directory and
// returns its path, so tests can exercise `projects`' default display limit
// (issue #41) without depending on testdata/node's fixed 7-project fixture.
// Modified times step backwards one hour per project (project-000 newest)
// so --sort modified has something deterministic to sort by.
func writeManyNodeProjects(t *testing.T, n int) string {
	t.Helper()
	root := t.TempDir()
	base := time.Now()
	for i := range n {
		dir := filepath.Join(root, fmt.Sprintf("project-%03d", i))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		manifest := filepath.Join(dir, "package.json")
		if err := os.WriteFile(manifest, fmt.Appendf(nil, `{"name":"project-%03d"}`, i), 0o644); err != nil {
			t.Fatal(err)
		}
		// The Node detector's LastModifiedAt comes from the scanner Entry for
		// the project *directory* (see NodeProjectDetector.Observe), not the
		// manifest file inside it -- Chtimes must target dir, not manifest.
		modTime := base.Add(-time.Duration(i) * time.Hour)
		if err := os.Chtimes(dir, modTime, modTime); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// TestProjectsCommandDefaultLimitAndAllFlag covers issue #41: an unfiltered
// `projects` on a real machine can report hundreds of rows. Default output
// is capped at projectsDefaultLimit with a footnote saying so; --all lifts
// the cap.
func TestProjectsCommandDefaultLimitAndAllFlag(t *testing.T) {
	scanRoot = ""
	cfgPath = ""
	projectsType = ""
	projectsDrive = ""
	projectsStatus = ""
	projectsName = ""
	projectsUnder = ""
	projectsSort = ""
	projectsAll = false
	t.Cleanup(func() { projectsAll = false })

	fixture := writeManyNodeProjects(t, projectsDefaultLimit+5)
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

	run("init")
	run("scan", "--root", fixture)

	// Count NAME-column occurrences only (line starts with "project-"):
	// each row also repeats the name in the PATH column
	// (".../project-000/project-000"), so a raw substring count double-counts.
	rowCount := func(s string) int {
		n := 0
		for _, line := range strings.Split(s, "\n") {
			if strings.HasPrefix(line, "project-") {
				n++
			}
		}
		return n
	}

	capped := run("projects")
	if got := rowCount(capped.String()); got != projectsDefaultLimit {
		t.Fatalf("default projects output shows %d rows, want the %d-row limit:\n%s", got, projectsDefaultLimit, capped)
	}
	if !strings.Contains(capped.String(), "Use --all to see the rest.") {
		t.Fatalf("default projects output missing the truncation footnote:\n%s", capped)
	}

	all := run("projects", "--all")
	if got := rowCount(all.String()); got != projectsDefaultLimit+5 {
		t.Fatalf("projects --all shows %d rows, want all %d:\n%s", got, projectsDefaultLimit+5, all)
	}
	if strings.Contains(all.String(), "Use --all to see the rest.") {
		t.Fatalf("projects --all should not print the truncation footnote:\n%s", all)
	}
}

// TestProjectsCommandNameUnderAndSortFilters covers issue #41's --name,
// --under, and --sort modified flags together against a small, deterministic
// fixture (well under projectsDefaultLimit, so truncation doesn't interact
// with these assertions).
func TestProjectsCommandNameUnderAndSortFilters(t *testing.T) {
	scanRoot = ""
	cfgPath = ""
	projectsType = ""
	projectsDrive = ""
	projectsStatus = ""
	projectsName = ""
	projectsUnder = ""
	projectsSort = ""
	projectsAll = false
	t.Cleanup(func() {
		projectsName = ""
		projectsUnder = ""
		projectsSort = ""
	})

	fixture, err := filepath.Abs("../testdata/node")
	if err != nil {
		t.Fatalf("resolve fixture path: %v", err)
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

	run("init")
	run("scan", "--root", fixture)

	named := run("projects", "--name", "app-a")
	if !strings.Contains(named.String(), "app-a") || strings.Contains(named.String(), "app-b") {
		t.Fatalf("projects --name app-a = %s, want only app-a", named)
	}
	// run() only overwrites flags explicitly passed in each call -- pflag
	// leaves a package-level flag var at whatever the previous Execute() set
	// it to, so --name from the call above must be reset explicitly or it
	// silently keeps filtering the --under assertion below to just app-a.
	projectsName = ""

	under := run("projects", "--under", filepath.Join(fixture, "workspace-npm"))
	for _, want := range []string{"app-a", "app-b"} {
		if !strings.Contains(under.String(), want) {
			t.Fatalf("projects --under workspace-npm missing %q:\n%s", want, under)
		}
	}
	if strings.Contains(under.String(), "sample-app") {
		t.Fatalf("projects --under workspace-npm should exclude sample-app (outside that root):\n%s", under)
	}

	invalid := &bytes.Buffer{}
	rootCmd.SetOut(invalid)
	rootCmd.SetErr(invalid)
	rootCmd.SetArgs([]string{"projects", "--sort", "bogus"})
	if err := rootCmd.Execute(); err == nil {
		t.Fatal("Execute(projects --sort bogus) error = nil, want a validation error")
	}
}

// TestProjectsCommandSortModifiedOrdersNewestFirst uses
// writeManyNodeProjects' deterministic, explicitly staggered mtimes (rather
// than testdata/node's real, uncontrolled filesystem timestamps) so the
// expected order is exact, not just "both rows appear somewhere".
func TestProjectsCommandSortModifiedOrdersNewestFirst(t *testing.T) {
	scanRoot = ""
	cfgPath = ""
	projectsType = ""
	projectsDrive = ""
	projectsStatus = ""
	projectsName = ""
	projectsUnder = ""
	projectsSort = ""
	projectsAll = false
	t.Cleanup(func() { projectsSort = "" })

	fixture := writeManyNodeProjects(t, 3) // project-000 newest ... project-002 oldest
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

	run("init")
	run("scan", "--root", fixture)
	out := run("projects", "--sort", "modified").String()

	newest := strings.Index(out, "project-000")
	oldest := strings.Index(out, "project-002")
	if newest == -1 || oldest == -1 {
		t.Fatalf("projects --sort modified missing expected rows:\n%s", out)
	}
	if newest > oldest {
		t.Fatalf("projects --sort modified = %s, want project-000 (newest) before project-002 (oldest)", out)
	}
}

// TestProjectsCommandSortSizeOrdersLargestFirst covers --sort size, which
// only became meaningful after AnalysisOrchestrator started measuring
// BuildProject.LogicalSize (issue #38's actual fix, landed after this PR was
// first opened) -- it was deliberately left out at first since every project
// tied at 0 (see the removed doc comment on projectsDefaultLimit). Each
// project's package.json is padded to a distinct, controlled size so the
// measured LogicalSize order is deterministic.
func TestProjectsCommandSortSizeOrdersLargestFirst(t *testing.T) {
	scanRoot = ""
	cfgPath = ""
	projectsType = ""
	projectsDrive = ""
	projectsStatus = ""
	projectsName = ""
	projectsUnder = ""
	projectsSort = ""
	projectsAll = false
	t.Cleanup(func() { projectsSort = "" })

	root := t.TempDir()
	writeSizedProject := func(name string, paddingBytes int) {
		t.Helper()
		dir := filepath.Join(root, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		manifest := fmt.Sprintf(`{"name":%q,"_pad":%q}`, name, strings.Repeat("x", paddingBytes))
		if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(manifest), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeSizedProject("small-project", 10)
	writeSizedProject("large-project", 5000)
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

	run("init")
	run("scan", "--root", root)
	out := run("projects", "--sort", "size").String()

	largest := strings.Index(out, "large-project")
	smallest := strings.Index(out, "small-project")
	if largest == -1 || smallest == -1 {
		t.Fatalf("projects --sort size missing expected rows:\n%s", out)
	}
	if largest > smallest {
		t.Fatalf("projects --sort size = %s, want large-project before small-project", out)
	}
}

func TestProjectsCommandReportsNoProjectsBeforeScan(t *testing.T) {
	scanRoot = ""
	cfgPath = ""
	projectsType = ""
	projectsDrive = ""
	projectsStatus = ""
	t.Chdir(t.TempDir())

	out := &bytes.Buffer{}
	rootCmd.SetOut(out)
	rootCmd.SetErr(out)
	rootCmd.SetArgs([]string{"init"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute(init) error = %v", err)
	}

	out.Reset()
	rootCmd.SetArgs([]string{"projects"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute(projects) error = %v", err)
	}
	if !bytes.Contains(out.Bytes(), []byte("No projects found")) {
		t.Fatalf("projects output = %s, want no-projects message", out)
	}
}
