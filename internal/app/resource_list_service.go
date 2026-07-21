package app

import (
	"context"
	"fmt"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

// ResourceListing pairs a resource with the project-dependency count
// cmd/resources.go renders.
type ResourceListing struct {
	Resource     domain.Resource
	ProjectCount int
}

// ResourceListService lists persisted resources for `libra resources`; see
// ProjectListService's doc comment (project_list_service.go) for why this
// exists. Distinct from ResourceService (resource_service.go), which
// enriches and persists a single freshly-detected resource during `libra
// scan` -- this type only ever reads what a prior scan already persisted.
type ResourceListService struct {
	resources    ResourceRepository
	dependencies DependencyRepository
	now          func() time.Time
}

func NewResourceListService(resources ResourceRepository, dependencies DependencyRepository) *ResourceListService {
	return &ResourceListService{resources: resources, dependencies: dependencies, now: time.Now}
}

// List returns every resource filter accepts (all of them if filter is
// nil), each paired with how many projects are connected to it -- via
// either RelationRequires (build/run dependency) or RelationOwns
// (cleanup ownership); see docs/libra_integration_contracts.md's Graph
// section for the distinction.
func (s *ResourceListService) List(ctx context.Context, filter func(domain.Resource) bool) ([]ResourceListing, error) {
	resources, err := s.resources.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list resources: %w", err)
	}

	listings := make([]ResourceListing, 0, len(resources))
	for _, resource := range resources {
		resource = ApplyFreshness(resource, s.now().UTC())
		if filter != nil && !filter(resource) {
			continue
		}
		projects, err := s.dependencies.FindProjectsByResource(ctx, resource.ID)
		if err != nil {
			return nil, fmt.Errorf("count projects for resource %q: %w", resource.ID, err)
		}
		listings = append(listings, ResourceListing{Resource: resource, ProjectCount: len(projects)})
	}
	return listings, nil
}
