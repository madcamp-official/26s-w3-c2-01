package app

import (
	"context"
	"fmt"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

// ExplainedUsage is one project's evidence for depending on a resource, or
// one resource a project depends on, depending on which direction
// ExplainService was asked to explain. See F-07/3.6 in
// docs/libra_cli_commands_and_schedule.md.
type ExplainedUsage struct {
	ProjectID    string
	ProjectName  string
	ProjectPath  string
	ResourceID   string
	ResourceName string
	Relation     domain.RelationType
	Evidence     []domain.Evidence
}

// ResourceExplanation is everything `libra explain <resource>` needs to
// render, per F-07's required fields.
type ResourceExplanation struct {
	Resource domain.Resource
	UsedBy   []ExplainedUsage
	Impact   []domain.ImpactAssessment
}

// ProjectExplanation is everything `libra explain project:<path>` needs to
// render.
type ProjectExplanation struct {
	Project  domain.BuildProject
	Requires []ExplainedUsage
}

// ExplainService assembles the full explanation of one resource or project
// from the repositories and judgment services other layers already own
// (DependencyRepository for evidence, ImpactService for expected impact).
// It does not compute new Risk, Confidence, or Impact values itself.
type ExplainService struct {
	resources    ResourceRepository
	projects     ProjectRepository
	dependencies DependencyRepository
	impact       *ImpactService
	now          func() time.Time
}

// NewExplainService constructs the service from repositories only -- it
// builds its own internal ImpactService rather than taking one as a
// parameter. ImpactService is stateless (holds only the repositories), so
// this and cmd/impact.go's own separate app.NewImpactService(...) call end
// up doing the same judgment against the same data; there's no shared state
// to keep in sync.
func NewExplainService(resources ResourceRepository, projects ProjectRepository, dependencies DependencyRepository) *ExplainService {
	return &ExplainService{
		resources: resources, projects: projects, dependencies: dependencies,
		impact: NewImpactService(dependencies, resources),
		now:    time.Now,
	}
}

// ExplainResource explains a single resource: which projects use it, the
// evidence behind each dependency, and the expected impact of removing it.
func (s *ExplainService) ExplainResource(ctx context.Context, resourceID string) (ResourceExplanation, error) {
	resource, err := s.resources.FindByID(ctx, resourceID)
	if err != nil {
		return ResourceExplanation{}, fmt.Errorf("find resource %q: %w", resourceID, err)
	}
	// Recompute from the persisted observation time, same as `libra
	// resources`/`libra plan` (resource_list_service.go, plan_service.go) --
	// without this, explain showed a SAFE resource's confidence/risk exactly
	// as they were at scan time, even after they'd gone stale enough that
	// every other read path had already downgraded it to REVIEW with an
	// EVIDENCE_STALE reason.
	resource = ApplyFreshness(resource, s.now().UTC())

	edges, err := s.dependencies.FindProjectsByResource(ctx, resourceID)
	if err != nil {
		return ResourceExplanation{}, fmt.Errorf("find projects depending on %q: %w", resourceID, err)
	}

	usages := make([]ExplainedUsage, 0, len(edges))
	for _, edge := range edges {
		project, err := s.projects.FindByID(ctx, edge.SourceID)
		if err != nil {
			return ResourceExplanation{}, fmt.Errorf("find project %q: %w", edge.SourceID, err)
		}
		evidence, err := s.dependencies.FindEvidence(ctx, edge.ID)
		if err != nil {
			return ResourceExplanation{}, fmt.Errorf("find evidence for dependency %q: %w", edge.ID, err)
		}
		usages = append(usages, ExplainedUsage{
			ProjectID: project.ID, ProjectName: project.Name, ProjectPath: project.RootPath,
			Relation: edge.Relation, Evidence: evidence,
		})
	}

	impact, err := s.impact.Assess(ctx, resourceID)
	if err != nil {
		return ResourceExplanation{}, fmt.Errorf("assess impact of %q: %w", resourceID, err)
	}

	return ResourceExplanation{Resource: resource, UsedBy: usages, Impact: impact}, nil
}

// ExplainProject explains a single project: which resources it depends on
// and the evidence behind each dependency.
func (s *ExplainService) ExplainProject(ctx context.Context, projectID string) (ProjectExplanation, error) {
	project, err := s.projects.FindByID(ctx, projectID)
	if err != nil {
		return ProjectExplanation{}, fmt.Errorf("find project %q: %w", projectID, err)
	}

	edges, err := s.dependencies.FindResourcesByProject(ctx, projectID)
	if err != nil {
		return ProjectExplanation{}, fmt.Errorf("find resources required by %q: %w", projectID, err)
	}

	usages := make([]ExplainedUsage, 0, len(edges))
	for _, edge := range edges {
		resource, err := s.resources.FindByID(ctx, edge.TargetID)
		if err != nil {
			return ProjectExplanation{}, fmt.Errorf("find resource %q: %w", edge.TargetID, err)
		}
		evidence, err := s.dependencies.FindEvidence(ctx, edge.ID)
		if err != nil {
			return ProjectExplanation{}, fmt.Errorf("find evidence for dependency %q: %w", edge.ID, err)
		}
		usages = append(usages, ExplainedUsage{
			ResourceID: resource.ID, ResourceName: resource.Name,
			Relation: edge.Relation, Evidence: evidence,
		})
	}

	return ProjectExplanation{Project: project, Requires: usages}, nil
}
