package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/pathutil"
)

func TestResourceRepositoryUpsertsAndFindsResource(t *testing.T) {
	repository := newTestResourceRepository(t)
	resource := testResource(t, domain.ResourceTypeWindowsSDK, "10.0.22621.0")
	if err := repository.Upsert(context.Background(), resource); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	resource.Name = "Updated Windows SDK"
	resource.LogicalSize = 2048
	resource.ReclaimableSize = 1024
	if err := repository.Upsert(context.Background(), resource); err != nil {
		t.Fatalf("Upsert(update) error = %v", err)
	}

	got, err := repository.FindByID(context.Background(), resource.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if !reflect.DeepEqual(got, resource) {
		t.Fatalf("FindByID() = %#v, want %#v", got, resource)
	}
}

func TestResourceRepositoryRoundTripsRegenerationCommand(t *testing.T) {
	repository := newTestResourceRepository(t)
	resource := testResource(t, domain.ResourceTypeNodeModules, "")
	resource.RegenerationCommand = "npm ci"
	if err := repository.Upsert(context.Background(), resource); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	got, err := repository.FindByID(context.Background(), resource.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if got.RegenerationCommand != "npm ci" {
		t.Fatalf("RegenerationCommand = %q, want %q", got.RegenerationCommand, "npm ci")
	}
}

func TestResourceRepositoryRoundTripsConfidenceProfileAndRiskReasons(t *testing.T) {
	repository := newTestResourceRepository(t)
	resource := testResource(t, domain.ResourceTypeBuildOutput, "")
	resource.ConfidenceProfile = domain.ConfidenceProfile{Classification: 90, Ownership: 100, Dependency: 80, CleanupSafety: 100, ScanCoverage: 80, Freshness: 100}
	resource.Confidence = resource.ConfidenceProfile.Overall()
	resource.RiskReasons = []domain.RiskReason{{Code: "CLEANUP_EVIDENCE_COMPLETE", Severity: domain.RiskReasonSafeguard, Message: "verified"}}
	if err := repository.Upsert(context.Background(), resource); err != nil {
		t.Fatal(err)
	}

	got, err := repository.FindByID(context.Background(), resource.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.ConfidenceProfile, resource.ConfidenceProfile) || !reflect.DeepEqual(got.RiskReasons, resource.RiskReasons) {
		t.Fatalf("FindByID() profile/reasons = %#v/%#v", got.ConfidenceProfile, got.RiskReasons)
	}
}

func TestResourceRepositoryListsOnlyRequestedType(t *testing.T) {
	repository := newTestResourceRepository(t)
	windowsSDK := testResource(t, domain.ResourceTypeWindowsSDK, "10.0.22621.0")
	dotnetSDK := testResource(t, domain.ResourceTypeDotNetSDK, "8.0.100")
	for _, resource := range []domain.Resource{windowsSDK, dotnetSDK} {
		if err := repository.Upsert(context.Background(), resource); err != nil {
			t.Fatalf("Upsert() error = %v", err)
		}
	}

	got, err := repository.ListByType(context.Background(), domain.ResourceTypeWindowsSDK)
	if err != nil {
		t.Fatalf("ListByType() error = %v", err)
	}
	if len(got) != 1 || got[0].ID != windowsSDK.ID {
		t.Fatalf("ListByType() = %#v, want only Windows SDK", got)
	}
}

func TestResourceRepositoryListReturnsEveryType(t *testing.T) {
	repository := newTestResourceRepository(t)
	windowsSDK := testResource(t, domain.ResourceTypeWindowsSDK, "10.0.22621.0")
	dotnetSDK := testResource(t, domain.ResourceTypeDotNetSDK, "8.0.100")
	for _, resource := range []domain.Resource{windowsSDK, dotnetSDK} {
		if err := repository.Upsert(context.Background(), resource); err != nil {
			t.Fatalf("Upsert() error = %v", err)
		}
	}

	got, err := repository.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("List() returned %d resources, want 2", len(got))
	}
}

func TestResourceRepositoryRejectsInvalidStableID(t *testing.T) {
	repository := newTestResourceRepository(t)
	resource := testResource(t, domain.ResourceTypeWindowsSDK, "10.0.22621.0")
	resource.ID = "random-id"
	if err := repository.Upsert(context.Background(), resource); err == nil {
		t.Fatal("Upsert() error = nil, want stable ID validation error")
	}
}

func TestResourceRepositoryReturnsNotFound(t *testing.T) {
	repository := newTestResourceRepository(t)
	_, err := repository.FindByID(context.Background(), "missing")
	if !errors.Is(err, ErrResourceNotFound) {
		t.Fatalf("FindByID() error = %v, want ErrResourceNotFound", err)
	}
}

func newTestResourceRepository(t *testing.T) *ResourceRepository {
	t.Helper()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	return NewResourceRepository(db)
}

func testResource(t *testing.T, resourceType domain.ResourceType, version string) domain.Resource {
	t.Helper()
	displayPath := filepath.Join(t.TempDir(), string(resourceType), version)
	normalizedPath, err := pathutil.Normalize(displayPath)
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	modifiedAt := time.Date(2026, 7, 17, 1, 2, 3, 4, time.UTC)
	return domain.Resource{
		ID:              domain.ResourceID(resourceType, version, normalizedPath),
		Name:            string(resourceType),
		Type:            resourceType,
		Version:         version,
		DisplayPath:     displayPath,
		NormalizedPath:  normalizedPath,
		LogicalSize:     1024,
		SizeKnown:       true,
		ReclaimableSize: 0,
		Regenerable:     false,
		SystemManaged:   true,
		LastModifiedAt:  &modifiedAt,
		LastObservedAt:  modifiedAt.Add(time.Hour),
		Risk:            domain.RiskBlocked,
		Confidence:      90,
	}
}
