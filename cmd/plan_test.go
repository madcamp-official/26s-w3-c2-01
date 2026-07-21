package cmd

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/app"
	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/output"
	"github.com/spf13/cobra"
)

func TestPlanCommandSelectsSeededSafeResourceUntilTarget(t *testing.T) {
	scanRoot = ""
	jsonOutput = false
	cfgPath = ""
	planTarget = ""
	planRisk = ""
	planProject = ""
	// Real detectors read the host machine (see resources_test.go's same
	// guard); disable them so the plan only sees the deterministic Node
	// fixture output plus the SAFE resource this test seeds directly.
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

	run("init")
	run("scan", "--root", fixture)
	// A real scan of this fixture never produces a SAFE resource today
	// (see seedSafeResource's doc comment), so seed one directly to
	// exercise the selection and persistence path end to end.
	seeded := seedSafeResource(t, "big-node-modules", 2_000_000_000)

	got := run("plan", "--target", "1GB", "--json").String()
	var view struct {
		PlanID   string `json:"plan_id"`
		Selected int64  `json:"selected_bytes"`
		Status   string `json:"status"`
		Safe     []struct {
			SizeBytes int64  `json:"size_bytes"`
			Path      string `json:"path"`
		} `json:"safe"`
	}
	if _, err := output.DecodeEnvelope([]byte(got), &view); err != nil {
		t.Fatalf("decode plan output: %v\n%s", err, got)
	}
	if view.Status != "READY" {
		t.Fatalf("status = %q, want READY", view.Status)
	}
	if len(view.Safe) != 1 || view.Safe[0].Path != seeded.DisplayPath {
		t.Fatalf("safe = %#v, want exactly [%s]", view.Safe, seeded.DisplayPath)
	}
	if view.Selected != seeded.ReclaimableSize {
		t.Fatalf("selected = %d, want %d", view.Selected, seeded.ReclaimableSize)
	}
	if !strings.HasPrefix(view.PlanID, "plan-") {
		t.Fatalf("unexpected plan id %q", view.PlanID)
	}
}

func TestPlanCommandDefaultsToUnlimitedTargetAndAllRiskTiers(t *testing.T) {
	scanRoot = ""
	jsonOutput = false
	cfgPath = ""
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
	got := run("plan").String()
	if !strings.Contains(got, "Target: unlimited") {
		t.Fatalf("plan output missing unlimited target:\n%s", got)
	}
	// This fixture's node_modules/dist are checked into this very repo's
	// Git history (testdata is real, tracked content), so
	// projectArtifactCleanupEvidence's Git-tracked-originals check correctly
	// keeps GitTrackedOriginalsAbsent=false and the risk policy classifies
	// them REVIEW, not SAFE -- this is about the fixture being tracked, not
	// about missing detector evidence (a real, untracked node_modules is
	// classified SAFE today; see plan_service.go and this file's other
	// tests for the seeded-SAFE path).
	if !strings.Contains(got, "(none)") {
		t.Fatalf("plan output missing empty SAFE marker:\n%s", got)
	}
	if !strings.Contains(got, "node_modules") {
		t.Fatalf("plan output missing REVIEW node_modules candidate:\n%s", got)
	}
	// issue #40: plan used to be just a path/size list with no indication
	// of *why* a candidate landed in its risk tier.
	if !strings.Contains(got, "Reason: cleanup safety has not been fully verified") {
		t.Fatalf("plan output missing REVIEW candidate's Reason:\n%s", got)
	}
}

// TestPlanCommandReviewAndBlockedShowRealSizeNotZero is a regression test
// for a bug where REVIEW/BLOCKED lines printed domain.Resource.ReclaimableSize
// (always 0 for anything that isn't SAFE, per resource_service.go's risk
// switch) instead of LogicalSize, so every REVIEW/BLOCKED candidate showed
// "0 B" regardless of its real on-disk size.
func TestPlanCommandReviewAndBlockedShowRealSizeNotZero(t *testing.T) {
	scanRoot = ""
	jsonOutput = false
	cfgPath = ""
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
	got := run("plan", "--json").String()
	var view struct {
		Review []struct {
			SizeBytes int64  `json:"size_bytes"`
			Path      string `json:"path"`
		} `json:"review"`
	}
	if _, err := output.DecodeEnvelope([]byte(got), &view); err != nil {
		t.Fatalf("decode plan output: %v\n%s", err, got)
	}
	if len(view.Review) == 0 {
		t.Fatalf("plan output has no REVIEW candidates to check:\n%s", got)
	}
	for _, r := range view.Review {
		if r.SizeBytes <= 0 {
			t.Fatalf("REVIEW candidate %q has size_bytes = %d, want > 0 (real LogicalSize)", r.Path, r.SizeBytes)
		}
	}
}

