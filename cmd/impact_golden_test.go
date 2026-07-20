package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestImpactGoldenGameClientWindowsSDK locks down `libra impact` output for
// a GameClient project seeded with a synthetic Windows SDK dependency (see
// seedWindowsSDKDependency's doc comment for why this is seeded directly
// rather than produced by `scan` itself). The project's absolute checkout
// path is substituted with a stable placeholder so the golden file does not
// depend on where the repository happens to be checked out (testdata paths
// must not be absolute, per docs/libra_collaboration_rules.md §11).
func TestImpactGoldenGameClientWindowsSDK(t *testing.T) {
	scanRoot = ""
	cfgPath = ""

	fixture, err := filepath.Abs("../testdata/msbuild")
	if err != nil {
		t.Fatalf("resolve fixture path: %v", err)
	}
	goldenPath, err := filepath.Abs(filepath.Join("..", "testdata", "golden", "impact_gameclient_windows_sdk.golden"))
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
	_, project := seedWindowsSDKDependency(t, "GameClient")

	got := run("impact", "windows-sdk:10.0.22621.0").String()
	got = strings.ReplaceAll(got, "\r\n", "\n")
	got = strings.ReplaceAll(got, project.RootPath, "<GameClient>")

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
		t.Fatalf("impact output does not match golden.\n--- got ---\n%s\n--- want ---\n%s", got, wantText)
	}
}
