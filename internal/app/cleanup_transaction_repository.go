package app

import (
	"context"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

type CleanupTransactionRepository interface {
	Create(context.Context, domain.CleanupTransaction) error
	Update(context.Context, domain.CleanupTransaction) error
	FindByID(context.Context, string) (domain.CleanupTransaction, error)
	List(context.Context) ([]domain.CleanupTransaction, error)
}
