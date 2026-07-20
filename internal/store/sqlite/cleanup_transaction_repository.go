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

var ErrCleanupTransactionNotFound = errors.New("cleanup transaction not found")

type CleanupTransactionRepository struct{ db *sql.DB }

var _ app.CleanupTransactionRepository = (*CleanupTransactionRepository)(nil)

func NewCleanupTransactionRepository(db *sql.DB) *CleanupTransactionRepository {
	return &CleanupTransactionRepository{db: db}
}

func (r *CleanupTransactionRepository) Create(ctx context.Context, transaction domain.CleanupTransaction) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin cleanup transaction: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO transactions (id, plan_id, started_at, finished_at, status) VALUES (?, ?, ?, ?, ?)`, transaction.ID, transaction.PlanID, encodeTime(transaction.StartedAt), encodeOptionalTime(transaction.FinishedAt), transaction.Status); err != nil {
		return fmt.Errorf("insert cleanup transaction %q: %w", transaction.ID, err)
	}
	for _, item := range transaction.Items {
		if _, err := tx.ExecContext(ctx, `INSERT INTO transaction_items (id, transaction_id, plan_item_id, resource_id, original_path, quarantine_path, manifest_path, expected_bytes, status, reason) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, item.ID, transaction.ID, item.PlanItemID, item.ResourceID, item.OriginalPath, item.QuarantinePath, item.ManifestPath, item.ExpectedSize, item.Status, item.Reason); err != nil {
			return fmt.Errorf("insert cleanup transaction item %q: %w", item.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit cleanup transaction: %w", err)
	}
	return nil
}

func (r *CleanupTransactionRepository) Update(ctx context.Context, transaction domain.CleanupTransaction) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin cleanup transaction update: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE transactions SET finished_at = ?, status = ? WHERE id = ?`, encodeOptionalTime(transaction.FinishedAt), transaction.Status, transaction.ID)
	if err != nil {
		return fmt.Errorf("update cleanup transaction %q: %w", transaction.ID, err)
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return fmt.Errorf("%w: %s", ErrCleanupTransactionNotFound, transaction.ID)
	}
	for _, item := range transaction.Items {
		if _, err := tx.ExecContext(ctx, `UPDATE transaction_items SET quarantine_path = ?, manifest_path = ?, status = ?, reason = ? WHERE id = ? AND transaction_id = ?`, item.QuarantinePath, item.ManifestPath, item.Status, item.Reason, item.ID, transaction.ID); err != nil {
			return fmt.Errorf("update cleanup transaction item %q: %w", item.ID, err)
		}
	}
	return tx.Commit()
}

func (r *CleanupTransactionRepository) FindByID(ctx context.Context, id string) (domain.CleanupTransaction, error) {
	var transaction domain.CleanupTransaction
	var started string
	var finished sql.NullString
	err := r.db.QueryRowContext(ctx, `SELECT id, plan_id, started_at, finished_at, status FROM transactions WHERE id = ?`, id).Scan(&transaction.ID, &transaction.PlanID, &started, &finished, &transaction.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.CleanupTransaction{}, fmt.Errorf("%w: %s", ErrCleanupTransactionNotFound, id)
	}
	if err != nil {
		return domain.CleanupTransaction{}, fmt.Errorf("find cleanup transaction %q: %w", id, err)
	}
	transaction.StartedAt, err = time.Parse(time.RFC3339Nano, started)
	if err != nil {
		return domain.CleanupTransaction{}, err
	}
	if finished.Valid {
		value, parseErr := time.Parse(time.RFC3339Nano, finished.String)
		if parseErr != nil {
			return domain.CleanupTransaction{}, parseErr
		}
		transaction.FinishedAt = &value
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, plan_item_id, resource_id, original_path, quarantine_path, manifest_path, expected_bytes, status, reason FROM transaction_items WHERE transaction_id = ? ORDER BY id`, id)
	if err != nil {
		return domain.CleanupTransaction{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var item domain.CleanupTransactionItem
		if err := rows.Scan(&item.ID, &item.PlanItemID, &item.ResourceID, &item.OriginalPath, &item.QuarantinePath, &item.ManifestPath, &item.ExpectedSize, &item.Status, &item.Reason); err != nil {
			return domain.CleanupTransaction{}, err
		}
		transaction.Items = append(transaction.Items, item)
	}
	return transaction, rows.Err()
}

func (r *CleanupTransactionRepository) List(ctx context.Context) ([]domain.CleanupTransaction, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id FROM transactions ORDER BY started_at DESC, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	result := make([]domain.CleanupTransaction, 0, len(ids))
	for _, id := range ids {
		transaction, err := r.FindByID(ctx, id)
		if err != nil {
			return nil, err
		}
		result = append(result, transaction)
	}
	return result, nil
}

func encodeTime(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }
func encodeOptionalTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return encodeTime(*value)
}
