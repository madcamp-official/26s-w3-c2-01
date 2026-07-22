package safety

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/pathutil"
	"github.com/madcamp-official/26s-w3-c2-01/internal/scanner"
)

// TestIsAllowedArtifactName covers docs/libra_integration_contracts.md §19.4
// 결정 6: Python cache/venv directory names must be structurally eligible
// for cleanup, egg-info's variable-prefix name must match by suffix, and
// conda environment directories must never be allowlisted (결정 4 keeps
// conda out of the cleanup path regardless of basename).
func TestIsAllowedArtifactName(t *testing.T) {
	cases := map[string]bool{
		"node_modules":   true,
		"dist":           true,
		"target":         true,
		".venv":          true,
		"venv":           true,
		"env":            true,
		"__pycache__":    true,
		".pytest_cache":  true,
		".mypy_cache":    true,
		"mypkg.egg-info": true,
		"a.egg-info":     true,
		"pods":           true,  // CocoaPods Pods/ (§19.9), compared lowercased
		".build":         true,  // SwiftPM .build/ (§19.9)
		"envs":           false, // a conda local prefix env directory name
		"conda-env":      false,
		"random-folder":  false,
	}
	for name, want := range cases {
		if got := isAllowedArtifactName(name); got != want {
			t.Errorf("isAllowedArtifactName(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestCleanupValidatorRejectsNestedTargetDirectory(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "nested", "target")
	normalizedRoot, _ := pathutil.Normalize(root)
	normalizedTarget, _ := pathutil.Normalize(nested)
	resource := domain.Resource{Type: domain.ResourceTypeBuildOutput, NormalizedPath: normalizedTarget, Risk: domain.RiskSafe, Regenerable: true}
	item := domain.CleanupPlanItem{NormalizedPath: normalizedTarget, ExpectedType: resource.Type}
	_, err := (CleanupValidator{}).Validate(context.Background(), item, resource, normalizedRoot)
	if !errors.Is(err, ErrCleanupBlocked) || !strings.Contains(err.Error(), "directly under") {
		t.Fatalf("Validate() error = %v, want direct-child target rejection", err)
	}
}

func TestCleanupValidatorAllowsVerifiedDirectTargetDirectory(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "artifact.bin"), []byte("generated"), 0o644); err != nil {
		t.Fatal(err)
	}
	normalizedRoot, _ := pathutil.Normalize(root)
	normalizedTarget, _ := pathutil.Normalize(target)
	measured, err := scanner.MeasureResource(context.Background(), scanner.New(1), target)
	if err != nil || measured.LastModifiedAt == nil {
		t.Fatalf("measure = %#v, %v", measured, err)
	}
	resource := domain.Resource{Type: domain.ResourceTypeBuildOutput, NormalizedPath: normalizedTarget, Risk: domain.RiskSafe, Regenerable: true}
	item := domain.CleanupPlanItem{NormalizedPath: normalizedTarget, ExpectedType: resource.Type,
		ExpectedSize: measured.LogicalSize, ExpectedModifiedTime: *measured.LastModifiedAt}
	classifier, err := NewPathClassifier(nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (CleanupValidator{Paths: classifier}).Validate(context.Background(), item, resource, normalizedRoot); err != nil {
		t.Fatalf("Validate() error = %v, want verified direct target allowed", err)
	}
}
