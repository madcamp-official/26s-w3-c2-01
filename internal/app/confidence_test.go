package app

import (
	"testing"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

func TestFreshnessScoreUsesConservativeAgeBands(t *testing.T) {
	now := time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		at   time.Time
		want int
	}{
		{"recent", now.Add(-7 * 24 * time.Hour), 100},
		{"aging", now.Add(-30 * 24 * time.Hour), 80},
		{"stale", now.Add(-31 * 24 * time.Hour), 50},
		{"very stale", now.Add(-91 * 24 * time.Hour), 20},
		{"unknown", time.Time{}, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := FreshnessScore(test.at, now); got != test.want {
				t.Fatalf("FreshnessScore() = %d, want %d", got, test.want)
			}
		})
	}
}

func TestApplyFreshnessDowngradesStaleSafeResource(t *testing.T) {
	now := time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
	resource := domain.Resource{
		Risk: domain.RiskSafe, LastObservedAt: now.Add(-31 * 24 * time.Hour),
		ConfidenceProfile: domain.ConfidenceProfile{
			Classification: 100, Ownership: 100, Dependency: 100,
			Regenerability: 100, PathSafety: 100, ScanCoverage: 100, Freshness: 100,
		},
	}
	got := ApplyFreshness(resource, now)
	if got.Risk != domain.RiskReview || got.Confidence != 50 {
		t.Fatalf("ApplyFreshness() risk/confidence = %s/%d, want REVIEW/50", got.Risk, got.Confidence)
	}
	if len(got.RiskReasons) != 1 || got.RiskReasons[0].Code != "EVIDENCE_STALE" {
		t.Fatalf("risk reasons = %#v, want EVIDENCE_STALE", got.RiskReasons)
	}
}

func TestApplyFreshnessNeverLowersBlockedDecision(t *testing.T) {
	now := time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
	resource := domain.Resource{Risk: domain.RiskBlocked, LastObservedAt: now.Add(-365 * 24 * time.Hour)}
	if got := ApplyFreshness(resource, now); got.Risk != domain.RiskBlocked {
		t.Fatalf("risk = %s, want BLOCKED", got.Risk)
	}
}
