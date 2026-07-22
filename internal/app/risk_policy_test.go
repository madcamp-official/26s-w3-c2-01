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

func TestDefaultRiskPolicyBlocksAndroidSDK(t *testing.T) {
	assessment := (DefaultRiskPolicy{}).Classify(ResourceContext{Resource: domain.Resource{Type: domain.ResourceTypeAndroidSDK}})
	if assessment.Level != domain.RiskBlocked || len(assessment.Blockers) != 1 || assessment.Blockers[0].Code != "ANDROID_SDK_MANAGED" {
		t.Fatalf("assessment = %#v", assessment)
	}
}

func TestDefaultRiskPolicyProvidesOfficialCacheCleanupGuidance(t *testing.T) {
	for _, version := range []string{"gradle", "cargo-registry", "maven", "npm", "pnpm", "xcode-deriveddata", "cocoapods-cache", "swiftpm-cache", "homebrew-cache", "simulator-cache", "simulator-devices"} {
		assessment := (DefaultRiskPolicy{}).Classify(ResourceContext{Resource: domain.Resource{Type: domain.ResourceTypeGlobalCache, Version: version}})
		if assessment.Level != domain.RiskReview || len(assessment.Warnings) != 1 || assessment.Warnings[0].Code != "OFFICIAL_CLEANUP_GUIDANCE" {
			t.Fatalf("%s assessment = %#v", version, assessment)
		}
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

func TestDefaultRiskPolicyAllowsRegenerableProjectLocalDependency(t *testing.T) {
	assessment := (DefaultRiskPolicy{}).Classify(ResourceContext{
		Resource:          domain.Resource{Type: domain.ResourceTypeNodeModules, Regenerable: true},
		RequiredByProject: true,
		Cleanup: CleanupEvidence{Verification: CleanupVerification{
			ProjectOwned:              domain.VerifiedFact{Status: domain.VerifiedTrue},
			KnownOutputPath:           domain.VerifiedFact{Status: domain.VerifiedTrue},
			ReparsePointFree:          domain.VerifiedFact{Status: domain.VerifiedTrue},
			GitTrackedOriginalsAbsent: domain.VerifiedFact{Status: domain.VerifiedTrue},
		}},
	})
	if assessment.Level != domain.RiskSafe || assessment.Disposition != domain.DispositionAutoQuarantine {
		t.Fatalf("assessment = %#v, want SAFE/AUTO_QUARANTINE", assessment)
	}
}

func TestDefaultRiskPolicySeparatesDispositionFromRisk(t *testing.T) {
	cache := (DefaultRiskPolicy{}).Classify(ResourceContext{Resource: domain.Resource{Type: domain.ResourceTypeGlobalCache}})
	if cache.Level != domain.RiskReview || cache.Disposition != domain.DispositionUseOfficialTool {
		t.Fatalf("assessment = %#v, want REVIEW/USE_OFFICIAL_TOOL", cache)
	}
}

func TestDefaultRiskPolicyReviewsInferredOrInactiveDependency(t *testing.T) {
	for _, impact := range []DependencyImpact{
		{RequiredByProjects: 1, ActiveProjects: 1, RelationStrength: domain.ConfidenceAssessment{Status: domain.ConfidencePartial}},
		{RequiredByProjects: 1, ActiveProjects: 0, RelationStrength: domain.ConfidenceAssessment{Status: domain.ConfidenceKnown}},
	} {
		assessment := (DefaultRiskPolicy{}).Classify(ResourceContext{DependencyImpact: impact})
		if assessment.Level != domain.RiskReview || assessment.Disposition != domain.DispositionManualReview {
			t.Fatalf("assessment = %#v, want REVIEW/MANUAL_REVIEW", assessment)
		}
	}
}

func TestDefaultRiskPolicyDispositionInvariants(t *testing.T) {
	contexts := []ResourceContext{
		{Resource: domain.Resource{Type: domain.ResourceTypeDockerVolume}},
		{Resource: domain.Resource{Type: domain.ResourceTypeGlobalCache}},
		{Resource: domain.Resource{Regenerable: true}, Cleanup: CleanupEvidence{
			ProjectOwned: true, KnownOutputPath: true, ReparsePointFree: true, GitTrackedOriginalsAbsent: true,
		}},
		{CriticalUnknowns: []domain.RiskReason{{Code: "UNKNOWN", Severity: domain.RiskReasonUnknown}}},
	}
	for _, context := range contexts {
		assessment := (DefaultRiskPolicy{}).Classify(context)
		if assessment.Level == domain.RiskBlocked && assessment.Disposition == domain.DispositionAutoQuarantine {
			t.Fatalf("BLOCKED assessment permits AUTO_QUARANTINE: %#v", assessment)
		}
		if assessment.Level == domain.RiskSafe && assessment.Disposition != domain.DispositionAutoQuarantine {
			t.Fatalf("SAFE assessment has invalid disposition: %#v", assessment)
		}
		if assessment.Disposition == domain.DispositionNeverDelete && assessment.Level == domain.RiskSafe {
			t.Fatalf("NEVER_DELETE assessment is SAFE: %#v", assessment)
		}
		if len(context.CriticalUnknowns) > 0 && assessment.Level == domain.RiskSafe {
			t.Fatalf("critical unknown produced SAFE: %#v", assessment)
		}
	}
}

func TestCleanupEvidenceNormalizeFallsBackPerField(t *testing.T) {
	got := (CleanupEvidence{
		ProjectOwned: true, KnownOutputPath: true,
		Verification: CleanupVerification{ProjectOwned: domain.VerifiedFact{Status: domain.VerifiedTrue}},
	}).Normalize()
	if got.KnownOutputPath.Status != domain.VerifiedTrue || got.ReparsePointFree.Status != domain.Unverified {
		t.Fatalf("Normalize() = %#v, want per-field legacy fallback", got)
	}
}
