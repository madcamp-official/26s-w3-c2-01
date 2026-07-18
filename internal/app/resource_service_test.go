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

	got, err := service.Observe(context.Background(), domain.Resource{
		Name:        "Windows SDK 10.0.22621.0",
		Type:        domain.ResourceTypeWindowsSDK,
		Version:     "10.0.22621.0",
		DisplayPath: resourcePath,
		Confidence:  90,
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

type resourceRepositoryStub struct {
	saved domain.Resource
}

func (r *resourceRepositoryStub) Upsert(_ context.Context, resource domain.Resource) error {
	r.saved = resource
	return nil
}

func (*resourceRepositoryStub) FindByID(context.Context, string) (domain.Resource, error) {
	return domain.Resource{}, errors.New("not implemented")
}

func (*resourceRepositoryStub) ListByType(context.Context, domain.ResourceType) ([]domain.Resource, error) {
	return nil, errors.New("not implemented")
}

func (*resourceRepositoryStub) List(context.Context) ([]domain.Resource, error) {
	return nil, errors.New("not implemented")
}
