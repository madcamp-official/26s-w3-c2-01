package cmd

import (
	"bytes"
	"path/filepath"
	"testing"
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

	// issue #38: no project detector ever measures BuildProject.LogicalSize,
	// so it is always 0 -- rendering that as "0 B" reads as "measured empty"
	// rather than "not measured", which is what it actually means. The SIZE
	// column must show the "not measured" placeholder instead, for every
	// project, with a footnote explaining why.
	if bytes.Contains(all.Bytes(), []byte("0 B")) {
		t.Fatalf("projects output must not render unmeasured project size as \"0 B\":\n%s", all)
	}
	if !bytes.Contains(all.Bytes(), []byte("not measured")) {
		t.Fatalf("projects output missing a footnote explaining unmeasured SIZE:\n%s", all)
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
