//go:build windows

// System-path negative fixtures for docs/libra_cli_commands_and_schedule.md
// Day 6 "C. 시스템 경로 negative test": README promises libra never touches
// C:\Windows, C:\Program Files, C:\Program Files (x86), Windows SDK, Visual
// Studio, or .NET Runtime installs, and risk_policy.go always BLOCKs
// Resource.SystemManaged / a Docker volume. Those are covered as synthetic
// unit tests in risk_policy_test.go and path_classifier_test.go already,
// but none of the existing tests pin down the literal real paths a Windows
// machine actually has (the ones the README calls out by name), and none
// prove the block survives an adversarially "fully verified" resource --
// i.e. that path protection is never overridable by another axis being
// wrong (a buggy detector marking a system directory Regenerable, say).
// This file locks both down using the real environment variables
// (WINDIR/ProgramFiles/ProgramFiles(x86)) libra itself reads in production
// (internal/safety/roots_windows.go), so it fails the moment that
// protection regresses on an actual Windows machine, not just in a mock.
package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/safety"
	"github.com/madcamp-official/26s-w3-c2-01/internal/scanner"
)

// systemPathFixtures builds one Resource per path category the Day 6
// schedule doc names explicitly. Every one of them is given SizeKnown=true
// so ResourceService.Observe skips a real filesystem walk (issue #many:
// C:\Windows itself is far too large to actually measure in a test, and a
// fresh machine may not have Visual Studio/the Windows SDK installed at
// all) -- the point here is the path string and resource type, not real
// bytes on disk. pathutil.Normalize never touches the filesystem (see
// internal/pathutil/normalize.go), so this works whether or not the deeper
// subpaths exist for real.
func systemPathFixtures(t *testing.T) []domain.Resource {
	t.Helper()
	winDir := os.Getenv("WINDIR")
	programFiles := os.Getenv("ProgramFiles")
	programFilesX86 := os.Getenv("ProgramFiles(x86)")
	if winDir == "" || programFiles == "" || programFilesX86 == "" {
		t.Skip("WINDIR/ProgramFiles/ProgramFiles(x86) not set; cannot build real system-path fixtures")
	}
	when := time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)
	return []domain.Resource{
		{Type: domain.ResourceTypeBuildOutput, DisplayPath: filepath.Join(winDir, "System32"), SizeKnown: true, LogicalSize: 1, LastModifiedAt: &when},
		{Type: domain.ResourceTypeBuildOutput, DisplayPath: filepath.Join(programFiles, "SomeVendor", "dist"), SizeKnown: true, LogicalSize: 1, LastModifiedAt: &when},
		{Type: domain.ResourceTypeBuildOutput, DisplayPath: filepath.Join(programFilesX86, "SomeVendor", "dist"), SizeKnown: true, LogicalSize: 1, LastModifiedAt: &when},
		{Type: domain.ResourceTypeWindowsSDK, Version: "10.0.22621.0", DisplayPath: filepath.Join(programFilesX86, "Windows Kits", "10", "Include", "10.0.22621.0"), SizeKnown: true, LogicalSize: 1, LastModifiedAt: &when},
		{Type: domain.ResourceTypeVisualStudio, Version: "2022", DisplayPath: filepath.Join(programFiles, "Microsoft Visual Studio", "2022", "Community"), SizeKnown: true, LogicalSize: 1, LastModifiedAt: &when},
		{Type: domain.ResourceTypeNetFXSDK, Version: "4.8", DisplayPath: filepath.Join(programFilesX86, "Windows Kits", "NETFXSDK", "4.8"), SizeKnown: true, LogicalSize: 1, LastModifiedAt: &when},
		{Type: domain.ResourceTypeDotNetSDK, Version: "8.0.100", DisplayPath: filepath.Join(programFiles, "dotnet", "shared", "Microsoft.NETCore.App", "8.0.0"), SizeKnown: true, LogicalSize: 1, LastModifiedAt: &when},
		{Type: domain.ResourceTypeDockerVolume, Version: "project-db-data", DisplayPath: `C:\ProgramData\Docker\volumes\project-db-data\_data`, SizeKnown: true, LogicalSize: 1, LastModifiedAt: &when},
	}
}

// fullyVerifiedEvidence is the strongest possible case *for* auto-cleanup:
// every fact RiskPolicy checks says "safe to delete". A real detector
// should never produce this for a system path, but this test assumes one
// day a bug makes it happen anyway, and checks the path/type gate still
// wins.
func fullyVerifiedEvidence() CleanupEvidence {
	return CleanupEvidence{ProjectOwned: true, KnownOutputPath: true, ReparsePointFree: true, GitTrackedOriginalsAbsent: true}
}

