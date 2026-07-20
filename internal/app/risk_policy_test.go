package app

import (
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

func TestDefaultRiskPolicyBlocksSystemManagedResource(t *testing.T) {
	assessment := (DefaultRiskPolicy{}).Classify(ResourceContext{
		Resource: domain.Resource{SystemManaged: true},
	})
	if assessment.Level != domain.RiskBlocked || len(assessment.Reasons) == 0 {
		t.Fatalf("Classify() = %#v, want BLOCKED with a reason", assessment)
	}
}

func TestDefaultRiskPolicyRequiresReviewWithoutSafetyEvidence(t *testing.T) {
	assessment := (DefaultRiskPolicy{}).Classify(ResourceContext{})
	if assessment.Level != domain.RiskReview {
		t.Fatalf("Classify() level = %q, want REVIEW", assessment.Level)
	}
}

func TestDefaultRiskPolicyMarksFullyVerifiedRegenerableArtifactSafe(t *testing.T) {
	assessment := (DefaultRiskPolicy{}).Classify(ResourceContext{
		Resource: domain.Resource{Regenerable: true},
		Cleanup: CleanupEvidence{
			ProjectOwned:              true,
			KnownOutputPath:           true,
			ReparsePointFree:          true,
			GitTrackedOriginalsAbsent: true,
		},
	})
	if assessment.Level != domain.RiskSafe || len(assessment.Reasons) == 0 {
		t.Fatalf("Classify() = %#v, want SAFE with a reason", assessment)
	}
}

func TestDefaultRiskPolicyRequiresEveryCleanupEvidenceFact(t *testing.T) {
	complete := CleanupEvidence{
		ProjectOwned:              true,
		KnownOutputPath:           true,
		ReparsePointFree:          true,
		GitTrackedOriginalsAbsent: true,
	}
	tests := []struct {
		name   string
		mutate func(*CleanupEvidence)
	}{
		{name: "project ownership", mutate: func(e *CleanupEvidence) { e.ProjectOwned = false }},
		{name: "known output path", mutate: func(e *CleanupEvidence) { e.KnownOutputPath = false }},
		{name: "reparse point check", mutate: func(e *CleanupEvidence) { e.ReparsePointFree = false }},
		{name: "Git tracked check", mutate: func(e *CleanupEvidence) { e.GitTrackedOriginalsAbsent = false }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evidence := complete
			tt.mutate(&evidence)
			assessment := (DefaultRiskPolicy{}).Classify(ResourceContext{
				Resource: domain.Resource{Regenerable: true}, Cleanup: evidence,
			})
			if assessment.Level != domain.RiskReview {
				t.Fatalf("Classify() level = %q, want REVIEW", assessment.Level)
			}
		})
	}
}

func TestDefaultRiskPolicyBlocksProtectedResourceDespiteCleanupEvidence(t *testing.T) {
	assessment := (DefaultRiskPolicy{}).Classify(ResourceContext{
		Resource:      domain.Resource{Regenerable: true},
		ProtectedPath: true,
		Cleanup: CleanupEvidence{
			ProjectOwned:              true,
			KnownOutputPath:           true,
			ReparsePointFree:          true,
			GitTrackedOriginalsAbsent: true,
		},
	})
	if assessment.Level != domain.RiskBlocked {
		t.Fatalf("Classify() level = %q, want BLOCKED", assessment.Level)
	}
}
