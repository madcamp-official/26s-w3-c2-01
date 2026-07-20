package domain

// [파일 역할] ImpactAssessment는 dependency.go의 Dependency 그래프를 바탕으로
// "리소스를 제거하면 어떤 프로젝트의 어떤 활동(RUN/BUILD/DEBUG/CI)이 얼마나
// 영향받는지"를 판정한 결과 모델이다. ImpactLevel 주석이 명시하듯 이것은
// 스캔이 관찰한 사실이 아니라 판단(judgment)이므로, 어댑터가 아니라
// 애플리케이션 레벨 규칙(internal/app/impact_service.go의
// ImpactService.Assess)만 이 값을 채운다. 현재 ImpactService.Assess는 BUILD
// 스코프만 구현하고 있어 RUN/DEBUG/CI는 이 모델은 있지만 아직 채워지지 않는다.

// ImpactScope classifies which kind of activity is affected if a resource a
// project depends on is removed.
type ImpactScope string

const (
	ImpactScopeRun   ImpactScope = "RUN"
	ImpactScopeBuild ImpactScope = "BUILD"
	ImpactScopeDebug ImpactScope = "DEBUG"
	ImpactScopeCI    ImpactScope = "CI"
)

// ImpactLevel is how severely a scope is affected. It is a judgment, not a
// fact -- adapters never set this; only application-level impact rules do.
type ImpactLevel string

const (
	ImpactLevelNone    ImpactLevel = "NONE"
	ImpactLevelLow     ImpactLevel = "LOW"
	ImpactLevelHigh    ImpactLevel = "HIGH"
	ImpactLevelUnknown ImpactLevel = "UNKNOWN"
)

// ImpactAssessment is one scope's judged impact on a single project if a
// resource it depends on is removed.
type ImpactAssessment struct {
	ProjectID string
	Scope     ImpactScope
	Level     ImpactLevel
	Reason    string
}
