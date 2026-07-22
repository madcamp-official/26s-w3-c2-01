package app

import (
	"testing"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

func TestConfidenceProfileConnectsCleanupFactsToClaimAssessments(t *testing.T) {
	profile := confidenceProfile(domain.Resource{
		Confidence: 95, Regenerable: true,
		LastObservedAt: time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC),
	}, CleanupEvidence{
		ProjectOwned: true, KnownOutputPath: true,
		ReparsePointFree: true, GitTrackedOriginalsAbsent: true,
	})
	if profile.ModelVersion != 1 || len(profile.Assessments) != 7 {
		t.Fatalf("profile = %#v, want version 1 with seven assessments", profile)
	}
	if profile.Ownership != 90 || profile.Regenerability != 90 || profile.PathSafety != 90 {
		t.Fatalf("claim-derived scores = %d/%d/%d, want 90/90/90", profile.Ownership, profile.Regenerability, profile.PathSafety)
	}
	if !profile.CleanupSummary().Eligible {
		t.Fatalf("summary = %#v, want cleanup eligible", profile.CleanupSummary())
	}
}

func TestConfidenceProfileLeavesUnverifiedPathSafetyUnknown(t *testing.T) {
	profile := confidenceProfile(domain.Resource{Confidence: 95, Regenerable: true, LastObservedAt: time.Now()}, CleanupEvidence{
		ProjectOwned: true, KnownOutputPath: true,
	})
	if profile.PathSafety != 0 || profile.CleanupSummary().Eligible {
		t.Fatalf("profile = %#v, want unknown path safety and ineligible cleanup", profile)
	}
}
