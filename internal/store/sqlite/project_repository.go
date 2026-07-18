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

var ErrProjectNotFound = errors.New("project not found")

type ProjectRepository struct {
	db *sql.DB
}

var _ app.ProjectRepository = (*ProjectRepository)(nil)

func NewProjectRepository(db *sql.DB) *ProjectRepository {
	return &ProjectRepository{db: db}
}

func (r *ProjectRepository) UpsertObserved(ctx context.Context, scanID string, projects []domain.BuildProject) error {
	if scanID == "" {
		return errors.New("scan ID is required")
	}
	for _, project := range projects {
		if err := validateProject(project); err != nil {
			return err
		}
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin project transaction: %w", err)
	}
	defer tx.Rollback()
	for _, project := range projects {
		var lastModified any
		if !project.LastModifiedAt.IsZero() {
			lastModified = project.LastModifiedAt.UTC().Format(time.RFC3339Nano)
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO projects (
				id, name, project_type, root_path, normalized_root_path,
				manifest_path, normalized_manifest_path, drive, logical_size,
				last_modified_at, last_observed_at, status, last_observed_scan_id
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				name = excluded.name,
				project_type = excluded.project_type,
				root_path = excluded.root_path,
				normalized_root_path = excluded.normalized_root_path,
				manifest_path = excluded.manifest_path,
				normalized_manifest_path = excluded.normalized_manifest_path,
				drive = excluded.drive,
				logical_size = excluded.logical_size,
				last_modified_at = excluded.last_modified_at,
				last_observed_at = excluded.last_observed_at,
				status = excluded.status,
				last_observed_scan_id = excluded.last_observed_scan_id
		`, project.ID, project.Name, project.Type, project.RootPath, project.NormalizedRootPath,
			project.ManifestPath, project.NormalizedManifestPath, project.Drive, project.LogicalSize,
			lastModified, project.LastObservedAt.UTC().Format(time.RFC3339Nano), project.Status, scanID)
		if err != nil {
			return fmt.Errorf("upsert project %q: %w", project.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit project transaction: %w", err)
	}
	return nil
}

func (r *ProjectRepository) FindByID(ctx context.Context, id string) (domain.BuildProject, error) {
	return findProject(r.db.QueryRowContext(ctx, projectSelect+" WHERE id = ?", id), id)
}

func (r *ProjectRepository) FindByManifestPath(ctx context.Context, projectType domain.ProjectType, manifestPath string) (domain.BuildProject, error) {
	normalized, err := pathutil.Normalize(manifestPath)
	if err != nil {
		return domain.BuildProject{}, fmt.Errorf("normalize project manifest query: %w", err)
	}
	return findProject(r.db.QueryRowContext(ctx, projectSelect+" WHERE project_type = ? AND normalized_manifest_path = ?", projectType, normalized), normalized)
}

// List returns every observed project, ordered by root path.
func (r *ProjectRepository) List(ctx context.Context) ([]domain.BuildProject, error) {
	rows, err := r.db.QueryContext(ctx, projectSelect+" ORDER BY root_path")
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	var projects []domain.BuildProject
	for rows.Next() {
		project, err := scanProject(rows)
		if err != nil {
			return nil, fmt.Errorf("list projects: %w", err)
		}
		projects = append(projects, project)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	return projects, nil
}

const projectSelect = `
	SELECT id, name, project_type, root_path, normalized_root_path,
		manifest_path, normalized_manifest_path, drive, logical_size,
		last_modified_at, last_observed_at, status
	FROM projects`

// projectRow is satisfied by both *sql.Row and *sql.Rows.
type projectRow interface {
	Scan(dest ...any) error
}

func scanProject(row projectRow) (domain.BuildProject, error) {
	var project domain.BuildProject
	var lastModified sql.NullString
	var lastObserved string
	err := row.Scan(&project.ID, &project.Name, &project.Type, &project.RootPath,
		&project.NormalizedRootPath, &project.ManifestPath, &project.NormalizedManifestPath,
		&project.Drive, &project.LogicalSize, &lastModified, &lastObserved, &project.Status)
	if err != nil {
		return domain.BuildProject{}, err
	}
	if lastModified.Valid {
		project.LastModifiedAt, err = time.Parse(time.RFC3339Nano, lastModified.String)
		if err != nil {
			return domain.BuildProject{}, fmt.Errorf("decode project last modified time: %w", err)
		}
	}
	project.LastObservedAt, err = time.Parse(time.RFC3339Nano, lastObserved)
	if err != nil {
		return domain.BuildProject{}, fmt.Errorf("decode project last observed time: %w", err)
	}
	return project, nil
}

func findProject(row *sql.Row, key string) (domain.BuildProject, error) {
	project, err := scanProject(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.BuildProject{}, fmt.Errorf("%w: %s", ErrProjectNotFound, key)
	}
	if err != nil {
		return domain.BuildProject{}, fmt.Errorf("find project %q: %w", key, err)
	}
	return project, nil
}

func validateProject(project domain.BuildProject) error {
	if project.ID == "" || project.Name == "" || project.Type == "" || project.RootPath == "" ||
		project.NormalizedRootPath == "" || project.ManifestPath == "" || project.NormalizedManifestPath == "" {
		return errors.New("project identity, name, type, root, and manifest fields are required")
	}
	if project.ID != domain.ProjectID(project.Type, project.NormalizedManifestPath) {
		return errors.New("project ID does not match stable identity")
	}
	if project.LogicalSize < 0 || project.LastObservedAt.IsZero() || project.Status != domain.ProjectStatusActive {
		return errors.New("observed project requires non-negative size, observation time, and ACTIVE status")
	}
	return nil
}
