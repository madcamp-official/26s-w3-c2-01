package app

import (
	"context"
	"fmt"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

// DependencyObservation pairs a Dependency edge with the Evidence backing
// it, as produced by an adapter's dependency-resolution logic (e.g.
// msbuild.ResolveDependencies). It mirrors msbuild.ResolvedDependency's
// shape rather than importing it, keeping internal/app decoupled from any
// specific adapter package.
type DependencyObservation struct {
	Dependency domain.Dependency
	Evidence   []domain.Evidence
}

// DependencyService persists dependency graph edges an adapter has already
// resolved. It does not itself interpret project files or match versions --
// that is adapter-specific domain knowledge -- it only orchestrates storage.
type DependencyService struct {
	repository DependencyRepository
}

func NewDependencyService(repository DependencyRepository) *DependencyService {
	return &DependencyService{repository: repository}
}

// Persist upserts every observation into the dependency graph for scanID.
// It stops at the first failure; observations before it are already
// persisted (each UpsertGraph call is independently atomic per the
// repository contract, but the batch as a whole is not).
func (s *DependencyService) Persist(ctx context.Context, scanID string, observations []DependencyObservation) error {
	for _, observation := range observations {
		if err := s.repository.UpsertGraph(ctx, scanID, observation.Dependency, observation.Evidence); err != nil {
			return fmt.Errorf("persist dependency %q: %w", observation.Dependency.ID, err)
		}
	}
	return nil
}
