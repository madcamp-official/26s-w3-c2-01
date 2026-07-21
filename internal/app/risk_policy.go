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
	Level      domain.RiskLevel
	Confidence domain.ConfidenceProfile
	Blockers   []domain.RiskReason
	Warnings   []domain.RiskReason
	Safeguards []domain.RiskReason
	Unknowns   []domain.RiskReason
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
			Level: domain.RiskBlocked, Confidence: context.Confidence,
			Blockers: []domain.RiskReason{{Code: "ANDROID_SDK_MANAGED", Severity: domain.RiskReasonBlocker, Message: "use `sdkmanager --list` and `sdkmanager --uninstall <package>` or Android Studio SDK Manager"}},
		}
	}
	if context.Resource.Type == domain.ResourceTypeGlobalCache {
		return RiskAssessment{Level: domain.RiskReview, Confidence: context.Confidence,
			Warnings: []domain.RiskReason{{Code: "OFFICIAL_CLEANUP_GUIDANCE", Severity: domain.RiskReasonWarning, Message: officialCacheCleanupGuidance(context.Resource.Version)}},
		}
	}
	if context.Resource.Type == domain.ResourceTypeDockerVolume {
		return RiskAssessment{
			Level: domain.RiskBlocked, Confidence: context.Confidence,
			Blockers: []domain.RiskReason{{Code: "DOCKER_VOLUME_USER_DATA", Severity: domain.RiskReasonBlocker, Message: "Docker volumes may contain persistent user data and are never automatic cleanup targets"}},
		}
	}
	if context.Resource.Type == domain.ResourceTypeDockerCache {
		return RiskAssessment{
			Level: domain.RiskReview, Confidence: context.Confidence,
			Warnings: []domain.RiskReason{{Code: "DOCKER_OFFICIAL_CLEANUP_REQUIRED", Severity: domain.RiskReasonWarning, Message: "Docker-managed data must be reviewed and cleaned with Docker's official commands"}},
		}
	}
	if context.ProtectedPath || context.Resource.SystemManaged {
		return RiskAssessment{
			Level: domain.RiskBlocked, Confidence: context.Confidence,
			Blockers: []domain.RiskReason{{Code: "SYSTEM_MANAGED", Severity: domain.RiskReasonBlocker, Message: "resource is inside an operating-system managed path"}},
		}
	}
	if context.RequiredByProject {
		return RiskAssessment{
			Level: domain.RiskBlocked, Confidence: context.Confidence,
			Blockers: []domain.RiskReason{{Code: "REQUIRED_BY_PROJECT", Severity: domain.RiskReasonBlocker, Message: "a scanned project currently depends on this resource"}},
		}
	}
	if len(context.CriticalUnknowns) > 0 {
		return RiskAssessment{Level: domain.RiskReview, Confidence: context.Confidence, Unknowns: append([]domain.RiskReason(nil), context.CriticalUnknowns...)}
	}
	if context.Resource.Regenerable && context.Cleanup.complete() {
		return RiskAssessment{
			Level: domain.RiskSafe, Confidence: context.Confidence,
			Safeguards: []domain.RiskReason{{Code: "CLEANUP_EVIDENCE_COMPLETE", Severity: domain.RiskReasonSafeguard, Message: "project artifact is regenerable and all cleanup evidence is verified"}},
		}
	}
	return RiskAssessment{
		Level: domain.RiskReview, Confidence: context.Confidence,
		Warnings: []domain.RiskReason{{Code: "SAFE_CONDITIONS_NOT_PROVEN", Severity: domain.RiskReasonWarning, Message: "cleanup safety has not been fully verified"}},
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
	default:
		return "use the owning package manager's official cleanup workflow"
	}
}
