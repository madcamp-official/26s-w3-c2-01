package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/app"
	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

type DependencyRepository struct {
	db *sql.DB
}

var _ app.DependencyRepository = (*DependencyRepository)(nil)

func NewDependencyRepository(db *sql.DB) *DependencyRepository {
	return &DependencyRepository{db: db}
}

func (r *DependencyRepository) UpsertGraph(ctx context.Context, scanID string, dependency domain.Dependency, evidence []domain.Evidence) error {
	if scanID == "" {
		return errors.New("scan ID is required")
	}
	if err := validateDependency(dependency); err != nil {
		return err
	}
	for _, item := range evidence {
		if err := validateEvidence(dependency.ID, item); err != nil {
			return err
		}
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin dependency graph transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO dependencies (id, source_type, source_id, target_type, target_id, relation, confidence)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			source_type = excluded.source_type,
			source_id = excluded.source_id,
			target_type = excluded.target_type,
			target_id = excluded.target_id,
			relation = excluded.relation,
			confidence = excluded.confidence
	`, dependency.ID, dependency.SourceType, dependency.SourceID, dependency.TargetType,
		dependency.TargetID, dependency.Relation, dependency.Confidence)
	if err != nil {
		return fmt.Errorf("upsert dependency %q: %w", dependency.ID, err)
	}

	for _, item := range evidence {
		var validUntil any
		if item.ValidUntil != nil {
			validUntil = item.ValidUntil.UTC().Format(time.RFC3339Nano)
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO evidence (
				id, dependency_id, scan_id, evidence_type, source_path,
				property_name, raw_value, resolved_value, collected_at,
				claim, method, source_family, source_hash, valid_until, polarity
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				dependency_id = excluded.dependency_id,
				scan_id = excluded.scan_id,
				evidence_type = excluded.evidence_type,
				source_path = excluded.source_path,
				property_name = excluded.property_name,
				raw_value = excluded.raw_value,
				resolved_value = excluded.resolved_value,
				collected_at = excluded.collected_at,
				claim = excluded.claim,
				method = excluded.method,
				source_family = excluded.source_family,
				source_hash = excluded.source_hash,
				valid_until = excluded.valid_until,
				polarity = excluded.polarity
		`, item.ID, item.DependencyID, scanID, item.Kind, item.SourcePath,
			nullableString(item.Property), nullableString(item.RawValue), nullableString(item.ResolvedValue),
			item.CollectedAt.UTC().Format(time.RFC3339Nano), nullableString(string(item.Claim)),
			item.Method, item.SourceFamily, item.SourceHash, validUntil, item.Polarity)
		if err != nil {
			return fmt.Errorf("upsert evidence %q: %w", item.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit dependency graph: %w", err)
	}
	return nil
}

func (r *DependencyRepository) FindResourcesByProject(ctx context.Context, projectID string) ([]domain.Dependency, error) {
	return r.findDependencies(ctx, `
		WHERE source_type = ? AND source_id = ? AND target_type = ?
		ORDER BY relation, target_id
	`, domain.NodeProject, projectID, domain.NodeResource)
}

func (r *DependencyRepository) FindProjectsByResource(ctx context.Context, resourceID string) ([]domain.Dependency, error) {
	return r.findDependencies(ctx, `
		WHERE source_type = ? AND target_type = ? AND target_id = ?
		ORDER BY relation, source_id
	`, domain.NodeProject, domain.NodeResource, resourceID)
}

func (r *DependencyRepository) FindEvidence(ctx context.Context, dependencyID string) ([]domain.Evidence, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, dependency_id, evidence_type, source_path, property_name,
			raw_value, resolved_value, collected_at, claim, method,
			source_family, source_hash, valid_until, polarity
		FROM evidence
		WHERE dependency_id = ?
		ORDER BY evidence_type, source_path, property_name, id
	`, dependencyID)
	if err != nil {
		return nil, fmt.Errorf("find evidence for dependency %q: %w", dependencyID, err)
	}
	defer rows.Close()

	items := make([]domain.Evidence, 0)
	for rows.Next() {
		var item domain.Evidence
		var property, rawValue, resolvedValue, claim, validUntil sql.NullString
		var collectedAt string
		if err := rows.Scan(&item.ID, &item.DependencyID, &item.Kind, &item.SourcePath,
			&property, &rawValue, &resolvedValue, &collectedAt, &claim, &item.Method,
			&item.SourceFamily, &item.SourceHash, &validUntil, &item.Polarity); err != nil {
			return nil, fmt.Errorf("decode evidence for dependency %q: %w", dependencyID, err)
		}
		item.Property = property.String
		item.RawValue = rawValue.String
		item.ResolvedValue = resolvedValue.String
		item.Claim = domain.ClaimType(claim.String)
		item.CollectedAt, err = time.Parse(time.RFC3339Nano, collectedAt)
		if err != nil {
			return nil, fmt.Errorf("decode evidence %q collection time: %w", item.ID, err)
		}
		if validUntil.Valid {
			parsed, parseErr := time.Parse(time.RFC3339Nano, validUntil.String)
			if parseErr != nil {
				return nil, fmt.Errorf("decode evidence %q validity: %w", item.ID, parseErr)
			}
			item.ValidUntil = &parsed
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("find evidence for dependency %q: %w", dependencyID, err)
	}
	return items, nil
}

func (r *DependencyRepository) findDependencies(ctx context.Context, clause string, args ...any) ([]domain.Dependency, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, source_type, source_id, target_type, target_id, relation, confidence
		FROM dependencies
	`+clause, args...)
	if err != nil {
		return nil, fmt.Errorf("query dependency graph: %w", err)
	}
	defer rows.Close()

	dependencies := make([]domain.Dependency, 0)
	for rows.Next() {
		var dependency domain.Dependency
		if err := rows.Scan(&dependency.ID, &dependency.SourceType, &dependency.SourceID,
			&dependency.TargetType, &dependency.TargetID, &dependency.Relation, &dependency.Confidence); err != nil {
			return nil, fmt.Errorf("decode dependency graph: %w", err)
		}
		dependencies = append(dependencies, dependency)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("query dependency graph: %w", err)
	}
	return dependencies, nil
}

