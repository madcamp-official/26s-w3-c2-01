package app

import (
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

const minimumAutoFreshness = 80

// FreshnessScore describes evidence age, not probability. Recent observations
// retain full strength; increasingly old observations are conservatively
// downgraded until a new scan refreshes them.
func FreshnessScore(observedAt, now time.Time) int {
	if observedAt.IsZero() {
		return 0
	}
	age := now.Sub(observedAt)
	if age < 0 || age <= 7*24*time.Hour {
		return 100
	}
	if age <= 30*24*time.Hour {
		return 80
	}
	if age <= 90*24*time.Hour {
		return 50
	}
	return 20
}

// ApplyFreshness derives the current view from persisted observation time.
// Staleness may lower SAFE to REVIEW, but never lowers BLOCKED.
func ApplyFreshness(resource domain.Resource, now time.Time) domain.Resource {
	if resource.ConfidenceProfile.IsZero() && resource.Confidence > 0 {
		resource.ConfidenceProfile = domain.ConfidenceProfile{
			Classification: resource.Confidence, Ownership: resource.Confidence,
			Dependency: resource.Confidence, Regenerability: resource.Confidence,
			PathSafety:   resource.Confidence,
			ScanCoverage: resource.Confidence, Freshness: resource.Confidence,
		}
	}
	resource.ConfidenceProfile.Freshness = FreshnessScore(resource.LastObservedAt, now)
	resource.Confidence = resource.ConfidenceProfile.Overall()
	if resource.Risk == domain.RiskSafe && resource.ConfidenceProfile.Freshness < minimumAutoFreshness {
		resource.Risk = domain.RiskReview
		resource.RiskReasons = append(resource.RiskReasons, domain.RiskReason{
			Code: "EVIDENCE_STALE", Severity: domain.RiskReasonUnknown,
			Message: "the latest resource observation is too old for automatic cleanup; run a new scan",
		})
	}
	return resource
}
