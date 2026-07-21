package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/madcamp-official/26s-w3-c2-01/internal/app"
)

type ScanIssueRepository struct {
	db *sql.DB
}

var _ app.ScanIssueRepository = (*ScanIssueRepository)(nil)

func NewScanIssueRepository(db *sql.DB) *ScanIssueRepository {
	return &ScanIssueRepository{db: db}
}

// Replace atomically replaces the complete issue snapshot for one scan.
func (r *ScanIssueRepository) Replace(ctx context.Context, scanID string, issues []app.Issue) error {
	if strings.TrimSpace(scanID) == "" {
		return fmt.Errorf("scan id is required")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin scan issue replacement: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "DELETE FROM scan_issues WHERE scan_id = ?", scanID); err != nil {
		return fmt.Errorf("delete scan issues for %q: %w", scanID, err)
	}
	for _, issue := range issues {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO scan_issues
				(scan_id, code, phase, adapter, path, operation, severity, message)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, scanID, issue.Code, issue.Phase, issue.Adapter, issue.Path, issue.Operation, issue.Severity, issue.Message); err != nil {
			return fmt.Errorf("insert scan issue for %q: %w", scanID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit scan issues for %q: %w", scanID, err)
	}
	return nil
}

func (r *ScanIssueRepository) List(ctx context.Context, filter app.IssueFilter) ([]app.Issue, error) {
	query := `SELECT code, phase, adapter, path, operation, severity, message
		FROM scan_issues WHERE scan_id = ?`
	args := []any{filter.ScanID}
	if filter.Code != "" {
		query += " AND code = ?"
		args = append(args, filter.Code)
	}
	if filter.Severity != "" {
		query += " AND severity = ?"
		args = append(args, filter.Severity)
	}
	query += " ORDER BY id"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query scan issues for %q: %w", filter.ScanID, err)
	}
	defer rows.Close()

	var issues []app.Issue
	for rows.Next() {
		var issue app.Issue
		if err := rows.Scan(&issue.Code, &issue.Phase, &issue.Adapter, &issue.Path,
			&issue.Operation, &issue.Severity, &issue.Message); err != nil {
			return nil, fmt.Errorf("decode scan issue for %q: %w", filter.ScanID, err)
		}
		issues = append(issues, issue)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate scan issues for %q: %w", filter.ScanID, err)
	}
	return issues, nil
}
