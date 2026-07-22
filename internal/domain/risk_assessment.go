package domain

import "time"

// ConfidenceProfile separates facts that the legacy scalar Confidence mixed
// together. Scores describe analysis coverage (0..100), never probability.
type ConfidenceProfile struct {
	ModelVersion   int                    `json:"model_version"`
	Classification int                    `json:"classification"`
	Ownership      int                    `json:"ownership"`
	Dependency     int                    `json:"dependency"`
	Regenerability int                    `json:"regenerability"`
	PathSafety     int                    `json:"path_safety"`
	ScanCoverage   int                    `json:"scan_coverage"`
	Freshness      int                    `json:"freshness"`
	Assessments    []ConfidenceAssessment `json:"assessments,omitempty"`
}

type ConfidenceStatus string

const (
	ConfidenceKnown      ConfidenceStatus = "KNOWN"
	ConfidencePartial    ConfidenceStatus = "PARTIAL"
	ConfidenceUnknown    ConfidenceStatus = "UNKNOWN"
	ConfidenceConflicted ConfidenceStatus = "CONFLICTED"
)

type ConfidenceAxis string

const (
	AxisClassification ConfidenceAxis = "CLASSIFICATION"
	AxisOwnership      ConfidenceAxis = "OWNERSHIP"
	AxisDependency     ConfidenceAxis = "DEPENDENCY"
	AxisRegenerability ConfidenceAxis = "REGENERABILITY"
	AxisPathSafety     ConfidenceAxis = "PATH_SAFETY"
	AxisScanCoverage   ConfidenceAxis = "SCAN_COVERAGE"
	AxisFreshness      ConfidenceAxis = "FRESHNESS"
)

type ClaimAssessment struct {
	Claim       ClaimType        `json:"claim"`
	Score       int              `json:"score"`
	Status      ConfidenceStatus `json:"status"`
	EvidenceIDs []string         `json:"evidence_ids,omitempty"`
	Explanation string           `json:"explanation,omitempty"`
}

type ConfidenceAssessment struct {
	Axis          ConfidenceAxis    `json:"axis"`
	Score         int               `json:"score"`
	Status        ConfidenceStatus  `json:"status"`
	LimitingClaim ClaimType         `json:"limiting_claim,omitempty"`
	Claims        []ClaimAssessment `json:"claims,omitempty"`
}

type ConfidenceSummary struct {
	Overall      int              `json:"overall"`
	Status       ConfidenceStatus `json:"status"`
	LimitingAxis ConfidenceAxis   `json:"limiting_axis"`
	Eligible     bool             `json:"eligible"`
}

func (p ConfidenceProfile) CleanupSummary() ConfidenceSummary {
	values := []struct {
		axis  ConfidenceAxis
		score int
	}{
		{AxisOwnership, p.Ownership},
		{AxisRegenerability, p.Regenerability},
		{AxisPathSafety, p.PathSafety},
		{AxisScanCoverage, p.ScanCoverage},
		{AxisFreshness, p.Freshness},
	}
	summary := ConfidenceSummary{Overall: values[0].score, LimitingAxis: values[0].axis, Status: ConfidenceKnown}
	for _, value := range values[1:] {
		if value.score < summary.Overall {
			summary.Overall, summary.LimitingAxis = value.score, value.axis
		}
	}
	if p.ModelVersion == 0 || len(p.Assessments) == 0 {
		summary.Status = ConfidencePartial
	}
	summary.Eligible = p.Classification > 0 && p.Dependency >= 80 && p.Ownership >= 90 &&
		p.Regenerability >= 90 && p.PathSafety >= 90 && p.ScanCoverage >= 80 && p.Freshness >= 80
	return summary
}

func (p ConfidenceProfile) IsZero() bool {
	return p.ModelVersion == 0 && p.Classification == 0 && p.Ownership == 0 && p.Dependency == 0 &&
		p.Regenerability == 0 && p.PathSafety == 0 && p.ScanCoverage == 0 &&
		p.Freshness == 0 && len(p.Assessments) == 0
}

func (p ConfidenceProfile) Overall() int {
	values := []int{p.Classification, p.Ownership, p.Dependency, p.Regenerability, p.PathSafety, p.ScanCoverage, p.Freshness}
	overall := values[0]
	for _, value := range values[1:] {
		if value < overall {
			overall = value
		}
	}
	return overall
}

func (p ConfidenceProfile) Valid() bool {
	if p.ModelVersion < 0 {
		return false
	}
	for _, value := range []int{p.Classification, p.Ownership, p.Dependency, p.Regenerability, p.PathSafety, p.ScanCoverage, p.Freshness} {
		if value < 0 || value > 100 {
			return false
		}
	}
	for _, assessment := range p.Assessments {
		if assessment.Score < 0 || assessment.Score > 100 {
			return false
		}
		for _, claim := range assessment.Claims {
			if claim.Score < 0 || claim.Score > 100 {
				return false
			}
		}
	}
	return true
}

type RiskReasonSeverity string

const (
	RiskReasonBlocker   RiskReasonSeverity = "BLOCKER"
	RiskReasonWarning   RiskReasonSeverity = "WARNING"
	RiskReasonSafeguard RiskReasonSeverity = "SAFEGUARD"
	RiskReasonUnknown   RiskReasonSeverity = "UNKNOWN"
)

type RiskReason struct {
	Code        string             `json:"code"`
	Severity    RiskReasonSeverity `json:"severity"`
	Message     string             `json:"message"`
	EvidenceID  string             `json:"evidence_id,omitempty"`
	Axis        ConfidenceAxis     `json:"axis,omitempty"`
	Scope       string             `json:"scope,omitempty"`
	Remediation string             `json:"remediation,omitempty"`
	EvidenceIDs []string           `json:"evidence_ids,omitempty"`
}

type CleanupDisposition string

const (
	DispositionAutoQuarantine  CleanupDisposition = "AUTO_QUARANTINE"
	DispositionManualReview    CleanupDisposition = "MANUAL_REVIEW"
	DispositionUseOfficialTool CleanupDisposition = "USE_OFFICIAL_TOOL"
	DispositionNeverDelete     CleanupDisposition = "NEVER_DELETE"
)

type VerificationStatus string

const (
	VerifiedTrue      VerificationStatus = "VERIFIED_TRUE"
	VerifiedFalse     VerificationStatus = "VERIFIED_FALSE"
	Unverified        VerificationStatus = "UNVERIFIED"
	VerificationError VerificationStatus = "ERROR"
)

type VerifiedFact struct {
	Status     VerificationStatus `json:"status"`
	EvidenceID string             `json:"evidence_id,omitempty"`
	CheckedAt  time.Time          `json:"checked_at,omitempty"`
}
