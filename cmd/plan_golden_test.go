package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/app"
)

// TestPlanGoldenNodeFixtureHasNoSafeCandidatesYet locks down `libra plan`
// output for the committed testdata/node fixture tree. Real Node/MSBuild
// artifact detectors only fill 2 of the 4 CleanupEvidence flags
// DefaultRiskPolicy requires for SAFE (see
// docs/libra_integration_contracts.md §19.3: reparse-point and Git-tracked-
// original checks are not implemented yet), so a real scan of this fixture
// is expected to produce zero SAFE candidates and list every artifact under
// REVIEW instead. This pins that as the current, honest behavior rather
// than a bug -- if a detector later fills in the missing evidence, this
// golden file should be regenerated with -update and the new SAFE output
// reviewed deliberately, not silently accepted.
func TestPlanGoldenNodeFixtureHasNoSafeCandidatesYet(t *testing.T) {
	scanRoot = ""
	cfgPath = ""
	jsonOutput = false
	planTarget = ""
	planRisk = ""
	planProject = ""
	previousResourceDetectors := resourceDetectors
	resourceDetectors = func() []app.ResourceDetector { return nil }
	t.Cleanup(func() { resourceDetectors = previousResourceDetectors })

	fixture, err := filepath.Abs("../testdata/node")
	if err != nil {
		t.Fatalf("resolve fixture path: %v", err)
	}
	// Resolve the golden path before chdir, so it stays anchored to the repo
	// rather than the temp working directory.
	goldenPath, err := filepath.Abs(filepath.Join("..", "testdata", "golden", "plan_node_no_safe.golden"))
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
	got := normalizePlanGolden(run("plan").String(), fixture)

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
		t.Fatalf("plan output does not match golden.\n--- got ---\n%s\n--- want ---\n%s", got, wantText)
	}
}

var planByteValue = regexp.MustCompile(`\d+(?:\.\d+)? [KMGT]?B\b`)

// normalizePlanGolden strips the two sources of non-determinism in `plan`
// output: the plan ID (timestamp + random suffix, see newPlanID) and the
// fixture's absolute checkout path (varies by machine/tempdir). Byte sizes
// are also normalized since Git line-ending conversion can change fixture
// sizes across platforms, the same rationale as normalizeSummaryGolden.
func normalizePlanGolden(output, fixtureRoot string) string {
	output = strings.ReplaceAll(output, "\r\n", "\n")
	output = strings.ReplaceAll(output, fixtureRoot, "<NODE_FIXTURE>")
	lines := strings.SplitN(output, "\n", 2)
	if len(lines) > 0 && strings.HasPrefix(lines[0], "Plan ID: ") {
		lines[0] = "Plan ID: <plan-id>"
	}
	output = strings.Join(lines, "\n")
	return planByteValue.ReplaceAllString(output, "<size>")
}
