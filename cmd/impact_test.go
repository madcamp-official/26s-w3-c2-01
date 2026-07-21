package cmd

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestImpactCommandReportsAffectedProject(t *testing.T) {
	scanRoot = ""
	cfgPath = ""

	fixture, err := filepath.Abs("../testdata/msbuild")
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
	_, project := seedWindowsSDKDependency(t, "GameClient")

	out := run("impact", "windows-sdk:10.0.22621.0")
	if !bytes.Contains(out.Bytes(), []byte("Affected projects: 1")) {
		t.Fatalf("impact output missing affected count:\n%s", out)
	}
	for _, want := range []string{
		project.RootPath,
		"RUN",
		"likely unaffected",
		"BUILD",
		"expected to fail",
		"DEBUG",
		"expected to fail if build runs",
		"RESTORE",
		"reinstall via the Visual Studio Installer",
	} {
		if !bytes.Contains(out.Bytes(), []byte(want)) {
			t.Fatalf("impact output missing %q:\n%s", want, out)
		}
	}
}

func TestImpactCommandNoDependentsIsZero(t *testing.T) {
	scanRoot = ""
	cfgPath = ""

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
	out := run("impact", filepath.Join(fixture, "basic", "node_modules"))
	if !bytes.Contains(out.Bytes(), []byte("Affected projects: 0")) {
		t.Fatalf("impact output = %s, want zero affected projects", out)
	}
}

func TestImpactCommandRejectsProjectTarget(t *testing.T) {
	scanRoot = ""
	cfgPath = ""

	fixture, err := filepath.Abs("../testdata/msbuild")
	if err != nil {
		t.Fatalf("resolve fixture path: %v", err)
	}
	t.Chdir(t.TempDir())

	rootCmd.SetOut(&bytes.Buffer{})
	rootCmd.SetErr(&bytes.Buffer{})
	rootCmd.SetArgs([]string{"init"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute(init) error = %v", err)
	}

	rootCmd.SetArgs([]string{"scan", "--root", fixture})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute(scan) error = %v", err)
	}

	rootCmd.SetArgs([]string{"impact", "project:GameClient"})
	if err := rootCmd.Execute(); err == nil {
		t.Fatalf("Execute(impact project:GameClient) error = nil, want error")
	}
}
