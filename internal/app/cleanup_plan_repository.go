package app

import (
	"context"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

// CleanupPlanRepository persists an immutable planning snapshot and all of
// its resource-sized execution items in one database transaction.
type CleanupPlanRepository interface {
	Create(context.Context, domain.CleanupPlan) error
	FindByID(context.Context, string) (domain.CleanupPlan, error)
}
