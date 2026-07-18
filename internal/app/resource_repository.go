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
