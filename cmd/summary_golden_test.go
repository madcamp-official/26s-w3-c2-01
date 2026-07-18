package cmd

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/app"
)

// updateGolden regenerates the .golden fixtures instead of comparing against
// them: `go test ./cmd/ -run Golden -update`.
var updateGolden = flag.Bool("update", false, "update golden output files")

// TestSummaryGoldenNodeFixture locks down `libra summary` output for the
// committed testdata/node fixture tree. System resource detectors are disabled
// so the result does not depend on SDKs installed on the test host. Byte values
// are normalized because Git line-ending conversion can change fixture sizes
// across platforms. If the output shape legitimately changes, regenerate with
// -update and review the diff.
func TestSummaryGoldenNodeFixture(t *testing.T) {
	scanRoot = ""
	cfgPath = ""
	summaryType = ""
	summaryDrive = ""
	previousResourceDetectors := resourceDetectors
	resourceDetectors = func() []app.ResourceDetector { return nil }
	t.Cleanup(func() { resourceDetectors = previousResourceDetectors })

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
	got := normalizeSummaryGolden(run("summary").String())

	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("create golden dir: %v", err)
		}
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden (run with -update to create): %v", err)
	}
	wantText := strings.ReplaceAll(string(want), "\r\n", "\n")
	if got != wantText {
		t.Fatalf("summary output does not match golden.\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

var summaryByteValue = regexp.MustCompile(`(?m)(  )\d+(?:\.\d+)? [KMGT]?B$`)

func normalizeSummaryGolden(output string) string {
	output = strings.ReplaceAll(output, "\r\n", "\n")
	return summaryByteValue.ReplaceAllString(output, `${1}<size>`)
}
