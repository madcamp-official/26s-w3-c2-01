package app

import (
	"context"
	"errors"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

type fakeProjectRepository struct {
	projects []domain.BuildProject
}

func (r fakeProjectRepository) UpsertObserved(context.Context, string, []domain.BuildProject) error {
	return errors.New("not implemented")
}
func (r fakeProjectRepository) FindByID(context.Context, string) (domain.BuildProject, error) {
	return domain.BuildProject{}, errors.New("not implemented")
}
func (r fakeProjectRepository) FindByManifestPath(context.Context, domain.ProjectType, string) (domain.BuildProject, error) {
	return domain.BuildProject{}, errors.New("not implemented")
}
func (r fakeProjectRepository) List(context.Context) ([]domain.BuildProject, error) {
	return r.projects, nil
}

type fakeResourceRepository struct {
	resources []domain.Resource
}

func (r fakeResourceRepository) Upsert(context.Context, domain.Resource) error {
	return errors.New("not implemented")
}
func (r fakeResourceRepository) FindByID(context.Context, string) (domain.Resource, error) {
	return domain.Resource{}, errors.New("not implemented")
}
func (r fakeResourceRepository) ListByType(context.Context, domain.ResourceType) ([]domain.Resource, error) {
	return nil, errors.New("not implemented")
}
func (r fakeResourceRepository) List(context.Context) ([]domain.Resource, error) {
	return r.resources, nil
}

func TestSummaryServiceAggregatesByTypeAndRisk(t *testing.T) {
	projects := []domain.BuildProject{{ID: "p1"}, {ID: "p2"}}
	resources := []domain.Resource{
		{Type: domain.ResourceTypeNodeModules, LogicalSize: 100, ReclaimableSize: 100, Risk: domain.RiskSafe},
		{Type: domain.ResourceTypeNodeModules, LogicalSize: 50, ReclaimableSize: 50, Risk: domain.RiskSafe},
		{Type: domain.ResourceTypeWindowsSDK, LogicalSize: 200, ReclaimableSize: 0, Risk: domain.RiskBlocked},
		{Type: domain.ResourceTypeGlobalCache, LogicalSize: 30, ReclaimableSize: 30, Risk: domain.RiskReview},
	}

	service := NewSummaryService(fakeProjectRepository{projects: projects}, fakeResourceRepository{resources: resources})
	summary, err := service.Summarize(context.Background(), nil)
	if err != nil {
		t.Fatalf("Summarize() error = %v", err)
	}

	if summary.ProjectCount != 2 || summary.ResourceCount != 4 {
		t.Fatalf("counts = %d/%d, want 2/4", summary.ProjectCount, summary.ResourceCount)
	}
	if summary.SafeReclaimable != 150 || summary.NeedsReview != 30 || summary.Blocked != 200 {
		t.Fatalf("risk buckets = %d/%d/%d, want 150/30/200", summary.SafeReclaimable, summary.NeedsReview, summary.Blocked)
	}
	if len(summary.ResourcesByType) != 3 {
		t.Fatalf("ResourcesByType = %#v, want 3 entries", summary.ResourcesByType)
	}
	for _, line := range summary.ResourcesByType {
		if line.Type == domain.ResourceTypeNodeModules && line.Bytes != 150 {
			t.Fatalf("node_modules bytes = %d, want 150", line.Bytes)
		}
	}
}

func TestSummaryServiceAppliesFilter(t *testing.T) {
	resources := []domain.Resource{
		{Type: domain.ResourceTypeNodeModules, LogicalSize: 100, Risk: domain.RiskSafe},
		{Type: domain.ResourceTypeWindowsSDK, LogicalSize: 200, Risk: domain.RiskBlocked},
	}
	service := NewSummaryService(fakeProjectRepository{}, fakeResourceRepository{resources: resources})
	summary, err := service.Summarize(context.Background(), func(r domain.Resource) bool {
		return r.Type == domain.ResourceTypeNodeModules
	})
	if err != nil {
		t.Fatalf("Summarize() error = %v", err)
	}
	if summary.ResourceCount != 1 || len(summary.ResourcesByType) != 1 {
		t.Fatalf("filtered summary = %#v", summary)
	}
}