// fakeDependencyRepository is a minimal app.DependencyRepository test
// double for buildPlanView tests that don't exercise the BLOCKED "used by"
// lookup. fakeResourceRepository/fakeProjectRepository (reused here) are
// defined in target_test.go.
type fakeDependencyRepository struct{}

func (fakeDependencyRepository) UpsertGraph(context.Context, string, domain.Dependency, []domain.Evidence) error {
	return nil
}
func (fakeDependencyRepository) FindResourcesByProject(context.Context, string) ([]domain.Dependency, error) {
	return nil, nil
}
func (fakeDependencyRepository) FindProjectsByResource(context.Context, string) ([]domain.Dependency, error) {
	return nil, nil
}
func (fakeDependencyRepository) FindEvidence(context.Context, string) ([]domain.Evidence, error) {
	return nil, nil
}

// TestBuildPlanViewSafeUsesResourceDisplayPathNotSnapshotNormalizedPath is a
// regression test for issue #41's path-casing bug: SAFE lines rendered
// CleanupPlanItem.NormalizedPath (the plan snapshot's identity field, always
// lowercase on Windows) instead of the resource's DisplayPath -- unlike
// `projects`, which has always correctly used DisplayPath. It exercises
// buildPlanView directly with a fake ResourceRepository rather than a real
// scan/sqlite round trip: sqlite.ResourceRepository.Upsert legitimately
// rejects a DisplayPath/NormalizedPath pair that don't match (it validates
// NormalizedPath against pathutil.Normalize(DisplayPath)), and a real
// Normalize() call can never produce the case difference this bug depends
// on anyway, since internal/pathutil/normalize_other.go's non-Windows build
// does not lowercase (only normalize_windows.go does).
func TestBuildPlanViewSafeUsesResourceDisplayPathNotSnapshotNormalizedPath(t *testing.T) {
	planRisk = ""
	t.Cleanup(func() { planRisk = "" })

	displayPath := "/Users/Someone/MixedCase/node_modules"
	normalizedPath := "/users/someone/mixedcase/node_modules" // what Windows' pathutil would store
	resources := &fakeResourceRepository{byID: []domain.Resource{
		{ID: "resource-1", DisplayPath: displayPath, NormalizedPath: normalizedPath},
	}}
	result := app.PlanResult{Plan: domain.CleanupPlan{Items: []domain.CleanupPlanItem{
		{ResourceID: "resource-1", NormalizedPath: normalizedPath, ExpectedSize: 1024, RiskAtPlanning: domain.RiskSafe},
	}}}

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	view, err := buildPlanView(cmd, result, resources, fakeDependencyRepository{}, &fakeProjectRepository{})
	if err != nil {
		t.Fatalf("buildPlanView() error = %v", err)
	}
	if len(view.Safe) != 1 || view.Safe[0].Path != displayPath {
		t.Fatalf("Safe = %#v, want exactly one item with Path %q (DisplayPath, not lowercased NormalizedPath %q)",
			view.Safe, displayPath, normalizedPath)
	}
}

func TestPlanCommandRejectsInvalidRiskFlag(t *testing.T) {
	scanRoot = ""
	jsonOutput = false
	cfgPath = ""
	planTarget = ""
	planRisk = ""
	planProject = ""
	t.Chdir(t.TempDir())

	out := &bytes.Buffer{}
	rootCmd.SetOut(out)
	rootCmd.SetErr(out)
	rootCmd.SetArgs([]string{"init"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute(init) error = %v", err)
	}

	rootCmd.SetArgs([]string{"plan", "--risk", "not-a-level"})
	if err := rootCmd.Execute(); err == nil {
		t.Fatal("Execute(plan --risk not-a-level) error = nil, want validation error")
	}
	planRisk = ""
}

func TestPlanCommandRequiresScanFirst(t *testing.T) {
	scanRoot = ""
	jsonOutput = false
	cfgPath = ""
	planTarget = ""
	planRisk = ""
	planProject = ""
	t.Chdir(t.TempDir())

	out := &bytes.Buffer{}
	rootCmd.SetOut(out)
	rootCmd.SetErr(out)
	rootCmd.SetArgs([]string{"init"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute(init) error = %v", err)
	}

	rootCmd.SetArgs([]string{"plan"})
	if err := rootCmd.Execute(); err == nil {
		t.Fatal("Execute(plan) error = nil, want error when no scan has run yet")
	}
}