type noopResourceRepository struct{}

func (noopResourceRepository) Upsert(context.Context, domain.Resource) error { return nil }
func (noopResourceRepository) FindByID(context.Context, string) (domain.Resource, error) {
	return domain.Resource{}, os.ErrNotExist
}
func (noopResourceRepository) ListByType(context.Context, domain.ResourceType) ([]domain.Resource, error) {
	return nil, nil
}
func (noopResourceRepository) List(context.Context) ([]domain.Resource, error) { return nil, nil }

func TestResourceServiceBlocksRealSystemPathsEvenWithAdversariallyPerfectEvidence(t *testing.T) {
	classifier, err := safety.NewSystemPathClassifier()
	if err != nil {
		t.Fatalf("NewSystemPathClassifier() error = %v", err)
	}
	service := NewResourceService(scanner.New(1), noopResourceRepository{}, classifier, DefaultRiskPolicy{})

	for _, fixture := range systemPathFixtures(t) {
		fixture := fixture
		t.Run(string(fixture.Type)+"/"+fixture.DisplayPath, func(t *testing.T) {
			fixture.Regenerable = true // adversarial: a buggy detector claims this is a disposable build artifact
			observation, err := service.Observe(context.Background(), ResourceObservationInput{
				Resource: fixture, Cleanup: fullyVerifiedEvidence(), ProjectScoped: true,
			})
			if err != nil {
				t.Fatalf("Observe() error = %v", err)
			}
			if observation.Resource.Risk != domain.RiskBlocked {
				t.Fatalf("Risk = %q, want BLOCKED for %q (regardless of Regenerable/CleanupEvidence)", observation.Resource.Risk, fixture.DisplayPath)
			}
			if observation.Resource.ReclaimableSize != 0 {
				t.Fatalf("ReclaimableSize = %d, want 0 for a BLOCKED system path", observation.Resource.ReclaimableSize)
			}
			if observation.Resource.CleanupDisposition == domain.DispositionAutoQuarantine {
				t.Fatalf("CleanupDisposition = %q, must never be AUTO_QUARANTINE for a system path", observation.Resource.CleanupDisposition)
			}
		})
	}
}

// TestPlanServiceNeverSelectsRealSystemPaths runs the same fixtures through
// the full Observe -> Build pipeline `libra scan` and `libra plan` actually
// use, with an unlimited target (opts.TargetBytes == 0, i.e. "select every
// SAFE candidate you can find") -- the scenario that would surface a
// regression fastest, since a capped target could otherwise coincidentally
// hide a wrongly-SAFE system resource simply by never reaching it in sort
// order.
func TestPlanServiceNeverSelectsRealSystemPaths(t *testing.T) {
	classifier, err := safety.NewSystemPathClassifier()
	if err != nil {
		t.Fatalf("NewSystemPathClassifier() error = %v", err)
	}
	fixtures := systemPathFixtures(t)
	collected := &planResourceRepositoryStub{}
	observer := NewResourceService(scanner.New(1), noopResourceRepository{}, classifier, DefaultRiskPolicy{})
	for _, fixture := range fixtures {
		fixture.Regenerable = true
		observation, err := observer.Observe(context.Background(), ResourceObservationInput{
			Resource: fixture, Cleanup: fullyVerifiedEvidence(), ProjectScoped: true,
		})
		if err != nil {
			t.Fatalf("Observe() error = %v", err)
		}
		collected.resources = append(collected.resources, observation.Resource)
	}

	scans := &planScanRepositoryStub{record: newTestScanRecord("scan-system-paths")}
	result, err := NewPlanService(collected, &planProjectRepositoryStub{}, scans, &planDependencyRepositoryStub{owners: map[string]string{}}).
		Build(context.Background(), PlanOptions{})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(result.Plan.Items) != 0 {
		t.Fatalf("Plan.Items = %#v, want none of these system paths ever auto-selected", result.Plan.Items)
	}
	if result.Plan.SelectedBytes != 0 {
		t.Fatalf("SelectedBytes = %d, want 0", result.Plan.SelectedBytes)
	}
	if len(result.Blocked) != len(fixtures) {
		t.Fatalf("Blocked = %d items, want all %d fixtures reported BLOCKED (visible, not silently dropped)", len(result.Blocked), len(fixtures))
	}
}
