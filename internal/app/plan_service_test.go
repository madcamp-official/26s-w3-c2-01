package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

var errNotImplemented = errors.New("not implemented")

func TestPlanServiceSelectsSafeCandidatesUntilTargetMet(t *testing.T) {
	resources := &planResourceRepositoryStub{resources: []domain.Resource{
		safeResource("low-confidence-small", 40, 100),
		safeResource("high-confidence-large", 90, 500),
		safeResource("high-confidence-small", 90, 50),
		reviewResource("review-item", 200),
		blockedResource("blocked-item", 1000),
	}}
	scans := &planScanRepositoryStub{record: newTestScanRecord("scan-latest")}

	result, err := NewPlanService(resources, &planProjectRepositoryStub{}, scans, planOwnershipStub(resources.resources)).
		Build(context.Background(), PlanOptions{TargetBytes: 500})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	// Selection order must be confidence desc, then reclaimable size desc:
	// high-confidence-large (500) alone already meets the 500-byte target,
	// so nothing else should be picked even though low-confidence-small
	// would also fit.
	if len(result.Plan.Items) != 1 || result.Plan.Items[0].ResourceID != "high-confidence-large" {
		t.Fatalf("Items = %#v, want exactly [high-confidence-large]", result.Plan.Items)
	}
	if result.Plan.SelectedBytes != 500 {
		t.Fatalf("SelectedBytes = %d, want 500", result.Plan.SelectedBytes)
	}
	if result.Plan.Status != domain.CleanupPlanReady {
		t.Fatalf("Status = %q, want READY", result.Plan.Status)
	}
	if len(result.Review) != 1 || result.Review[0].ID != "review-item" {
		t.Fatalf("Review = %#v, want [review-item]", result.Review)
	}
	if len(result.Blocked) != 1 || result.Blocked[0].ID != "blocked-item" {
		t.Fatalf("Blocked = %#v, want [blocked-item]", result.Blocked)
	}
	for _, item := range result.Plan.Items {
		if item.ScanID != "scan-latest" {
			t.Fatalf("item ScanID = %q, want scan-latest", item.ScanID)
		}
	}
}

func TestPlanServiceReportsInsufficientCandidates(t *testing.T) {
	resources := &planResourceRepositoryStub{resources: []domain.Resource{
		safeResource("only-safe-item", 90, 100),
	}}
	scans := &planScanRepositoryStub{record: newTestScanRecord("scan-latest")}

	result, err := NewPlanService(resources, &planProjectRepositoryStub{}, scans, planOwnershipStub(resources.resources)).
		Build(context.Background(), PlanOptions{TargetBytes: 1000})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if result.Plan.Status != domain.CleanupPlanInsufficientCandidates {
		t.Fatalf("Status = %q, want INSUFFICIENT_CANDIDATES", result.Plan.Status)
	}
	if result.Plan.SelectedBytes != 100 {
		t.Fatalf("SelectedBytes = %d, want 100", result.Plan.SelectedBytes)
	}
}

func TestPlanServiceUnlimitedTargetSelectsEverySafeCandidate(t *testing.T) {
	resources := &planResourceRepositoryStub{resources: []domain.Resource{
		safeResource("first", 90, 100),
		safeResource("second", 80, 200),
		blockedResource("blocked-item", 1000),
	}}
	scans := &planScanRepositoryStub{record: newTestScanRecord("scan-latest")}

	result, err := NewPlanService(resources, &planProjectRepositoryStub{}, scans, planOwnershipStub(resources.resources)).
		Build(context.Background(), PlanOptions{})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(result.Plan.Items) != 2 {
		t.Fatalf("Items = %#v, want 2 items (blocked excluded)", result.Plan.Items)
	}
	if result.Plan.Status != domain.CleanupPlanReady {
		t.Fatalf("Status = %q, want READY", result.Plan.Status)
	}
}

