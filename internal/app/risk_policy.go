package app

import "github.com/madcamp-official/26s-w3-c2-01/internal/domain"

type ResourceContext struct {
	Resource      domain.Resource
	ProtectedPath bool
}

type RiskAssessment struct {
	Level   domain.RiskLevel
	Reasons []string
}

type RiskPolicy interface {
	Classify(ResourceContext) RiskAssessment
}

// DefaultRiskPolicy is conservative: protected/system-managed resources are
// blocked, and resources without enough cleanup evidence require review.
type DefaultRiskPolicy struct{}

func (DefaultRiskPolicy) Classify(context ResourceContext) RiskAssessment {
	if context.ProtectedPath || context.Resource.SystemManaged {
		return RiskAssessment{
			Level:   domain.RiskBlocked,
			Reasons: []string{"resource is inside an operating-system managed path"},
		}
	}
	return RiskAssessment{
		Level:   domain.RiskReview,
		Reasons: []string{"cleanup safety has not been fully verified"},
	}
}
