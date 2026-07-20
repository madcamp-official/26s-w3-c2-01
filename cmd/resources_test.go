package cmd

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/app"
)

func TestResourcesCommandListsAndFiltersScannedResources(t *testing.T) {
	scanRoot = ""
	cfgPath = ""
	resourcesType = ""
	resourcesRisk = ""
	// System resource detectors (windows-sdk, dotnet-sdk, visual-studio) read
	// the actual host machine, so a "--type windows-sdk finds nothing" style
	// assertion is only true on hosts without one installed -- it failed on
	// Windows CI runners, which do have real SDKs. Disable them so this test
	// only sees the deterministic resources the Node fixture itself produces,
	// same guard as TestSummaryGoldenNodeFixture.
	previousResourceDetectors := resourceDetectors
	resourceDetectors = func() []app.ResourceDetector { return nil }
	t.Cleanup(func() { resourceDetectors = previousResourceDetectors })

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
	resourcesRisk = ""

	// --type must match case-insensitively like --risk already does (same
	// bug class as finding #8 in docs/libra_review_findings_day4.md, found
	// here too while scoping finding #5): stored resource types are
	// lowercase ("node-modules"), so a differently-cased query is the
	// realistic way a user would type it.
	caseInsensitive := run("resources", "--type", "Node-Modules")
	if !bytes.Contains(caseInsensitive.Bytes(), []byte("node_modules")) {
		t.Fatalf("resources --type Node-Modules = %s, want node_modules (case-insensitive match)", caseInsensitive)
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
