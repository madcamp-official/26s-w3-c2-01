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

type ResourcePathClassifier interface {
	Classify(string) (safety.PathClassification, error)
}

type ResourceObservation struct {
	Resource domain.Resource
	Issues   []scanner.Issue
	Reasons  []string
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

func (s *ResourceService) Observe(ctx context.Context, detected domain.Resource) (ResourceObservation, error) {
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
	})
	detected.Risk = assessment.Level
	if detected.Risk == domain.RiskBlocked {
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
