package app

import (
	"context"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

type DependencyRepository interface {
	UpsertGraph(context.Context, string, domain.Dependency, []domain.Evidence) error
	FindResourcesByProject(context.Context, string) ([]domain.Dependency, error)
	FindProjectsByResource(context.Context, string) ([]domain.Dependency, error)
	FindEvidence(context.Context, string) ([]domain.Evidence, error)
}
