package app

import (
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

func TestDefaultRiskPolicyBlocksSystemManagedResource(t *testing.T) {
	assessment := (DefaultRiskPolicy{}).Classify(ResourceContext{
		Resource: domain.Resource{SystemManaged: true},
	})
	if assessment.Level != domain.RiskBlocked || len(assessment.Reasons()) == 0 {
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
	if assessment.Level != domain.RiskSafe || len(assessment.Reasons()) == 0 {
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

func TestDefaultRiskPolicyBlocksResourceRequiredByProjectDespiteCleanupEvidence(t *testing.T) {
	assessment := (DefaultRiskPolicy{}).Classify(ResourceContext{
		Resource:          domain.Resource{Regenerable: true},
		RequiredByProject: true,
		Cleanup: CleanupEvidence{
			ProjectOwned:              true,
			KnownOutputPath:           true,
			ReparsePointFree:          true,
			GitTrackedOriginalsAbsent: true,
		},
	})
	if assessment.Level != domain.RiskBlocked || len(assessment.Reasons()) == 0 {
		t.Fatalf("Classify() = %#v, want BLOCKED with a reason (a project depends on this resource)", assessment)
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

func TestDefaultRiskPolicyCriticalUnknownForcesReview(t *testing.T) {
	assessment := (DefaultRiskPolicy{}).Classify(ResourceContext{
		Resource: domain.Resource{Regenerable: true},
		Cleanup:  CleanupEvidence{ProjectOwned: true, KnownOutputPath: true, ReparsePointFree: true, GitTrackedOriginalsAbsent: true},
		CriticalUnknowns: []domain.RiskReason{{
			Code: "SCAN_COVERAGE_INCOMPLETE", Severity: domain.RiskReasonUnknown,
			Message: "one configured root could not be scanned",
		}},
	})
	if assessment.Level != domain.RiskReview || len(assessment.Unknowns) != 1 {
		t.Fatalf("Classify() = %#v, want REVIEW with structured unknown", assessment)
	}
}

func TestDefaultRiskPolicyKeepsDockerCleanupInOfficialTools(t *testing.T) {
	cache := (DefaultRiskPolicy{}).Classify(ResourceContext{Resource: domain.Resource{Type: domain.ResourceTypeDockerCache}})
	if cache.Level != domain.RiskReview || cache.Warnings[0].Code != "DOCKER_OFFICIAL_CLEANUP_REQUIRED" {
		t.Fatalf("Docker cache assessment = %#v", cache)
	}
	volume := (DefaultRiskPolicy{}).Classify(ResourceContext{Resource: domain.Resource{Type: domain.ResourceTypeDockerVolume}})
	if volume.Level != domain.RiskBlocked || volume.Blockers[0].Code != "DOCKER_VOLUME_USER_DATA" {
		t.Fatalf("Docker volume assessment = %#v", volume)
	}
}
