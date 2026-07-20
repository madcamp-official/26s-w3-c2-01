package app

import (
	"context"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

// DependencyRepository is the persistence contract for the PROJECT ->
// RESOURCE dependency graph (implemented by
// internal/store/sqlite.DependencyRepository). app.ImpactService and
// app.ExplainService both depend only on this interface, not the sqlite
// package, so they can be unit-tested against an in-memory stub.
type DependencyRepository interface {
	UpsertGraph(context.Context, string, domain.Dependency, []domain.Evidence) error
	FindResourcesByProject(context.Context, string) ([]domain.Dependency, error)
	FindProjectsByResource(context.Context, string) ([]domain.Dependency, error)
	FindEvidence(context.Context, string) ([]domain.Evidence, error)
}
