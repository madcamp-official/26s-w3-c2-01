package app

import (
	"context"
	"fmt"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

// ProjectListing pairs a project with the resource-dependency count
// cmd/projects.go renders, joined here rather than in cmd since the join
// (DependencyRepository.FindResourcesByProject) is repository-level logic,
// not CLI rendering.
type ProjectListing struct {
	Project       domain.BuildProject
	ResourceCount int
}

// ProjectListService lists persisted projects for `libra projects`,
// applying a caller-supplied filter and joining each survivor's dependency
// count. Exists so cmd/projects.go follows the same cmd -> application
// service -> repository layering cmd/scan.go and cmd/summary.go already
// use, closing finding #5 in docs/libra_review_findings_day4.md.
type ProjectListService struct {
	projects     ProjectRepository
	dependencies DependencyRepository
}

func NewProjectListService(projects ProjectRepository, dependencies DependencyRepository) *ProjectListService {
	return &ProjectListService{projects: projects, dependencies: dependencies}
}

// List returns every project filter accepts (all of them if filter is
// nil), each paired with how many resources it depends on. One
// FindResourcesByProject call per surviving project, not one query total,
// so a project filtered out never pays for a count nobody asked to see --
// the same N+1-by-design tradeoff cmd/projects.go made before this move.
func (s *ProjectListService) List(ctx context.Context, filter func(domain.BuildProject) bool) ([]ProjectListing, error) {
	projects, err := s.projects.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}

	listings := make([]ProjectListing, 0, len(projects))
	for _, project := range projects {
		if filter != nil && !filter(project) {
			continue
		}
		resources, err := s.dependencies.FindResourcesByProject(ctx, project.ID)
		if err != nil {
			return nil, fmt.Errorf("count resources for project %q: %w", project.ID, err)
		}
		listings = append(listings, ProjectListing{Project: project, ResourceCount: len(resources)})
	}
	return listings, nil
}
