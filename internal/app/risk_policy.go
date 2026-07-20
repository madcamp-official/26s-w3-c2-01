package app

import "github.com/madcamp-official/26s-w3-c2-01/internal/domain"

type ResourceContext struct {
	Resource      domain.Resource
	ProtectedPath bool
	// RequiredByProject is true when the dependency graph shows at least one
	// scanned project currently depends on this resource (e.g. a project
	// declares the exact Windows SDK version installed here). It can only be
	// known once dependency resolution has run, which is after this
	// resource's first Classify pass -- see
	// AnalysisOrchestrator.Run/ResourceService.ReclassifyRequired.
	RequiredByProject bool
	Cleanup           CleanupEvidence
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

// DefaultRiskPolicy is conservative: protected/system-managed resources and
// resources a scanned project currently depends on are blocked, fully
// verified and regenerable project artifacts are safe, and every incomplete
// evidence set requires review.
type DefaultRiskPolicy struct{}

func (DefaultRiskPolicy) Classify(context ResourceContext) RiskAssessment {
	if context.ProtectedPath || context.Resource.SystemManaged {
		return RiskAssessment{
			Level:   domain.RiskBlocked,
			Reasons: []string{"resource is inside an operating-system managed path"},
		}
	}
	if context.RequiredByProject {
		return RiskAssessment{
			Level:   domain.RiskBlocked,
			Reasons: []string{"a scanned project currently depends on this resource"},
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
