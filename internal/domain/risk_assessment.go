package domain

// ConfidenceProfile separates facts that the legacy scalar Confidence mixed
// together. Scores describe analysis coverage (0..100), never probability.
type ConfidenceProfile struct {
	Classification int `json:"classification"`
	Ownership      int `json:"ownership"`
	Dependency     int `json:"dependency"`
	CleanupSafety  int `json:"cleanup_safety"`
	ScanCoverage   int `json:"scan_coverage"`
}

func (p ConfidenceProfile) Overall() int {
	values := []int{p.Classification, p.Ownership, p.Dependency, p.CleanupSafety, p.ScanCoverage}
	overall := values[0]
	for _, value := range values[1:] {
		if value < overall {
			overall = value
		}
	}
	return overall
}

func (p ConfidenceProfile) Valid() bool {
	for _, value := range []int{p.Classification, p.Ownership, p.Dependency, p.CleanupSafety, p.ScanCoverage} {
		if value < 0 || value > 100 {
			return false
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
	Code       string             `json:"code"`
	Severity   RiskReasonSeverity `json:"severity"`
	Message    string             `json:"message"`
	EvidenceID string             `json:"evidence_id,omitempty"`
}
