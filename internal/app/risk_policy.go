// [파일 역할] 리소스를 SAFE/REVIEW/BLOCKED로 분류하는 정책인 RiskPolicy
// 인터페이스와 그 기본 구현체 DefaultRiskPolicy를 담고 있는 파일이다.
// resource_service.go의 ResourceService.Observe가 리소스를 저장하기 직전에
// 이 정책을 호출해 domain.Resource.Risk를 채운다. 알려진 한계: 아래
// DefaultRiskPolicy.Classify는 domain.Resource.Regenerable을 전혀 참조하지
// 않아 domain.RiskSafe를 절대 반환하지 않는다(모든 리소스가 REVIEW 또는
// BLOCKED) — docs/libra_review_findings_day4.md §4에 기록된 이슈이며,
// `libra summary`의 "Safely reclaimable" 합계(summary_service.go의
// Summary.SafeReclaimable)가 항상 0이 되는 원인이다.
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
//
// Known gap (see docs/libra_review_findings_day4.md §4): Classify below
// never returns domain.RiskSafe -- it ignores Resource.Regenerable
// entirely, even though §20.3 of docs/libra_integration_contracts.md's
// CONFIRMED decision table says a project-owned, clearly-regenerable
// artifact should be SAFE. Every resource today is REVIEW or BLOCKED, so
// `libra summary`'s "Safely reclaimable" total is always 0.
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
