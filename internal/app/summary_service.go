// [파일 역할] SummaryService.Summarize가 이미 저장되어 있는(즉 `libra scan`이
// 끝난 뒤의) 프로젝트/리소스를 project_repository.go의 ProjectRepository와
// resource_repository.go의 ResourceRepository로 읽어와 개수, 리소스 타입별
// 용량, 위험도(RiskSafe/RiskReview/RiskBlocked)별 회수 가능 용량을 집계하는
// 파일이다. 이 서비스 자체는 스캔이나 탐지를 전혀 하지 않고 오직 이미 있는
// 데이터를 집계만 하며, cmd/summary.go(`libra summary`)가 이 서비스를 호출해
// 결과를 output.SummaryView로 렌더링한다.
package app

import (
	"context"
	"fmt"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

// SummaryResourceLine is the total logical size observed for one resource
// type.
type SummaryResourceLine struct {
	Type  domain.ResourceType
	Bytes int64
}

// Summary is the aggregated result of SummaryService.Summarize: project and
// resource counts, storage by resource type, and reclaimable space by risk
// level. See output.SummaryView for how this is rendered.
type Summary struct {
	ProjectCount    int
	ResourceCount   int
	ResourcesByType []SummaryResourceLine
	SafeReclaimable int64
	NeedsReview     int64
	Blocked         int64
}

// SummaryService aggregates already-persisted projects and resources. It
// does not scan or detect anything itself; run `libra scan` first.
type SummaryService struct {
	projects  ProjectRepository
	resources ResourceRepository
}

func NewSummaryService(projects ProjectRepository, resources ResourceRepository) *SummaryService {
	return &SummaryService{projects: projects, resources: resources}
}

// Summarize reads every observed project and resource and aggregates them.
// filter, if non-nil, excludes resources it returns false for (used for
// --type/--drive).
func (s *SummaryService) Summarize(ctx context.Context, filter func(domain.Resource) bool) (Summary, error) {
	projects, err := s.projects.List(ctx)
	if err != nil {
		return Summary{}, fmt.Errorf("list projects: %w", err)
	}
	resources, err := s.resources.List(ctx)
	if err != nil {
		return Summary{}, fmt.Errorf("list resources: %w", err)
	}

	summary := Summary{ProjectCount: len(projects)}
	byType := make(map[domain.ResourceType]int64)
	var order []domain.ResourceType
	for _, resource := range resources {
		if filter != nil && !filter(resource) {
			continue
		}
		summary.ResourceCount++
		if _, seen := byType[resource.Type]; !seen {
			order = append(order, resource.Type)
		}
		byType[resource.Type] += resource.LogicalSize

		switch resource.Risk {
		case domain.RiskSafe:
			summary.SafeReclaimable += resource.ReclaimableSize
		case domain.RiskReview:
			summary.NeedsReview += resource.LogicalSize
		case domain.RiskBlocked:
			summary.Blocked += resource.LogicalSize
		}
	}
	for _, resourceType := range order {
		summary.ResourcesByType = append(summary.ResourcesByType, SummaryResourceLine{Type: resourceType, Bytes: byType[resourceType]})
	}
	return summary, nil
}
