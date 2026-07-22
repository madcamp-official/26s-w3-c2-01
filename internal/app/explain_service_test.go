package app

import (
	"context"
	"testing"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

// TestExplainResourceAppliesFreshness locks down that ExplainResource
// recomputes from the persisted observation time, same as `libra
// resources`/`libra plan` (resource_list_service.go, plan_service.go) --
// before this, explain skipped ApplyFreshness entirely, so a resource that
// every other read path had already downgraded to REVIEW with an
// EVIDENCE_STALE reason still showed as SAFE when explained directly.
func TestExplainResourceAppliesFreshness(t *testing.T) {
	now := time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)
	resources := &resourceRepositoryStub{byID: domain.Resource{
		ID: "resource-1", Risk: domain.RiskSafe, LastObservedAt: now.Add(-31 * 24 * time.Hour),
		ConfidenceProfile: domain.ConfidenceProfile{
			Classification: 100, Ownership: 100, Dependency: 100,
			Regenerability: 100, PathSafety: 100, ScanCoverage: 100, Freshness: 100,
		},
	}}
	dependencies := &dependencyRepositoryStub{byResource: map[string][]domain.Dependency{}}

	service := NewExplainService(resources, &planProjectRepositoryStub{}, dependencies)
	service.now = func() time.Time { return now }

	got, err := service.ExplainResource(context.Background(), "resource-1")
	if err != nil {
		t.Fatalf("ExplainResource() error = %v", err)
	}
	if got.Resource.Risk != domain.RiskReview {
		t.Fatalf("Resource.Risk = %s, want REVIEW (stale SAFE resource must be downgraded)", got.Resource.Risk)
	}
	found := false
	for _, reason := range got.Resource.RiskReasons {
		if reason.Code == "EVIDENCE_STALE" {
			found = true
		}
	}
	if !found {
		t.Fatalf("RiskReasons = %#v, want EVIDENCE_STALE", got.Resource.RiskReasons)
	}
	if got.Resource.ConfidenceProfile.Freshness != 50 {
		t.Fatalf("ConfidenceProfile.Freshness = %d, want 50 (recomputed from LastObservedAt, not the stale persisted 100)", got.Resource.ConfidenceProfile.Freshness)
	}
}
