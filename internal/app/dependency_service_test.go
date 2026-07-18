package app

import (
	"context"
	"errors"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

type dependencyRepositoryStub struct {
	upserted []DependencyObservation
	failOn   string // scanID that should fail, for testing stop-on-error
}

func (r *dependencyRepositoryStub) UpsertGraph(_ context.Context, scanID string, dependency domain.Dependency, evidence []domain.Evidence) error {
	if scanID == r.failOn {
		return errors.New("simulated upsert failure")
	}
	r.upserted = append(r.upserted, DependencyObservation{Dependency: dependency, Evidence: evidence})
	return nil
}

func (*dependencyRepositoryStub) FindResourcesByProject(context.Context, string) ([]domain.Dependency, error) {
	return nil, errors.New("not implemented")
}

func (*dependencyRepositoryStub) FindProjectsByResource(context.Context, string) ([]domain.Dependency, error) {
	return nil, errors.New("not implemented")
}

func (*dependencyRepositoryStub) FindEvidence(context.Context, string) ([]domain.Evidence, error) {
	return nil, errors.New("not implemented")
}

func TestDependencyServicePersistsEveryObservation(t *testing.T) {
	repository := &dependencyRepositoryStub{}
	service := NewDependencyService(repository)

	observations := []DependencyObservation{
		{
			Dependency: domain.Dependency{ID: "dep-1", SourceType: domain.NodeProject, SourceID: "project-1", TargetType: domain.NodeResource, TargetID: "resource-1", Relation: domain.RelationRequires},
			Evidence:   []domain.Evidence{{ID: "ev-1", DependencyID: "dep-1", Kind: domain.EvidenceDeclared}},
		},
		{
			Dependency: domain.Dependency{ID: "dep-2", SourceType: domain.NodeProject, SourceID: "project-1", TargetType: domain.NodeResource, TargetID: "resource-2", Relation: domain.RelationRequires},
			Evidence:   []domain.Evidence{{ID: "ev-2", DependencyID: "dep-2", Kind: domain.EvidenceResolved}},
		},
	}

	if err := service.Persist(context.Background(), "scan-1", observations); err != nil {
		t.Fatalf("Persist() error = %v", err)
	}
	if len(repository.upserted) != 2 {
		t.Fatalf("upserted %d observations, want 2: %+v", len(repository.upserted), repository.upserted)
	}
}

func TestDependencyServicePersist_StopsOnFirstError(t *testing.T) {
	repository := &dependencyRepositoryStub{failOn: "bad-scan"}
	service := NewDependencyService(repository)

	observations := []DependencyObservation{
		{Dependency: domain.Dependency{ID: "dep-1"}},
	}

	err := service.Persist(context.Background(), "bad-scan", observations)
	if err == nil {
		t.Fatal("Persist() error = nil, want error from repository")
	}
	if len(repository.upserted) != 0 {
		t.Fatalf("upserted = %+v, want none persisted", repository.upserted)
	}
}
