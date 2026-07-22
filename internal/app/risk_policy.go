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
	Confidence        domain.ConfidenceProfile
	CriticalUnknowns  []domain.RiskReason
	DependencyImpact  DependencyImpact
}

type DependencyImpact struct {
	RequiredByProjects int
	ActiveProjects     int
	RelationStrength   domain.ConfidenceAssessment
	FailureModes       []string
	Recoverable        bool
	RecoveryAction     string
}

// CleanupEvidence records the independent safety facts required before a
// project artifact may be classified SAFE. Zero values mean unverified, not
// false evidence.
type CleanupEvidence struct {
	ProjectOwned              bool
	KnownOutputPath           bool
	ReparsePointFree          bool
	GitTrackedOriginalsAbsent bool
	Verification              CleanupVerification
}

type CleanupVerification struct {
	ProjectOwned              domain.VerifiedFact
	KnownOutputPath           domain.VerifiedFact
	ReparsePointFree          domain.VerifiedFact
	GitTrackedOriginalsAbsent domain.VerifiedFact
}

func (e CleanupEvidence) complete() bool {
	if e.Verification != (CleanupVerification{}) {
		return e.Verification.ProjectOwned.Status == domain.VerifiedTrue &&
			e.Verification.KnownOutputPath.Status == domain.VerifiedTrue &&
			e.Verification.ReparsePointFree.Status == domain.VerifiedTrue &&
			e.Verification.GitTrackedOriginalsAbsent.Status == domain.VerifiedTrue
	}
	return e.ProjectOwned && e.KnownOutputPath && e.ReparsePointFree && e.GitTrackedOriginalsAbsent
}

type RiskAssessment struct {
	Level          domain.RiskLevel
	Disposition    domain.CleanupDisposition
	Impact         int
	Likelihood     int
	Recoverability int
	Uncertainty    int
	Confidence     domain.ConfidenceProfile
	Blockers       []domain.RiskReason
	Warnings       []domain.RiskReason
	Safeguards     []domain.RiskReason
	Unknowns       []domain.RiskReason
}

