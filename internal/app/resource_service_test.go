package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/safety"
	"github.com/madcamp-official/26s-w3-c2-01/internal/scanner"
)

func TestResourceServiceObservesClassifiesAndPersists(t *testing.T) {
	protectedRoot := t.TempDir()
	resourcePath := filepath.Join(protectedRoot, "Windows Kits", "10")
	if err := os.MkdirAll(resourcePath, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(resourcePath, "sdk.bin"), []byte("123456"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	classifier, err := safety.NewPathClassifier([]string{protectedRoot})
	if err != nil {
		t.Fatalf("NewPathClassifier() error = %v", err)
	}
	repository := &resourceRepositoryStub{}
	service := NewResourceService(scanner.New(2), repository, classifier, DefaultRiskPolicy{})
	observedAt := time.Date(2026, 7, 18, 3, 4, 5, 6, time.UTC)
	service.now = func() time.Time { return observedAt }

	got, err := service.Observe(context.Background(), ResourceObservationInput{
		Resource: domain.Resource{
			Name:        "Windows SDK 10.0.22621.0",
			Type:        domain.ResourceTypeWindowsSDK,
			Version:     "10.0.22621.0",
			DisplayPath: resourcePath,
			Confidence:  90,
		},
	})
	if err != nil {
		t.Fatalf("Observe() error = %v", err)
	}
	if got.Resource.ID == "" || got.Resource.NormalizedPath == "" {
		t.Fatalf("Observe() identity = %#v, want populated ID and normalized path", got.Resource)
	}
	if got.Resource.LogicalSize != 6 || !got.Resource.SizeKnown || !got.Resource.SystemManaged || got.Resource.Risk != domain.RiskBlocked {
		t.Fatalf("Observe() resource = %#v, want measured BLOCKED system resource", got.Resource)
	}
	if !got.Resource.LastObservedAt.Equal(observedAt) || got.Resource.ReclaimableSize != 0 {
		t.Fatalf("Observe() timestamps/size = %#v", got.Resource)
	}
	if repository.saved.ID != got.Resource.ID {
		t.Fatalf("persisted resource = %#v, want observed resource", repository.saved)
	}
	if len(got.Reasons) == 0 || len(got.Issues) != 0 {
		t.Fatalf("Observe() reasons/issues = %v/%v", got.Reasons, got.Issues)
	}
}

func TestResourceServiceSetsMeasuredSizeReclaimableForSafeArtifact(t *testing.T) {
	resourcePath := filepath.Join(t.TempDir(), "node_modules")
	if err := os.Mkdir(resourcePath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(resourcePath, "index.js"), []byte("1234"), 0o644); err != nil {
		t.Fatal(err)
	}
	classifier, err := safety.NewPathClassifier(nil)
	if err != nil {
		t.Fatal(err)
	}
	repository := &resourceRepositoryStub{}
	service := NewResourceService(scanner.New(1), repository, classifier, DefaultRiskPolicy{})

	got, err := service.Observe(context.Background(), ResourceObservationInput{
		Resource: domain.Resource{
			Name: "node_modules", Type: domain.ResourceTypeNodeModules,
			DisplayPath: resourcePath, Regenerable: true,
		},
		Cleanup: CleanupEvidence{
			ProjectOwned:              true,
			KnownOutputPath:           true,
			ReparsePointFree:          true,
			GitTrackedOriginalsAbsent: true,
		},
	})
	if err != nil {
		t.Fatalf("Observe() error = %v", err)
	}
	if got.Resource.Risk != domain.RiskSafe || got.Resource.LogicalSize != 4 || got.Resource.ReclaimableSize != 4 {
		t.Fatalf("Observe() resource = %#v, want measured SAFE resource", got.Resource)
	}
}

func TestResourceServiceUsesPremeasuredDockerUsageWithoutWalkingCLIPath(t *testing.T) {
	dockerPath := filepath.Join(t.TempDir(), "docker")
	if err := os.WriteFile(dockerPath, []byte("cli"), 0o700); err != nil {
		t.Fatal(err)
	}
	classifier, err := safety.NewPathClassifier(nil)
	if err != nil {
		t.Fatal(err)
	}
	repository := &resourceRepositoryStub{}
	service := NewResourceService(nil, repository, classifier, DefaultRiskPolicy{})

	got, err := service.Observe(context.Background(), ResourceObservationInput{Resource: domain.Resource{
		Name: "Docker Build Cache", Type: domain.ResourceTypeDockerCache, Version: "build-cache",
		DisplayPath: dockerPath, LogicalSize: 2_000, SizeKnown: true, ReclaimableSize: 1_500,
		Confidence: domain.DefaultConfidence[domain.EvidenceResolved],
	}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Resource.LogicalSize != 2_000 || got.Resource.ReclaimableSize != 1_500 || got.Resource.Risk != domain.RiskReview {
		t.Fatalf("resource = %#v, want premeasured REVIEW Docker usage", got.Resource)
	}
}

func TestResourceServiceReclassifyRequiredBlocksAnUnclassifiedResource(t *testing.T) {
	repository := &resourceRepositoryStub{
		byID: domain.Resource{
			ID: "resource-1", Type: domain.ResourceTypeWindowsSDK,
			Risk: domain.RiskReview, LogicalSize: 100, ReclaimableSize: 100,
		},
	}
	// ReclassifyRequired never touches the classifier (the resource is
	// already-classified), so a nil ResourcePathClassifier is fine here.
	service := NewResourceService(scanner.New(1), repository, nil, DefaultRiskPolicy{})

	got, err := service.ReclassifyRequired(context.Background(), "resource-1")
	if err != nil {
		t.Fatalf("ReclassifyRequired() error = %v", err)
	}
	if got.Resource.Risk != domain.RiskBlocked || got.Resource.ReclaimableSize != 0 {
		t.Fatalf("ReclassifyRequired() resource = %#v, want BLOCKED with ReclaimableSize 0", got.Resource)
	}
	if repository.saved.ID != "resource-1" || repository.saved.Risk != domain.RiskBlocked {
		t.Fatalf("persisted resource = %#v, want the reclassified BLOCKED resource", repository.saved)
	}
	if len(got.Reasons) == 0 {
		t.Error("ReclassifyRequired() gave no reason for the BLOCKED verdict")
	}
}

func TestResourceServiceReclassifyRequiredLeavesAlreadyBlockedResourceAlone(t *testing.T) {
	repository := &resourceRepositoryStub{
		byID: domain.Resource{ID: "resource-1", Risk: domain.RiskBlocked},
	}
	// ReclassifyRequired never touches the classifier (the resource is
	// already-classified), so a nil ResourcePathClassifier is fine here.
	service := NewResourceService(scanner.New(1), repository, nil, DefaultRiskPolicy{})

	if _, err := service.ReclassifyRequired(context.Background(), "resource-1"); err != nil {
		t.Fatalf("ReclassifyRequired() error = %v", err)
	}
	if repository.saved.ID != "" {
		t.Fatalf("ReclassifyRequired() re-persisted an already-BLOCKED resource: %#v", repository.saved)
	}
}

type resourceRepositoryStub struct {
	saved domain.Resource
	byID  domain.Resource
}

func (r *resourceRepositoryStub) Upsert(_ context.Context, resource domain.Resource) error {
	r.saved = resource
	return nil
}

func (r *resourceRepositoryStub) FindByID(context.Context, string) (domain.Resource, error) {
	return r.byID, nil
}

func (*resourceRepositoryStub) ListByType(context.Context, domain.ResourceType) ([]domain.Resource, error) {
	return nil, errors.New("not implemented")
}

func (*resourceRepositoryStub) List(context.Context) ([]domain.Resource, error) {
	return nil, errors.New("not implemented")
}