func TestPlanServiceFiltersByProjectScope(t *testing.T) {
	inScope := safeResource("in-scope", 90, 100)
	inScope.NormalizedPath = `/projects/gameclient/node_modules`
	outOfScope := safeResource("out-of-scope", 90, 100)
	outOfScope.NormalizedPath = `/projects/otherapp/node_modules`

	resources := &planResourceRepositoryStub{resources: []domain.Resource{inScope, outOfScope}}
	scans := &planScanRepositoryStub{record: newTestScanRecord("scan-latest")}

	result, err := NewPlanService(resources, &planProjectRepositoryStub{}, scans, planOwnershipStub(resources.resources)).
		Build(context.Background(), PlanOptions{ProjectRootNormalized: `/projects/gameclient`})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(result.Plan.Items) != 1 || result.Plan.Items[0].ResourceID != "in-scope" {
		t.Fatalf("Items = %#v, want exactly [in-scope]", result.Plan.Items)
	}
}

func TestPlanServiceFillsOwnerProjectIDFromClosestRoot(t *testing.T) {
	resource := safeResource("nested-modules", 90, 100)
	resource.NormalizedPath = `/projects/gameclient/packages/api/node_modules`

	projects := &planProjectRepositoryStub{projects: []domain.BuildProject{
		{ID: "workspace-root", NormalizedRootPath: `/projects/gameclient`},
		{ID: "package-api", NormalizedRootPath: `/projects/gameclient/packages/api`},
	}}
	resources := &planResourceRepositoryStub{resources: []domain.Resource{resource}}
	scans := &planScanRepositoryStub{record: newTestScanRecord("scan-latest")}

	result, err := NewPlanService(resources, projects, scans, &planDependencyRepositoryStub{owners: map[string]string{"nested-modules": "package-api"}}).Build(context.Background(), PlanOptions{})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(result.Plan.Items) != 1 {
		t.Fatalf("Items = %#v, want 1 item", result.Plan.Items)
	}
	if got := result.Plan.Items[0].OwnerProjectID; got != "package-api" {
		t.Fatalf("OwnerProjectID = %q, want package-api (the more specific root)", got)
	}
}

func TestPlanServiceRequiresAScan(t *testing.T) {
	resources := &planResourceRepositoryStub{}
	scans := &planScanRepositoryStub{err: ErrNoScans}

	_, err := NewPlanService(resources, &planProjectRepositoryStub{}, scans, planOwnershipStub(resources.resources)).
		Build(context.Background(), PlanOptions{})
	if err == nil {
		t.Fatal("Build() error = nil, want error when no scan has been recorded")
	}
}

func TestPlanServiceDoesNotAutoSelectSafeResourceBelowCoverageGate(t *testing.T) {
	resource := safeResource("low-coverage", 90, 100)
	resource.ConfidenceProfile.ScanCoverage = minimumAutoScanCoverage - 1
	resources := &planResourceRepositoryStub{resources: []domain.Resource{resource}}

	result, err := NewPlanService(resources, &planProjectRepositoryStub{}, &planScanRepositoryStub{record: newTestScanRecord("scan-latest")}, planOwnershipStub(resources.resources)).
		Build(context.Background(), PlanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Plan.Items) != 0 || len(result.Review) != 1 || result.Review[0].ID != resource.ID {
		t.Fatalf("result = %#v, want low-coverage SAFE resource routed to review", result)
	}
}

func newTestScanRecord(id string) ScanRecord {
	return ScanRecord{ID: id, StartedAt: time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC), Roots: []string{"/projects"}, Status: ScanStatusCompleted}
}

func safeResource(id string, confidence int, reclaimable int64) domain.Resource {
	return domain.Resource{
		ID: id, Name: id, Type: domain.ResourceTypeNodeModules,
		NormalizedPath: "/projects/" + id, LogicalSize: reclaimable, SizeKnown: true,
		ReclaimableSize: reclaimable, Regenerable: true,
		LastObservedAt: time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC),
		Risk:           domain.RiskSafe, Confidence: confidence,
		ConfidenceProfile: domain.ConfidenceProfile{
			Classification: confidence, Ownership: 100, Dependency: 80,
			CleanupSafety: 100, ScanCoverage: 80,
		},
	}
}