func (a RiskAssessment) Reasons() []domain.RiskReason {
	reasons := make([]domain.RiskReason, 0, len(a.Blockers)+len(a.Warnings)+len(a.Safeguards)+len(a.Unknowns))
	reasons = append(reasons, a.Blockers...)
	reasons = append(reasons, a.Warnings...)
	reasons = append(reasons, a.Safeguards...)
	reasons = append(reasons, a.Unknowns...)
	return reasons
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
	if context.Resource.Type == domain.ResourceTypeAndroidSDK {
		return RiskAssessment{
			Level: domain.RiskBlocked, Disposition: domain.DispositionUseOfficialTool, Impact: 90, Likelihood: 80, Recoverability: 40, Confidence: context.Confidence,
			Blockers: []domain.RiskReason{{Code: "ANDROID_SDK_MANAGED", Severity: domain.RiskReasonBlocker, Message: "use `sdkmanager --list` and `sdkmanager --uninstall <package>` or Android Studio SDK Manager"}},
		}
	}
	if context.Resource.Type == domain.ResourceTypeGlobalCache {
		return RiskAssessment{Level: domain.RiskReview, Disposition: domain.DispositionUseOfficialTool, Impact: 50, Likelihood: 40, Recoverability: 90, Confidence: context.Confidence,
			Warnings: []domain.RiskReason{{Code: "OFFICIAL_CLEANUP_GUIDANCE", Severity: domain.RiskReasonWarning, Message: officialCacheCleanupGuidance(context.Resource.Version)}},
		}
	}
	if context.Resource.Type == domain.ResourceTypeDockerVolume {
		return RiskAssessment{
			Level: domain.RiskBlocked, Disposition: domain.DispositionNeverDelete, Impact: 100, Likelihood: 70, Recoverability: 10, Confidence: context.Confidence,
			Blockers: []domain.RiskReason{{Code: "DOCKER_VOLUME_USER_DATA", Severity: domain.RiskReasonBlocker, Message: "Docker volumes may contain persistent user data and are never automatic cleanup targets"}},
		}
	}
	if context.Resource.Type == domain.ResourceTypeDockerCache {
		return RiskAssessment{
			Level: domain.RiskReview, Disposition: domain.DispositionUseOfficialTool, Impact: 60, Likelihood: 50, Recoverability: 80, Confidence: context.Confidence,
			Warnings: []domain.RiskReason{{Code: "DOCKER_OFFICIAL_CLEANUP_REQUIRED", Severity: domain.RiskReasonWarning, Message: "Docker-managed data must be reviewed and cleaned with Docker's official commands"}},
		}
	}
	if context.ProtectedPath || context.Resource.SystemManaged {
		return RiskAssessment{
			Level: domain.RiskBlocked, Disposition: domain.DispositionNeverDelete, Impact: 100, Likelihood: 80, Recoverability: 20, Confidence: context.Confidence,
			Blockers: []domain.RiskReason{{Code: "SYSTEM_MANAGED", Severity: domain.RiskReasonBlocker, Message: "resource is inside an operating-system managed path"}},
		}
	}
	if context.RequiredByProject && !(context.Resource.Regenerable && isProjectLocalArtifact(context.Resource.Type)) {
		return RiskAssessment{
			Level: domain.RiskBlocked, Disposition: domain.DispositionNeverDelete, Impact: 90, Likelihood: 80, Recoverability: 30, Confidence: context.Confidence,
			Blockers: []domain.RiskReason{{Code: "REQUIRED_BY_PROJECT", Severity: domain.RiskReasonBlocker, Message: "a scanned project currently depends on this resource"}},
		}
	}
	if len(context.CriticalUnknowns) > 0 {
		return RiskAssessment{Level: domain.RiskReview, Disposition: domain.DispositionManualReview, Uncertainty: 100, Confidence: context.Confidence, Unknowns: append([]domain.RiskReason(nil), context.CriticalUnknowns...)}
	}
	if context.Resource.Regenerable && context.Cleanup.complete() {
		return RiskAssessment{
			Level: domain.RiskSafe, Disposition: domain.DispositionAutoQuarantine, Impact: 30, Likelihood: 10, Recoverability: 100, Confidence: context.Confidence,
			Safeguards: []domain.RiskReason{{Code: "CLEANUP_EVIDENCE_COMPLETE", Severity: domain.RiskReasonSafeguard, Message: "project artifact is regenerable and all cleanup evidence is verified"}},
		}
	}
	return RiskAssessment{
		Level: domain.RiskReview, Disposition: domain.DispositionManualReview, Impact: 50, Likelihood: 40, Recoverability: 50, Uncertainty: 60, Confidence: context.Confidence,
		Warnings: []domain.RiskReason{{Code: "SAFE_CONDITIONS_NOT_PROVEN", Severity: domain.RiskReasonWarning, Message: "cleanup safety has not been fully verified"}},
	}
}

func isProjectLocalArtifact(resourceType domain.ResourceType) bool {
	switch resourceType {
	case domain.ResourceTypeNodeModules, domain.ResourceTypeBuildOutput, domain.ResourceTypePods, domain.ResourceTypeVenv:
		return true
	default:
		return false
	}
}

func officialCacheCleanupGuidance(version string) string {
	switch version {
	case "npm":
		return "inspect with `npm cache verify`; reclaim only when necessary with `npm cache clean --force`"
	case "pnpm":
		return "remove unreferenced packages with `pnpm store prune`"
	case "maven":
		return "use `mvn dependency:purge-local-repository` for project dependency cleanup"
	case "cargo-registry", "cargo-git":
		return "Cargo has no built-in global cache purge; use `cargo clean` only for project target artifacts"
	case "gradle":
		return "Gradle manages cache cleanup automatically; configure retention in Gradle User Home init scripts"
	case "xcode-deriveddata":
		return "safe to delete entirely; Xcode regenerates it on the next build (`rm -rf ~/Library/Developer/Xcode/DerivedData`, or Xcode > Settings > Locations > the arrow next to Derived Data)"
	case "cocoapods-cache":
		return "use `pod cache clean --all` to clear CocoaPods' download cache"
	case "swiftpm-cache":
		return "use `rm -rf ~/Library/Caches/org.swift.swiftpm`, or `swift package purge-cache` on toolchains that support it"
	case "homebrew-cache":
		return "use `brew cleanup` to remove old downloads, or `brew cleanup -s` to also clear the cache directory"
	case "simulator-cache":
		return "safe to delete; Simulator regenerates it automatically. Manage installed runtimes and devices separately via Xcode > Settings > Platforms or `xcrun simctl`"
	case "simulator-devices":
		return "may hold app data or state you seeded (test fixtures, logins) -- review first. Reclaim stale devices with `xcrun simctl delete unavailable`, or a specific one with `xcrun simctl delete <udid>`"
	default:
		return "use the owning package manager's official cleanup workflow"
	}
}
