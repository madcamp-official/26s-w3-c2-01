// [파일 역할] ResourceService.Observe가 어댑터가 보고한 domain.Resource
// 원시 사실(fact) 하나를 받아 (1) 경로 절대화/정규화 + domain.ResourceID
// 부여, (2) scanner.MeasureResource로 실제 디스크 크기 측정, (3)
// ResourcePathClassifier로 시스템 보호 경로 여부 판별, (4) risk_policy.go의
// RiskPolicy 적용, (5) resource_repository.go의 ResourceRepository로 최종
// 저장까지 한 번에 처리하는 파일이다. analysis_orchestrator.go의
// AnalysisOrchestrator가 ResourceObserver 인터페이스(analysis_orchestrator.go
// 자체 선언)를 통해 탐지된 리소스(시스템 리소스든 Node 프로젝트 소유
// 리소스든)마다 한 번씩 이 Observe를 호출한다.
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
