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

// workspace_repository.go는 internal/app에 정의된 app.WorkspaceRepository
// 인터페이스를 SQLite로 구현한다(아래 var _ 컴파일타임 assertion이 이를
// 보장). 두 테이블을 다룬다: workspace 자체를 저장하는 workspaces
// 테이블(Upsert)과, 워크스페이스에 속한 프로젝트 ID 목록을 저장하는
// workspace_projects 매핑 테이블(ReplaceMembers). ReplaceMembers는
// 멤버 목록을 delete-then-insert 방식으로 트랜잭션 안에서 통째로
// 교체하며 중복 ID는 무시한다. project_repository.go,
// resource_repository.go, dependency_repository.go, scan_repository.go와
// 함께 도메인 엔티티 하나당 파일 하나 구조를 이루는 형제 파일이다.
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
