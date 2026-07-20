package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/app"
	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/store/sqlite"
)

// fakeInstalledWindowsSDKDetector reports one installed Windows SDK without
// touching the real filesystem/registry, standing in for
// windowsdk.FilesystemDetector so this test's SDK version match is
// deterministic and runs on any host OS.
type fakeInstalledWindowsSDKDetector struct{ path string }

func (d fakeInstalledWindowsSDKDetector) Detect(context.Context, app.Environment) app.DetectionResult[domain.Resource] {
	return app.DetectionResult[domain.Resource]{Items: []domain.Resource{{
		Name: "Windows SDK", Type: domain.ResourceTypeWindowsSDK, Version: "10.0.22621.0", DisplayPath: d.path,
	}}}
}

// TestScanWiresRealDependencyEdgeForExplainAndImpact closes issue #22: a
// real `libra scan` -- not cmd/dependency_fixture_test.go's
// seedWindowsSDKDependency helper every other explain/impact test still
// uses -- must produce the PROJECT -> RESOURCE dependency edge that
// GameClient.vcxproj's unconditional WindowsTargetPlatformVersion=10.0.22621.0
// declaration implies, once matched against an installed Windows SDK of the
// same version. This exercises the full chain now wired in cmd/scan.go:
// MSBuildProjectDetector carries the declared property into
// AnalysisOrchestrator, which hands it to app.MSBuildDependencyAnalyzer
// alongside a ResourceIndex built from whatever ResourceDetectors found,
// and persists whatever it resolves.
func TestScanWiresRealDependencyEdgeForExplainAndImpact(t *testing.T) {
	scanRoot = ""
	cfgPath = ""

	fixture, err := filepath.Abs("../testdata/msbuild")
	if err != nil {
		t.Fatalf("resolve fixture path: %v", err)
	}
	dir := t.TempDir()
	t.Chdir(dir)

	sdkPath := filepath.Join(t.TempDir(), "WindowsKits10")
	if err := os.MkdirAll(sdkPath, 0o755); err != nil {
		t.Fatalf("create fake SDK dir: %v", err)
	}
	previousDetectors := resourceDetectors
	resourceDetectors = func() []app.ResourceDetector {
		return []app.ResourceDetector{fakeInstalledWindowsSDKDetector{path: sdkPath}}
	}
	t.Cleanup(func() { resourceDetectors = previousDetectors })

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

	// Check the persisted graph directly first: this is the actual claim
	// issue #22 was about (a real scan never wrote a Dependency/Evidence
	// row), independent of how explain/impact happen to render it.
	db, err := openDatabase()
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	ctx := context.Background()
	resources, err := sqlite.NewResourceRepository(db).ListByType(ctx, domain.ResourceTypeWindowsSDK)
	if err != nil {
		t.Fatalf("list windows-sdk resources: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("persisted windows-sdk resources = %d, want 1", len(resources))
	}
	resource := resources[0]

	edges, err := sqlite.NewDependencyRepository(db).FindProjectsByResource(ctx, resource.ID)
	if err != nil {
		t.Fatalf("find projects depending on %q: %v", resource.ID, err)
	}
	if len(edges) != 1 {
		t.Fatalf("dependency edges for %q = %d, want 1 (scan should have resolved GameClient's declared WindowsTargetPlatformVersion without any seeding)", resource.ID, len(edges))
	}
	if edges[0].Relation != domain.RelationRequires {
		t.Errorf("edge relation = %v, want RelationRequires", edges[0].Relation)
	}
	db.Close()

	// No seedWindowsSDKDependency call above and none below: everything
	// explain/impact render past this point must come from the edge just
	// asserted, produced entirely by the real scan.
	explainOut := run("explain", "windows-sdk:10.0.22621.0")
	for _, want := range []string{
		"Resource: Windows SDK 10.0.22621.0",
		"GameClient",
		"Evidence: DECLARED",
		"Property: WindowsTargetPlatformVersion",
		"Rebuild: HIGH",
	} {
		if !bytes.Contains(explainOut.Bytes(), []byte(want)) {
			t.Fatalf("explain output missing %q:\n%s", want, explainOut)
		}
	}

	impactOut := run("impact", "windows-sdk:10.0.22621.0")
	for _, want := range []string{
		"Affected projects: 1",
		"GameClient",
		"RUN",
		"likely unaffected",
		"BUILD",
		"expected to fail",
		"DEBUG",
		"expected to fail if build runs",
		"RESTORE",
	} {
		if !bytes.Contains(impactOut.Bytes(), []byte(want)) {
			t.Fatalf("impact output missing %q:\n%s", want, impactOut)
		}
	}
}
