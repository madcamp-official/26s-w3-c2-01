package app

import (
	"context"
	"fmt"
	"time"

	gitadapter "github.com/madcamp-official/26s-w3-c2-01/internal/adapter/git"
	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/pathutil"
	"github.com/madcamp-official/26s-w3-c2-01/internal/safety"
	"github.com/madcamp-official/26s-w3-c2-01/internal/scanner"
)

// resource_service.go turns one adapter-reported domain.Resource fact into
// a fully persisted observation: measures its real on-disk size, classifies
// whether it's system-protected, applies RiskPolicy, and upserts it.
// analysis_orchestrator.go calls this once per detected resource (system or
// project-owned) through the ResourceObserver interface it declares.
type ResourcePathClassifier interface {
	Classify(string) (safety.PathClassification, error)
}

type ResourceObservation struct {
	Resource domain.Resource
	Issues   []scanner.Issue
	Reasons  []domain.RiskReason
}

// ResourceObservationInput keeps cleanup evidence separate from persisted
// resource facts. Detectors must explicitly provide every verified fact;
// omitted facts remain conservative REVIEW inputs.
type ResourceObservationInput struct {
	Resource domain.Resource
	Cleanup  CleanupEvidence
}

// ResourceService enriches one adapter-detected resource with filesystem and
// safety facts, applies central risk policy, and persists the observation.
type ResourceService struct {
	filesystem scanner.Scanner
	repository ResourceRepository
	classifier ResourcePathClassifier
	riskPolicy RiskPolicy
	now        func() time.Time
}

func NewResourceService(
	filesystem scanner.Scanner,
	repository ResourceRepository,
	classifier ResourcePathClassifier,
	riskPolicy RiskPolicy,
) *ResourceService {
	return &ResourceService{
		filesystem: filesystem,
		repository: repository,
		classifier: classifier,
		riskPolicy: riskPolicy,
		now:        time.Now,
	}
}

func (s *ResourceService) Observe(ctx context.Context, input ResourceObservationInput) (ResourceObservation, error) {
	detected := input.Resource
	displayPath, err := pathutil.Absolute(detected.DisplayPath)
	if err != nil {
		return ResourceObservation{}, fmt.Errorf("resolve resource display path: %w", err)
	}
	normalizedPath, err := pathutil.Normalize(displayPath)
	if err != nil {
		return ResourceObservation{}, fmt.Errorf("normalize resource path: %w", err)
	}
	detected.DisplayPath = displayPath
	detected.NormalizedPath = normalizedPath
	detected.ID = domain.ResourceID(detected.Type, detected.Version, normalizedPath)

	measured := scanner.ResourceSize{
		LogicalSize: detected.LogicalSize, SizeKnown: detected.SizeKnown,
		LastModifiedAt: detected.LastModifiedAt,
	}
	if !detected.SizeKnown {
		measured, err = scanner.MeasureResource(ctx, s.filesystem, displayPath)
		if err != nil {
			return ResourceObservation{}, fmt.Errorf("measure resource %q: %w", displayPath, err)
		}
	}
	detected.LogicalSize = measured.LogicalSize
	detected.SizeKnown = measured.SizeKnown
	detected.LastModifiedAt = measured.LastModifiedAt
	detected.LastObservedAt = s.now()

	classification, err := s.classifier.Classify(displayPath)
	if err != nil {
		return ResourceObservation{}, err
	}
	detected.SystemManaged = classification.SystemManaged
	cleanup := enrichCleanupEvidence(ctx, displayPath, input.Cleanup)
	detected.ConfidenceProfile = confidenceProfile(detected.Confidence, cleanup)
	assessment := s.riskPolicy.Classify(ResourceContext{
		Resource:      detected,
		ProtectedPath: classification.SystemManaged,
		Cleanup:       cleanup,
		Confidence:    detected.ConfidenceProfile,
	})
	detected.Risk = assessment.Level
	detected.RiskReasons = assessment.Reasons()
	detected.Confidence = detected.ConfidenceProfile.Overall()
	switch detected.Risk {
	case domain.RiskSafe:
		detected.ReclaimableSize = detected.LogicalSize
	case domain.RiskBlocked:
		detected.ReclaimableSize = 0
	}

	if err := s.repository.Upsert(ctx, detected); err != nil {
		return ResourceObservation{}, fmt.Errorf("persist resource observation: %w", err)
	}
	return ResourceObservation{
		Resource: detected,
		Issues:   measured.Issues,
		Reasons:  assessment.Reasons(),
	}, nil
}

func enrichCleanupEvidence(ctx context.Context, path string, evidence CleanupEvidence) CleanupEvidence {
	if !evidence.ProjectOwned || !evidence.KnownOutputPath {
		return evidence
	}
	if reparse, err := safety.IsReparsePoint(path); err == nil {
		evidence.ReparsePointFree = !reparse
	}
	repoRoot, found, err := gitadapter.FindRepoRoot(path)
	if err != nil {
		return evidence
	}
	if !found {
		evidence.GitTrackedOriginalsAbsent = true
		return evidence
	}
	if tracked, err := (gitadapter.TrackedFilesChecker{}).HasTrackedFiles(ctx, repoRoot, path); err == nil {
		evidence.GitTrackedOriginalsAbsent = !tracked
	}
	return evidence
}

// ReclassifyRequired re-classifies an already-observed-and-persisted
// resource as BLOCKED because scan discovered, only after Observe already
// ran, that a project depends on it -- see AnalysisOrchestrator.Run, which
// resolves dependencies (PhaseResolveDependencies) after every resource's
// first Observe pass (PhaseDiscoverSystemResources), so "does any project
// require this resource" genuinely isn't knowable any earlier.
//
// A resource whose first pass already produced BLOCKED (a protected system
// path) is left untouched and returned as-is: "required by project" only
// ever raises the bar to BLOCKED, it never overrides a stronger existing
// reason or lowers one.
func (s *ResourceService) ReclassifyRequired(ctx context.Context, resourceID string) (ResourceObservation, error) {
	resource, err := s.repository.FindByID(ctx, resourceID)
	if err != nil {
		return ResourceObservation{}, fmt.Errorf("find resource %q: %w", resourceID, err)
	}
	if resource.Risk == domain.RiskBlocked {
		return ResourceObservation{Resource: resource}, nil
	}

	assessment := s.riskPolicy.Classify(ResourceContext{Resource: resource, RequiredByProject: true})
	resource.Risk = assessment.Level
	resource.ReclaimableSize = 0

	if err := s.repository.Upsert(ctx, resource); err != nil {
		return ResourceObservation{}, fmt.Errorf("persist reclassified resource: %w", err)
	}
	resource.RiskReasons = assessment.Reasons()
	return ResourceObservation{Resource: resource, Reasons: assessment.Reasons()}, nil
}

func confidenceProfile(classification int, cleanup CleanupEvidence) domain.ConfidenceProfile {
	// Dependency and scan coverage are conservative compatibility baselines
	// until the orchestrator maps its per-run UnverifiedScope collection into
	// resource-specific scores. They meet, but never exceed, the auto-plan
	// gate; any explicit lower score blocks automatic selection.
	profile := domain.ConfidenceProfile{
		Classification: classification,
		Dependency:     minimumAutoDependencyConfidence,
		ScanCoverage:   minimumAutoScanCoverage,
		Freshness:      100,
	}
	if cleanup.ProjectOwned {
		profile.Ownership = 100
	}
	if cleanup.complete() {
		profile.CleanupSafety = 100
	}
	return profile
}
