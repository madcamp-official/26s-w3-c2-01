package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/app"
	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

func TestDependencyRepositoryStoresGraphAndQueriesBothDirections(t *testing.T) {
	repository, scanID := newTestDependencyRepository(t)
	dependency, evidence := testGraph()
	if err := repository.UpsertGraph(context.Background(), scanID, dependency, evidence); err != nil {
		t.Fatalf("UpsertGraph() error = %v", err)
	}

	resources, err := repository.FindResourcesByProject(context.Background(), dependency.SourceID)
	if err != nil || len(resources) != 1 || resources[0] != dependency {
		t.Fatalf("FindResourcesByProject() = %#v, %v", resources, err)
	}
	projects, err := repository.FindProjectsByResource(context.Background(), dependency.TargetID)
	if err != nil || len(projects) != 1 || projects[0] != dependency {
		t.Fatalf("FindProjectsByResource() = %#v, %v", projects, err)
	}
	gotEvidence, err := repository.FindEvidence(context.Background(), dependency.ID)
	if err != nil || len(gotEvidence) != 1 || gotEvidence[0] != evidence[0] {
		t.Fatalf("FindEvidence() = %#v, %v", gotEvidence, err)
	}
}

func TestDependencyRepositoryRejectsGraphAtomically(t *testing.T) {
	repository, _ := newTestDependencyRepository(t)
	dependency, evidence := testGraph()
	if err := repository.UpsertGraph(context.Background(), "missing-scan", dependency, evidence); err == nil {
		t.Fatal("UpsertGraph() error = nil, want evidence foreign key error")
	}

	got, err := repository.FindResourcesByProject(context.Background(), dependency.SourceID)
	if err != nil {
		t.Fatalf("FindResourcesByProject() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("dependencies = %#v, want transaction rollback", got)
	}
}

func TestDependencyRepositoryRefreshesDuplicateEvidence(t *testing.T) {
	repository, scanID := newTestDependencyRepository(t)
	dependency, evidence := testGraph()
	if err := repository.UpsertGraph(context.Background(), scanID, dependency, evidence); err != nil {
		t.Fatalf("UpsertGraph(first) error = %v", err)
	}
	evidence[0].CollectedAt = evidence[0].CollectedAt.Add(time.Hour)
	if err := repository.UpsertGraph(context.Background(), scanID, dependency, evidence); err != nil {
		t.Fatalf("UpsertGraph(refresh) error = %v", err)
	}
	got, err := repository.FindEvidence(context.Background(), dependency.ID)
	if err != nil || len(got) != 1 || !got[0].CollectedAt.Equal(evidence[0].CollectedAt) {
		t.Fatalf("FindEvidence() = %#v, %v, want one refreshed row", got, err)
	}
}

func newTestDependencyRepository(t *testing.T) (*DependencyRepository, string) {
	t.Helper()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	scanID := "scan-day4"
	if err := NewScanRepository(db).Save(context.Background(), app.ScanRecord{
		ID: scanID, StartedAt: time.Now(), Roots: []string{`D:\Projects`}, Status: app.ScanStatusRunning,
	}); err != nil {
		t.Fatalf("Save(scan) error = %v", err)
	}
	return NewDependencyRepository(db), scanID
}

func testGraph() (domain.Dependency, []domain.Evidence) {
	dependency := domain.Dependency{
		SourceType: domain.NodeProject,
		SourceID:   "project-on-d-drive",
		TargetType: domain.NodeResource,
		TargetID:   "sdk-on-c-drive",
		Relation:   domain.RelationRequires,
		Confidence: 75,
	}
	dependency.ID = domain.DependencyID(dependency.SourceType, dependency.SourceID, dependency.Relation, dependency.TargetType, dependency.TargetID)
	evidence := domain.Evidence{
		DependencyID:  dependency.ID,
		Kind:          domain.EvidenceDeclared,
		Claim:         domain.ClaimRequiredDependency,
		Polarity:      domain.EvidenceSupports,
		Method:        "fixture-resolution",
		SourceFamily:  "fixture-manifest",
		SourceHash:    "sha256:fixture",
		SourcePath:    `D:\Projects\Game\Game.vcxproj`,
		Property:      "WindowsTargetPlatformVersion",
		RawValue:      "10.0",
		ResolvedValue: "10.0.22621.0",
		CollectedAt:   time.Date(2026, 7, 18, 8, 0, 0, 0, time.UTC),
	}
	evidence.ID = domain.EvidenceID(evidence.DependencyID, evidence.Kind, evidence.Claim, evidence.Polarity, evidence.SourcePath,
		evidence.Property, evidence.RawValue, evidence.ResolvedValue)
	return dependency, []domain.Evidence{evidence}
}
