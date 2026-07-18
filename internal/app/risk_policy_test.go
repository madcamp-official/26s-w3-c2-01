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