func reviewResource(id string, reclaimable int64) domain.Resource {
	r := safeResource(id, 40, reclaimable)
	r.Risk = domain.RiskReview
	return r
}

func blockedResource(id string, reclaimable int64) domain.Resource {
	r := safeResource(id, 75, reclaimable)
	r.Risk = domain.RiskBlocked
	return r
}

type planResourceRepositoryStub struct {
	resources []domain.Resource
}

func (s *planResourceRepositoryStub) Upsert(context.Context, domain.Resource) error {
	return nil
}

func (s *planResourceRepositoryStub) FindByID(_ context.Context, id string) (domain.Resource, error) {
	for _, r := range s.resources {
		if r.ID == id {
			return r, nil
		}
	}
	return domain.Resource{}, errNotImplemented
}

func (s *planResourceRepositoryStub) ListByType(context.Context, domain.ResourceType) ([]domain.Resource, error) {
	return nil, nil
}

func (s *planResourceRepositoryStub) List(context.Context) ([]domain.Resource, error) {
	return s.resources, nil
}

type planProjectRepositoryStub struct {
	projects []domain.BuildProject
}

func (s *planProjectRepositoryStub) UpsertObserved(context.Context, string, []domain.BuildProject) error {
	return nil
}

func (s *planProjectRepositoryStub) FindByID(context.Context, string) (domain.BuildProject, error) {
	return domain.BuildProject{}, errNotImplemented
}

func (s *planProjectRepositoryStub) FindByManifestPath(context.Context, domain.ProjectType, string) (domain.BuildProject, error) {
	return domain.BuildProject{}, errNotImplemented
}

func (s *planProjectRepositoryStub) List(context.Context) ([]domain.BuildProject, error) {
	return s.projects, nil
}

type planScanRepositoryStub struct {
	record ScanRecord
	err    error
}

type planDependencyRepositoryStub struct{ owners map[string]string }

func planOwnershipStub(resources []domain.Resource) *planDependencyRepositoryStub {
	owners := make(map[string]string)
	for _, resource := range resources {
		if resource.Risk == domain.RiskSafe {
			owners[resource.ID] = "owner"
		}
	}
	return &planDependencyRepositoryStub{owners: owners}
}
func (s *planDependencyRepositoryStub) UpsertGraph(context.Context, string, domain.Dependency, []domain.Evidence) error {
	return nil
}
func (s *planDependencyRepositoryStub) FindResourcesByProject(context.Context, string) ([]domain.Dependency, error) {
	return nil, nil
}
func (s *planDependencyRepositoryStub) FindProjectsByResource(_ context.Context, resourceID string) ([]domain.Dependency, error) {
	owner := s.owners[resourceID]
	if owner == "" {
		return nil, nil
	}
	return []domain.Dependency{{SourceType: domain.NodeProject, SourceID: owner, TargetType: domain.NodeResource, TargetID: resourceID, Relation: domain.RelationOwns}}, nil
}
func (s *planDependencyRepositoryStub) FindEvidence(context.Context, string) ([]domain.Evidence, error) {
	return nil, nil
}

func (s *planScanRepositoryStub) Save(context.Context, ScanRecord) error {
	return nil
}

func (s *planScanRepositoryStub) Find(context.Context, string) (ScanRecord, error) {
	if s.err != nil {
		return ScanRecord{}, s.err
	}
	return s.record, nil
}

func (s *planScanRepositoryStub) FindLatest(context.Context) (ScanRecord, error) {
	if s.err != nil {
		return ScanRecord{}, s.err
	}
	return s.record, nil
}
