package app

import (
	"context"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

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
