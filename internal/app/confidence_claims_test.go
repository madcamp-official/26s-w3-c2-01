package app

import (
	"testing"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

func TestAssessClaimDeduplicatesSourceFamiliesAndRewardsIndependentSources(t *testing.T) {
	now := time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)
	evidence := []domain.Evidence{
		{ID: "manifest-1", Claim: domain.ClaimOutputDeclared, Kind: domain.EvidenceDeclared, SourceFamily: "package-manifest"},
		{ID: "manifest-2", Claim: domain.ClaimOutputDeclared, Kind: domain.EvidenceObserved, SourceFamily: "package-manifest"},
		{ID: "build", Claim: domain.ClaimOutputDeclared, Kind: domain.EvidenceResolved, SourceFamily: "build-run"},
	}
	got := AssessClaim(domain.ClaimOutputDeclared, evidence, now)
	if got.Score != 92 || got.Status != domain.ConfidenceKnown {
		t.Fatalf("assessment = %#v, want KNOWN score 92", got)
	}
}

func TestAssessClaimCapsContradictionAndStaleEvidence(t *testing.T) {
	now := time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)
	expired := now.Add(-time.Hour)
	evidence := []domain.Evidence{
		{ID: "support", Claim: domain.ClaimProjectOwnership, Kind: domain.EvidenceResolved, SourceFamily: "manifest", ValidUntil: &expired},
		{ID: "contradiction", Claim: domain.ClaimProjectOwnership, Kind: domain.EvidenceObserved, Polarity: domain.EvidenceContradicts},
	}
	got := AssessClaim(domain.ClaimProjectOwnership, evidence, now)
	if got.Score != 30 || got.Status != domain.ConfidenceConflicted {
		t.Fatalf("assessment = %#v, want CONFLICTED score 30", got)
	}
}

func TestAssessAxisUsesLimitingRequiredClaim(t *testing.T) {
	now := time.Now()
	evidence := []domain.Evidence{
		{ID: "output", Claim: domain.ClaimOutputDeclared, Kind: domain.EvidenceResolved},
		{ID: "command", Claim: domain.ClaimBuildCommandKnown, Kind: domain.EvidenceInferred},
	}
	got := AssessAxis(domain.AxisRegenerability, []domain.ClaimType{domain.ClaimOutputDeclared, domain.ClaimBuildCommandKnown}, evidence, now)
	if got.Score != 40 || got.LimitingClaim != domain.ClaimBuildCommandKnown {
		t.Fatalf("assessment = %#v, want score 40 limited by build command", got)
	}
}

func TestAssessClaimTreatsUnknownKindAsEvaluationState(t *testing.T) {
	now := time.Now()
	unknown := AssessClaim(domain.ClaimProjectOwnership, []domain.Evidence{{
		ID: "unknown", Claim: domain.ClaimProjectOwnership, Kind: domain.EvidenceUnknown,
	}}, now)
	if unknown.Score != 0 || unknown.Status != domain.ConfidenceUnknown {
		t.Fatalf("unknown assessment = %#v, want UNKNOWN score 0", unknown)
	}

	partial := AssessClaim(domain.ClaimProjectOwnership, []domain.Evidence{
		{ID: "known", Claim: domain.ClaimProjectOwnership, Kind: domain.EvidenceResolved},
		{ID: "unknown", Claim: domain.ClaimProjectOwnership, Kind: domain.EvidenceUnknown},
	}, now)
	if partial.Score != 90 || partial.Status != domain.ConfidencePartial {
		t.Fatalf("mixed assessment = %#v, want PARTIAL score 90", partial)
	}
}

func TestAssessClaimContradictionWithoutSupportIsConflicted(t *testing.T) {
	got := AssessClaim(domain.ClaimProjectOwnership, []domain.Evidence{{
		ID: "negative", Claim: domain.ClaimProjectOwnership, Kind: domain.EvidenceObserved,
		Polarity: domain.EvidenceContradicts,
	}}, time.Now())
	if got.Score != 0 || got.Status != domain.ConfidenceConflicted {
		t.Fatalf("assessment = %#v, want CONFLICTED score 0", got)
	}
}