func validateDependency(dependency domain.Dependency) error {
	if dependency.SourceType != domain.NodeProject || dependency.TargetType != domain.NodeResource {
		return errors.New("Day 4 dependency must be a PROJECT to RESOURCE edge")
	}
	if dependency.SourceID == "" || dependency.TargetID == "" {
		return errors.New("dependency source and target IDs are required")
	}
	if dependency.Relation != domain.RelationRequires && dependency.Relation != domain.RelationOwns {
		return fmt.Errorf("unsupported dependency relation %q", dependency.Relation)
	}
	wantID := domain.DependencyID(dependency.SourceType, dependency.SourceID, dependency.Relation, dependency.TargetType, dependency.TargetID)
	if dependency.ID != wantID {
		return fmt.Errorf("dependency ID %q does not match stable ID %q", dependency.ID, wantID)
	}
	if dependency.Confidence < 0 || dependency.Confidence > 100 {
		return errors.New("dependency confidence must be between 0 and 100")
	}
	return nil
}

func validateEvidence(dependencyID string, evidence domain.Evidence) error {
	if evidence.DependencyID != dependencyID {
		return fmt.Errorf("evidence dependency ID %q does not match graph dependency %q", evidence.DependencyID, dependencyID)
	}
	if evidence.SourcePath == "" || evidence.CollectedAt.IsZero() {
		return errors.New("evidence source path and collection time are required")
	}
	switch evidence.Kind {
	case domain.EvidenceDeclared, domain.EvidenceResolved, domain.EvidenceObserved, domain.EvidenceInferred, domain.EvidenceUnknown:
	default:
		return fmt.Errorf("invalid evidence kind %q", evidence.Kind)
	}
	wantID := domain.EvidenceID(evidence.DependencyID, evidence.Kind, evidence.SourcePath,
		evidence.Property, evidence.RawValue, evidence.ResolvedValue)
	if evidence.ID != wantID {
		return fmt.Errorf("evidence ID %q does not match stable ID %q", evidence.ID, wantID)
	}
	return nil
}
