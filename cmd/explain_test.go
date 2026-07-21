package cmd

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestExplainCommandDescribesResourceWithEvidence(t *testing.T) {
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
	seedWindowsSDKDependency(t, "GameClient")

	out := run("explain", "windows-sdk:10.0.22621.0")
	for _, want := range []string{
		"Resource: Windows SDK 10.0.22621.0",
		"GameClient",
		"Evidence: DECLARED",
		"Property: WindowsTargetPlatformVersion",
		"Rebuild: HIGH",
		"Risk: BLOCKED",
		"Confidence: 75%",
		"Recovery:",
	} {
		if !bytes.Contains(out.Bytes(), []byte(want)) {
			t.Fatalf("explain output missing %q:\n%s", want, out)
		}
	}
}

func TestExplainCommandDescribesProject(t *testing.T) {
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

	out := run("explain", "project:"+project.RootPath)
	for _, want := range []string{
		"Project: GameClient",
		"Uses:",
		"  Requires:",
		"Windows SDK",
		"Evidence: DECLARED",
		"Property: WindowsTargetPlatformVersion",
	} {
		if !bytes.Contains(out.Bytes(), []byte(want)) {
			t.Fatalf("explain project output missing %q:\n%s", want, out)
		}
	}

	// issue #38: project size is now measured (see
	// internal/app/analysis_orchestrator.go), so the line must show a real
	// humanized value instead of the old "—" placeholder.
	if bytes.Contains(out.Bytes(), []byte("Size: —")) {
		t.Fatalf("explain project output must not render the unmeasured-size placeholder:\n%s", out)
	}
}

func TestExplainCommandUnknownTargetErrors(t *testing.T) {
	scanRoot = ""
	cfgPath = ""
	t.Chdir(t.TempDir())

	out := &bytes.Buffer{}
	rootCmd.SetOut(out)
	rootCmd.SetErr(out)
	rootCmd.SetArgs([]string{"init"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute(init) error = %v", err)
	}

	rootCmd.SetArgs([]string{"explain", "does-not-exist"})
	if err := rootCmd.Execute(); err == nil {
		t.Fatalf("Execute(explain does-not-exist) error = nil, want ErrTargetNotFound")
	}
}
