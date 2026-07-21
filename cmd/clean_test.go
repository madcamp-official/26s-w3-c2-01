package cmd

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/app"
	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/output"
	"github.com/madcamp-official/26s-w3-c2-01/internal/store/sqlite"
)

func TestCleanCommandDryRunPreviewsSeededSafeResource(t *testing.T) {
	scanRoot = ""
	jsonOutput = false
	cfgPath = ""
	planTarget = ""
	planRisk = ""
	planProject = ""
	cleanPlanID = ""
	cleanExecute = false
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
	seeded := seedSafeResource(t, "big-node-modules", 100)

	planOut := run("plan", "--json").String()
	var planView struct {
		PlanID string `json:"plan_id"`
	}
	if _, err := output.DecodeEnvelope([]byte(planOut), &planView); err != nil {
		t.Fatalf("decode plan output: %v\n%s", err, planOut)
	}

	cleanOut := run("clean", "--plan", planView.PlanID, "--json").String()
	var cleanView struct {
		PlanID string `json:"plan_id"`
		DryRun bool   `json:"dry_run"`
		Items  []struct {
			Path              string `json:"path"`
			ExpectedSizeBytes int64  `json:"expected_size_bytes"`
			Status            string `json:"status"`
		} `json:"items"`
	}
	if _, err := output.DecodeEnvelope([]byte(cleanOut), &cleanView); err != nil {
		t.Fatalf("decode clean output: %v\n%s", err, cleanOut)
	}
	if !cleanView.DryRun {
		t.Fatal("dry_run = false, want true")
	}
	if len(cleanView.Items) != 1 || cleanView.Items[0].Status != "WOULD_MOVE" {
		t.Fatalf("items = %#v, want single WOULD_MOVE item", cleanView.Items)
	}
	if cleanView.Items[0].Path != seeded.NormalizedPath {
		t.Fatalf("item path = %q, want %q", cleanView.Items[0].Path, seeded.NormalizedPath)
	}
	if cleanView.Items[0].ExpectedSizeBytes != seeded.ReclaimableSize {
		t.Fatalf("item size = %d, want %d", cleanView.Items[0].ExpectedSizeBytes, seeded.ReclaimableSize)
	}
}

func TestCleanCommandFlagsDriftSincePlanning(t *testing.T) {
	scanRoot = ""
	jsonOutput = false
	cfgPath = ""
	planTarget = ""
	planRisk = ""
	planProject = ""
	cleanPlanID = ""
	cleanExecute = false
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
	seeded := seedSafeResource(t, "big-node-modules", 100)

	planOut := run("plan", "--json").String()
	var planView struct {
		PlanID string `json:"plan_id"`
	}
	if _, err := output.DecodeEnvelope([]byte(planOut), &planView); err != nil {
		t.Fatalf("decode plan output: %v\n%s", err, planOut)
	}

	// Re-scanning the resource as REVIEW simulates the world drifting after
	// planning but before clean runs -- clean must catch this rather than
	// trusting the stale plan snapshot.
	db, err := openDatabase()
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	seeded.Risk = domain.RiskReview
	if err := sqlite.NewResourceRepository(db).Upsert(t.Context(), seeded); err != nil {
		t.Fatalf("mutate seeded resource: %v", err)
	}
	db.Close()

	cleanOut := run("clean", "--plan", planView.PlanID, "--json").String()
	var cleanView struct {
		Items []struct {
			Status string `json:"status"`
			Detail string `json:"detail"`
		} `json:"items"`
	}
	if _, err := output.DecodeEnvelope([]byte(cleanOut), &cleanView); err != nil {
		t.Fatalf("decode clean output: %v\n%s", err, cleanOut)
	}
	if len(cleanView.Items) != 1 || cleanView.Items[0].Status != "CHANGED" {
		t.Fatalf("items = %#v, want single CHANGED item", cleanView.Items)
	}
	if cleanView.Items[0].Detail == "" {
		t.Fatal("CHANGED item has no explanatory detail")
	}
}

func TestCleanCommandExecuteRejectsUnknownPlan(t *testing.T) {
	scanRoot = ""
	jsonOutput = false
	cfgPath = ""
	cleanPlanID = ""
	cleanExecute = false
	t.Chdir(t.TempDir())

	out := &bytes.Buffer{}
	rootCmd.SetOut(out)
	rootCmd.SetErr(out)
	rootCmd.SetArgs([]string{"init"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute(init) error = %v", err)
	}

	rootCmd.SetArgs([]string{"clean", "--plan", "does-not-matter", "--execute"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("Execute(clean --execute) error = nil, want not-implemented error")
	}
	if !strings.Contains(err.Error(), "cleanup plan not found") {
		t.Fatalf("error = %v, want cleanup plan not found", err)
	}
	cleanExecute = false
}

func TestCleanCommandRequiresPlanFlag(t *testing.T) {
	jsonOutput = false
	cleanPlanID = ""
	cleanExecute = false
	t.Chdir(t.TempDir())

	out := &bytes.Buffer{}
	rootCmd.SetOut(out)
	rootCmd.SetErr(out)
	rootCmd.SetArgs([]string{"init"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute(init) error = %v", err)
	}

	rootCmd.SetArgs([]string{"clean"})
	if err := rootCmd.Execute(); err == nil {
		t.Fatal("Execute(clean) error = nil, want --plan required error")
	}
}
