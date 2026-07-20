package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/app"
	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/pathutil"
)

var ErrResourceNotFound = errors.New("resource not found")

type ResourceRepository struct {
	db *sql.DB
}

var _ app.ResourceRepository = (*ResourceRepository)(nil)

func NewResourceRepository(db *sql.DB) *ResourceRepository {
	return &ResourceRepository{db: db}
}

func (r *ResourceRepository) Upsert(ctx context.Context, resource domain.Resource) error {
	if err := validateResource(resource); err != nil {
		return err
	}

	var lastModifiedAt any
	if resource.LastModifiedAt != nil {
		lastModifiedAt = resource.LastModifiedAt.UTC().Format(time.RFC3339Nano)
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO resources (
			id, resource_type, name, version, path, normalized_path,
			logical_size, size_known, reclaimable_size, regenerable, system_managed,
			last_modified_at, last_observed_at, risk, confidence, regeneration_command
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			resource_type = excluded.resource_type,
			name = excluded.name,
			version = excluded.version,
			path = excluded.path,
			normalized_path = excluded.normalized_path,
			logical_size = excluded.logical_size,
			size_known = excluded.size_known,
			reclaimable_size = excluded.reclaimable_size,
			regenerable = excluded.regenerable,
			system_managed = excluded.system_managed,
			last_modified_at = excluded.last_modified_at,
			last_observed_at = excluded.last_observed_at,
			risk = excluded.risk,
			confidence = excluded.confidence,
			regeneration_command = excluded.regeneration_command
	`,
		resource.ID, resource.Type, resource.Name, nullableString(resource.Version),
		resource.DisplayPath, resource.NormalizedPath, resource.LogicalSize, boolInt(resource.SizeKnown),
		resource.ReclaimableSize, boolInt(resource.Regenerable), boolInt(resource.SystemManaged),
		lastModifiedAt, resource.LastObservedAt.UTC().Format(time.RFC3339Nano),
		resource.Risk, resource.Confidence, nullableString(resource.RegenerationCommand),
	)
	if err != nil {
		return fmt.Errorf("upsert resource %q: %w", resource.ID, err)
	}
	return nil
}

func (r *ResourceRepository) FindByID(ctx context.Context, id string) (domain.Resource, error) {
	row := r.db.QueryRowContext(ctx, resourceSelect+` WHERE id = ?`, id)
	resource, err := scanResource(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Resource{}, fmt.Errorf("%w: %s", ErrResourceNotFound, id)
	}
	if err != nil {
		return domain.Resource{}, fmt.Errorf("find resource %q: %w", id, err)
	}
	return resource, nil
}

func (r *ResourceRepository) ListByType(ctx context.Context, resourceType domain.ResourceType) ([]domain.Resource, error) {
	rows, err := r.db.QueryContext(ctx, resourceSelect+` WHERE resource_type = ? ORDER BY name, version, normalized_path`, resourceType)
	if err != nil {
		return nil, fmt.Errorf("list resources of type %q: %w", resourceType, err)
	}
	defer rows.Close()

	resources := make([]domain.Resource, 0)
	for rows.Next() {
		resource, err := scanResource(rows)
		if err != nil {
			return nil, fmt.Errorf("decode resource of type %q: %w", resourceType, err)
		}
		resources = append(resources, resource)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list resources of type %q: %w", resourceType, err)
	}
	return resources, nil
}

// List returns every observed resource, ordered by type, name, and version.
func (r *ResourceRepository) List(ctx context.Context) ([]domain.Resource, error) {
	rows, err := r.db.QueryContext(ctx, resourceSelect+` ORDER BY resource_type, name, version, normalized_path`)
	if err != nil {
		return nil, fmt.Errorf("list resources: %w", err)
	}
	defer rows.Close()

	resources := make([]domain.Resource, 0)
	for rows.Next() {
		resource, err := scanResource(rows)
		if err != nil {
			return nil, fmt.Errorf("decode resource: %w", err)
		}
		resources = append(resources, resource)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list resources: %w", err)
	}
	return resources, nil
}

const resourceSelect = `
	SELECT id, resource_type, name, version, path, normalized_path,
		logical_size, size_known, reclaimable_size, regenerable, system_managed,
		last_modified_at, last_observed_at, risk, confidence, regeneration_command
	FROM resources`

type rowScanner interface {
	Scan(...any) error
}

func scanResource(row rowScanner) (domain.Resource, error) {
	var resource domain.Resource
	var version sql.NullString
	var lastModifiedAt sql.NullString
	var lastObservedAt string
	var regenerable int
	var systemManaged int
	var sizeKnown int
	var regenerationCommand sql.NullString
	err := row.Scan(
		&resource.ID, &resource.Type, &resource.Name, &version,
		&resource.DisplayPath, &resource.NormalizedPath,
		&resource.LogicalSize, &sizeKnown, &resource.ReclaimableSize,
		&regenerable, &systemManaged, &lastModifiedAt, &lastObservedAt,
		&resource.Risk, &resource.Confidence, &regenerationCommand,
	)
	if err != nil {
		return domain.Resource{}, err
	}
	if version.Valid {
		resource.Version = version.String
	}
	if regenerationCommand.Valid {
		resource.RegenerationCommand = regenerationCommand.String
	}
	resource.Regenerable = regenerable == 1
	resource.SystemManaged = systemManaged == 1
	resource.SizeKnown = sizeKnown == 1
	if lastModifiedAt.Valid {
		parsed, err := time.Parse(time.RFC3339Nano, lastModifiedAt.String)
		if err != nil {
			return domain.Resource{}, fmt.Errorf("decode last modified time: %w", err)
		}
		resource.LastModifiedAt = &parsed
	}
	parsed, err := time.Parse(time.RFC3339Nano, lastObservedAt)
	if err != nil {
		return domain.Resource{}, fmt.Errorf("decode last observed time: %w", err)
	}
	resource.LastObservedAt = parsed
	return resource, nil
}

func validateResource(resource domain.Resource) error {
	if resource.Type == "" || resource.Name == "" || resource.DisplayPath == "" || resource.NormalizedPath == "" {
		return errors.New("resource type, name, display path, and normalized path are required")
	}
	normalized, err := pathutil.Normalize(resource.DisplayPath)
	if err != nil {
		return fmt.Errorf("normalize resource display path: %w", err)
	}
	if normalized != resource.NormalizedPath {
		return fmt.Errorf("resource normalized path %q does not match display path identity %q", resource.NormalizedPath, normalized)
	}
	wantID := domain.ResourceID(resource.Type, resource.Version, resource.NormalizedPath)
	if resource.ID != wantID {
		return fmt.Errorf("resource ID %q does not match stable ID %q", resource.ID, wantID)
	}
	if resource.LogicalSize < 0 || resource.ReclaimableSize < 0 || resource.ReclaimableSize > resource.LogicalSize {
		return errors.New("resource sizes must be non-negative and reclaimable size cannot exceed logical size")
	}
	if resource.LastObservedAt.IsZero() {
		return errors.New("resource last observed time is required")
	}
	if resource.Confidence < 0 || resource.Confidence > 100 {
		return errors.New("resource confidence must be between 0 and 100")
	}
	if resource.Risk != domain.RiskSafe && resource.Risk != domain.RiskReview && resource.Risk != domain.RiskBlocked {
		return fmt.Errorf("invalid resource risk %q", resource.Risk)
	}
	return nil
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
