package domain

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
