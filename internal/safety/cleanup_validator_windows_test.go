//go:build windows

// A real NTFS junction run through CleanupValidator.Validate, the exact
// gate `libra clean --execute` calls immediately before quarantine.Move for
// every plan item (internal/app/cleanup_service.go Execute). This was an
// open item in docs/libra_integration_contracts.md §15 ("Windows 실제
// volume에서 junction, ACL, hidden attribute 통합 테스트") and, until now,
// cleanup_validator.go's reparse-point rejection (line ~101) had zero test
// coverage, synthetic or real -- despite being the single check standing
// between "someone replaced node_modules with a junction into a shared
// cache" and libra quarantining whatever that junction points at.
package safety

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/pathutil"
)

func TestCleanupValidatorBlocksRealJunctionEvenWhenEverythingElseLooksSafe(t *testing.T) {
	root := t.TempDir()
	sharedTarget := filepath.Join(root, "shared-cache")
	if err := os.MkdirAll(sharedTarget, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sharedTarget, "payload.txt"), []byte("shared, not disposable"), 0o644); err != nil {
		t.Fatal(err)
	}

	projectRoot := filepath.Join(root, "project")
	junction := filepath.Join(projectRoot, "node_modules")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", junction, sharedTarget).CombinedOutput(); err != nil {
		t.Skipf("mklink /J unsupported in this environment: %v: %s", err, out)
	}

	normalizedJunction, err := pathutil.Normalize(junction)
	if err != nil {
		t.Fatal(err)
	}
	normalizedRoot, err := pathutil.Normalize(projectRoot)
	if err != nil {
		t.Fatal(err)
	}

	classifier, err := NewPathClassifier(nil)
	if err != nil {
		t.Fatal(err)
	}
	validator := CleanupValidator{Paths: classifier}

	// Every other fact a real detector would report says "safe": type
	// matches, Risk is SAFE, Regenerable is true, it's inside its owner
	// project. Only the reparse-point check should be standing between this
	// and a real quarantine move -- and it must hold on its own.
	resource := domain.Resource{NormalizedPath: normalizedJunction, Type: domain.ResourceTypeNodeModules, Risk: domain.RiskSafe, Regenerable: true}
	planItem := domain.CleanupPlanItem{NormalizedPath: normalizedJunction, ExpectedType: domain.ResourceTypeNodeModules}

	_, err = validator.Validate(context.Background(), planItem, resource, normalizedRoot)
	if err == nil {
		t.Fatal("Validate() succeeded for a junction; want it blocked before any quarantine move is attempted")
	}
	if !errors.Is(err, ErrCleanupBlocked) {
		t.Fatalf("Validate() error = %v, want it wrapping ErrCleanupBlocked", err)
	}

	// The validator must reject read-only -- it must not have touched
	// anything on disk to reach that verdict.
	if _, statErr := os.Lstat(junction); statErr != nil {
		t.Fatalf("junction should be untouched after a blocked validation: %v", statErr)
	}
	if data, readErr := os.ReadFile(filepath.Join(sharedTarget, "payload.txt")); readErr != nil || string(data) != "shared, not disposable" {
		t.Fatalf("shared target must be untouched: data=%q err=%v", data, readErr)
	}
}
