// [파일 역할] Resource 저장/조회 계약인 ResourceRepository 인터페이스 하나만
// 선언하는 파일이다. 구현체는 internal/store/sqlite. resource_service.go의
// ResourceService.Observe가 관찰·위험도 분류를 마친 리소스를 저장할 때, 그리고
// summary_service.go의 SummaryService.Summarize가 이미 저장된 리소스를
// 집계용으로 읽어올 때 각각 이 인터페이스만 참조한다.
package app

import (
	"context"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

// ResourceRepository is the persistence contract consumed by resource-facing
// application services and CLI commands.
type ResourceRepository interface {
	Upsert(context.Context, domain.Resource) error
	FindByID(context.Context, string) (domain.Resource, error)
	ListByType(context.Context, domain.ResourceType) ([]domain.Resource, error)
	List(context.Context) ([]domain.Resource, error)
}
