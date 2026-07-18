package cmd

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestResourcesCommandListsAndFiltersScannedResources(t *testing.T) {
	scanRoot = ""
	cfgPath = ""
	resourcesType = ""
	resourcesRisk = ""

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

	all := run("resources")
	for _, want := range []string{"node_modules", "dist"} {
		if !bytes.Contains(all.Bytes(), []byte(want)) {
			t.Fatalf("resources output missing %q:\n%s", want, all)
		}
	}

	filtered := run("resources", "--type", "windows-sdk")
	if !bytes.Contains(filtered.Bytes(), []byte("No resources found")) {
		t.Fatalf("resources --type windows-sdk = %s, want no matches", filtered)
	}
	resourcesType = ""

	byRisk := run("resources", "--risk", "review")
	if !bytes.Contains(byRisk.Bytes(), []byte("node_modules")) {
		t.Fatalf("resources --risk review = %s, want node_modules", byRisk)
	}
}

func TestResourcesCommandReportsNoResourcesBeforeScan(t *testing.T) {
	scanRoot = ""
	cfgPath = ""
	resourcesType = ""
	resourcesRisk = ""
	t.Chdir(t.TempDir())

	out := &bytes.Buffer{}
	rootCmd.SetOut(out)
	rootCmd.SetErr(out)
	rootCmd.SetArgs([]string{"init"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute(init) error = %v", err)
	}

	out.Reset()
	rootCmd.SetArgs([]string{"resources"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute(resources) error = %v", err)
	}
	if !bytes.Contains(out.Bytes(), []byte("No resources found")) {
		t.Fatalf("resources output = %s, want no-resources message", out)
	}
}
