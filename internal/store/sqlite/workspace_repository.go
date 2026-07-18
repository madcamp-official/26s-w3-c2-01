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

type WorkspaceRepository struct {
	db *sql.DB
}

var _ app.WorkspaceRepository = (*WorkspaceRepository)(nil)

func NewWorkspaceRepository(db *sql.DB) *WorkspaceRepository {
	return &WorkspaceRepository{db: db}
}

func (r *WorkspaceRepository) Upsert(ctx context.Context, scanID string, workspace domain.Workspace) error {
	if scanID == "" || workspace.ID == "" || workspace.Name == "" || workspace.Type == "" ||
		workspace.ManifestPath == "" || workspace.NormalizedManifestPath == "" || workspace.LastObservedAt.IsZero() {
		return errors.New("scan and complete workspace identity are required")
	}
	if workspace.ID != domain.WorkspaceID(workspace.Type, workspace.NormalizedManifestPath) {
		return errors.New("workspace ID does not match stable identity")
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO workspaces (
			id, name, workspace_type, manifest_path, normalized_manifest_path,
			last_observed_at, last_observed_scan_id
		) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			workspace_type = excluded.workspace_type,
			manifest_path = excluded.manifest_path,
			normalized_manifest_path = excluded.normalized_manifest_path,
			last_observed_at = excluded.last_observed_at,
			last_observed_scan_id = excluded.last_observed_scan_id
	`, workspace.ID, workspace.Name, workspace.Type, workspace.ManifestPath,
		workspace.NormalizedManifestPath, workspace.LastObservedAt.UTC().Format(time.RFC3339Nano), scanID)
	if err != nil {
		return fmt.Errorf("upsert workspace %q: %w", workspace.ID, err)
	}
	return nil
}

func (r *WorkspaceRepository) ReplaceMembers(ctx context.Context, workspaceID string, projectIDs []string) error {
	if workspaceID == "" {
		return errors.New("workspace ID is required")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin workspace membership transaction: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "DELETE FROM workspace_projects WHERE workspace_id = ?", workspaceID); err != nil {
		return fmt.Errorf("clear workspace %q members: %w", workspaceID, err)
	}
	seen := make(map[string]struct{}, len(projectIDs))
	for _, projectID := range projectIDs {
		if projectID == "" {
			return errors.New("workspace project ID must not be empty")
		}
		if _, exists := seen[projectID]; exists {
			continue
		}
		seen[projectID] = struct{}{}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO workspace_projects (workspace_id, project_id) VALUES (?, ?)
		`, workspaceID, projectID); err != nil {
			return fmt.Errorf("add project %q to workspace %q: %w", projectID, workspaceID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit workspace membership: %w", err)
	}
	return nil
}
