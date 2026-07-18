package cmd

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestSummaryCommandReflectsScannedProjects(t *testing.T) {
	scanRoot = ""
	cfgPath = ""
	summaryType = ""
	summaryDrive = ""

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
	out := run("summary")
	if !bytes.Contains(out.Bytes(), []byte("Projects            7")) {
		t.Fatalf("summary output missing project count:\n%s", out)
	}
}

func TestSummaryCommandBeforeScanIsAllZero(t *testing.T) {
	scanRoot = ""
	cfgPath = ""
	summaryType = ""
	summaryDrive = ""
	t.Chdir(t.TempDir())

	out := &bytes.Buffer{}
	rootCmd.SetOut(out)
	rootCmd.SetErr(out)
	rootCmd.SetArgs([]string{"init"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute(init) error = %v", err)
	}

	out.Reset()
	rootCmd.SetArgs([]string{"summary"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute(summary) error = %v", err)
	}
	if !bytes.Contains(out.Bytes(), []byte("Projects            0")) {
		t.Fatalf("summary output = %s, want zero projects", out)
	}
}
