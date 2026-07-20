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

var ErrCleanupPlanNotFound = errors.New("cleanup plan not found")

type CleanupPlanRepository struct {
	db *sql.DB
}

var _ app.CleanupPlanRepository = (*CleanupPlanRepository)(nil)

func NewCleanupPlanRepository(db *sql.DB) *CleanupPlanRepository {
	return &CleanupPlanRepository{db: db}
}

func (r *CleanupPlanRepository) Create(ctx context.Context, plan domain.CleanupPlan) error {
	if err := validateCleanupPlan(plan); err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin cleanup plan transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO cleanup_plans (id, created_at, target_bytes, selected_bytes, status)
		VALUES (?, ?, ?, ?, ?)
	`, plan.ID, plan.CreatedAt.UTC().Format(time.RFC3339Nano), plan.TargetBytes, plan.SelectedBytes, plan.Status); err != nil {
		return fmt.Errorf("insert cleanup plan %q: %w", plan.ID, err)
	}
	for _, item := range plan.Items {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO cleanup_items (
				id, plan_id, resource_id, expected_bytes, risk, action_type, reason,
				normalized_path, expected_type, expected_modified_at,
				confidence_at_planning, owner_project_id, scan_id, regeneration_command
			) VALUES (?, ?, ?, ?, ?, 'QUARANTINE', '', ?, ?, ?, ?, ?, ?, ?)
		`, item.ID, plan.ID, item.ResourceID, item.ExpectedSize, item.RiskAtPlanning,
			item.NormalizedPath, item.ExpectedType,
			item.ExpectedModifiedTime.UTC().Format(time.RFC3339Nano),
			item.ConfidenceAtPlanning, nullableString(item.OwnerProjectID), item.ScanID,
			nullableString(item.RegenerationCommand)); err != nil {
			return fmt.Errorf("insert cleanup plan item %q: %w", item.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit cleanup plan %q: %w", plan.ID, err)
	}
	return nil
}

func (r *CleanupPlanRepository) FindByID(ctx context.Context, id string) (domain.CleanupPlan, error) {
	var plan domain.CleanupPlan
	var createdAt string
	err := r.db.QueryRowContext(ctx, `
		SELECT id, created_at, target_bytes, selected_bytes, status
		FROM cleanup_plans WHERE id = ?
	`, id).Scan(&plan.ID, &createdAt, &plan.TargetBytes, &plan.SelectedBytes, &plan.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.CleanupPlan{}, fmt.Errorf("%w: %s", ErrCleanupPlanNotFound, id)
	}
	if err != nil {
		return domain.CleanupPlan{}, fmt.Errorf("find cleanup plan %q: %w", id, err)
	}
	plan.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return domain.CleanupPlan{}, fmt.Errorf("decode cleanup plan created time: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, resource_id, normalized_path, expected_type, expected_bytes,
			expected_modified_at, risk, confidence_at_planning,
			owner_project_id, scan_id, regeneration_command
		FROM cleanup_items WHERE plan_id = ? ORDER BY id
	`, id)
	if err != nil {
		return domain.CleanupPlan{}, fmt.Errorf("list cleanup plan items %q: %w", id, err)
	}
	defer rows.Close()
	for rows.Next() {
		var item domain.CleanupPlanItem
		var modifiedAt string
		var ownerProjectID, regenerationCommand sql.NullString
		if err := rows.Scan(&item.ID, &item.ResourceID, &item.NormalizedPath,
			&item.ExpectedType, &item.ExpectedSize, &modifiedAt,
			&item.RiskAtPlanning, &item.ConfidenceAtPlanning,
			&ownerProjectID, &item.ScanID, &regenerationCommand); err != nil {
			return domain.CleanupPlan{}, fmt.Errorf("decode cleanup plan item: %w", err)
		}
		item.ExpectedModifiedTime, err = time.Parse(time.RFC3339Nano, modifiedAt)
		if err != nil {
			return domain.CleanupPlan{}, fmt.Errorf("decode cleanup plan item modified time: %w", err)
		}
		if ownerProjectID.Valid {
			item.OwnerProjectID = ownerProjectID.String
		}
		if regenerationCommand.Valid {
			item.RegenerationCommand = regenerationCommand.String
		}
		plan.Items = append(plan.Items, item)
	}
	if err := rows.Err(); err != nil {
		return domain.CleanupPlan{}, fmt.Errorf("list cleanup plan items %q: %w", id, err)
	}
	return plan, nil
}

func validateCleanupPlan(plan domain.CleanupPlan) error {
	if plan.ID == "" || plan.CreatedAt.IsZero() {
		return errors.New("cleanup plan ID and created time are required")
	}
	if plan.TargetBytes < 0 || plan.SelectedBytes < 0 {
		return errors.New("cleanup plan byte counts must be non-negative")
	}
	if plan.Status != domain.CleanupPlanReady && plan.Status != domain.CleanupPlanInsufficientCandidates {
		return fmt.Errorf("invalid cleanup plan status %q", plan.Status)
	}
	var selected int64
	seenResources := make(map[string]struct{}, len(plan.Items))
	for _, item := range plan.Items {
		if item.ID == "" || item.ResourceID == "" || item.NormalizedPath == "" || item.ExpectedType == "" || item.ScanID == "" || item.ExpectedModifiedTime.IsZero() {
			return errors.New("cleanup plan item snapshot fields are required")
		}
		if item.ExpectedSize < 0 || item.ConfidenceAtPlanning < 0 || item.ConfidenceAtPlanning > 100 {
			return errors.New("cleanup plan item size and confidence are invalid")
		}
		if item.RiskAtPlanning != domain.RiskSafe {
			return errors.New("cleanup plan automatically includes SAFE resources only")
		}
		if _, exists := seenResources[item.ResourceID]; exists {
			return fmt.Errorf("duplicate cleanup plan resource %q", item.ResourceID)
		}
		seenResources[item.ResourceID] = struct{}{}
		selected += item.ExpectedSize
	}
	if selected != plan.SelectedBytes {
		return fmt.Errorf("selected bytes %d do not match item total %d", plan.SelectedBytes, selected)
	}
	return nil
}
