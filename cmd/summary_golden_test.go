package cmd

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

// updateGolden regenerates the .golden fixtures instead of comparing against
// them: `go test ./cmd/ -run Golden -update`.
var updateGolden = flag.Bool("update", false, "update golden output files")

// TestSummaryGoldenNodeFixture locks down `libra summary` output for the
// committed testdata/node fixture tree. The output is deterministic: byte
// counts come only from committed fixture files, and summary prints no dates
// or absolute paths. If the output legitimately changes (e.g. a risk-policy
// update reclassifies node_modules), regenerate with -update and review the
// diff.
func TestSummaryGoldenNodeFixture(t *testing.T) {
	scanRoot = ""
	cfgPath = ""
	summaryType = ""
	summaryDrive = ""

	fixture, err := filepath.Abs("../testdata/node")
	if err != nil {
		t.Fatalf("resolve fixture path: %v", err)
	}
	// Resolve the golden path before chdir, so it stays anchored to the repo
	// rather than the temp working directory.
	goldenPath, err := filepath.Abs(filepath.Join("..", "testdata", "golden", "summary_node.golden"))
	if err != nil {
		t.Fatalf("resolve golden path: %v", err)
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
	got := run("summary").Bytes()

	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("create golden dir: %v", err)
		}
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden (run with -update to create): %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("summary output does not match golden.\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}
