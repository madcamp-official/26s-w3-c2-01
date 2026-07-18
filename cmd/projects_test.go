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
