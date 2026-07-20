package app

import (
	"context"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

// ProjectRepository and WorkspaceRepository are the persistence contracts
// for BuildProject/Workspace (implemented by internal/store/sqlite). Kept
// as two separate interfaces, not one, because a BuildProject can exist
// with no Workspace at all (§3.1 of docs/libra_integration_contracts.md)
// and callers that only ever touch one side (e.g. cmd/projects.go never
// touches WorkspaceRepository) shouldn't have to depend on the other.
type ProjectRepository interface {
	UpsertObserved(context.Context, string, []domain.BuildProject) error
	FindByID(context.Context, string) (domain.BuildProject, error)
	FindByManifestPath(context.Context, domain.ProjectType, string) (domain.BuildProject, error)
	List(context.Context) ([]domain.BuildProject, error)
}

type WorkspaceRepository interface {
	Upsert(context.Context, string, domain.Workspace) error
	ReplaceMembers(context.Context, string, []string) error
}
