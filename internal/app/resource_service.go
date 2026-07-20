package app

import (
	"context"
	"fmt"
	"time"

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
	Reasons  []string
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

	measured, err := scanner.MeasureResource(ctx, s.filesystem, displayPath)
	if err != nil {
		return ResourceObservation{}, fmt.Errorf("measure resource %q: %w", displayPath, err)
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
	assessment := s.riskPolicy.Classify(ResourceContext{
		Resource:      detected,
		ProtectedPath: classification.SystemManaged,
		Cleanup:       input.Cleanup,
	})
	detected.Risk = assessment.Level
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
		Reasons:  assessment.Reasons,
	}, nil
}
