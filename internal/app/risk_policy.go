package app

import "github.com/madcamp-official/26s-w3-c2-01/internal/domain"

type ResourceContext struct {
	Resource      domain.Resource
	ProtectedPath bool
	Cleanup       CleanupEvidence
}

// CleanupEvidence records the independent safety facts required before a
// project artifact may be classified SAFE. Zero values mean unverified, not
// false evidence.
type CleanupEvidence struct {
	ProjectOwned              bool
	KnownOutputPath           bool
	ReparsePointFree          bool
	GitTrackedOriginalsAbsent bool
}

func (e CleanupEvidence) complete() bool {
	return e.ProjectOwned && e.KnownOutputPath && e.ReparsePointFree && e.GitTrackedOriginalsAbsent
}

type RiskAssessment struct {
	Level   domain.RiskLevel
	Reasons []string
}

type RiskPolicy interface {
	Classify(ResourceContext) RiskAssessment
}

// DefaultRiskPolicy is conservative: protected/system-managed resources are
// blocked, fully verified and regenerable project artifacts are safe, and
// every incomplete evidence set requires review.
type DefaultRiskPolicy struct{}

func (DefaultRiskPolicy) Classify(context ResourceContext) RiskAssessment {
	if context.ProtectedPath || context.Resource.SystemManaged {
		return RiskAssessment{
			Level:   domain.RiskBlocked,
			Reasons: []string{"resource is inside an operating-system managed path"},
		}
	}
	if context.Resource.Regenerable && context.Cleanup.complete() {
		return RiskAssessment{
			Level:   domain.RiskSafe,
			Reasons: []string{"project artifact is regenerable and all cleanup evidence is verified"},
		}
	}
	return RiskAssessment{
		Level:   domain.RiskReview,
		Reasons: []string{"cleanup safety has not been fully verified"},
	}
}
