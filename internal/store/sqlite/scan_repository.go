package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var ErrScanNotFound = errors.New("scan not found")

type ScanRecord struct {
	ID         string
	StartedAt  time.Time
	FinishedAt *time.Time
	Roots      []string
	FileCount  int64
	ErrorCount int64
	Status     string
}

type ScanRepository struct {
	db *sql.DB
}

func NewScanRepository(db *sql.DB) *ScanRepository {
	return &ScanRepository{db: db}
}

// Save inserts or replaces the mutable summary of a scan. Roots are encoded as
// JSON so Windows paths are preserved without delimiter ambiguity.
func (r *ScanRepository) Save(ctx context.Context, scan ScanRecord) error {
	if err := scan.validate(); err != nil {
		return err
	}
	roots, err := json.Marshal(scan.Roots)
	if err != nil {
		return fmt.Errorf("encode scan roots: %w", err)
	}

	var finishedAt any
	if scan.FinishedAt != nil {
		finishedAt = scan.FinishedAt.UTC().Format(time.RFC3339Nano)
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO scans (id, started_at, finished_at, roots, file_count, error_count, status)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			finished_at = excluded.finished_at,
			roots = excluded.roots,
			file_count = excluded.file_count,
			error_count = excluded.error_count,
			status = excluded.status
	`, scan.ID, scan.StartedAt.UTC().Format(time.RFC3339Nano), finishedAt, string(roots), scan.FileCount, scan.ErrorCount, scan.Status)
	if err != nil {
		return fmt.Errorf("save scan %q: %w", scan.ID, err)
	}
	return nil
}

func (r *ScanRepository) Find(ctx context.Context, id string) (ScanRecord, error) {
	var record ScanRecord
	var startedAt string
	var finishedAt sql.NullString
	var roots string
	err := r.db.QueryRowContext(ctx, `
		SELECT id, started_at, finished_at, roots, file_count, error_count, status
		FROM scans
		WHERE id = ?
	`, id).Scan(
		&record.ID, &startedAt, &finishedAt, &roots,
		&record.FileCount, &record.ErrorCount, &record.Status,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ScanRecord{}, fmt.Errorf("%w: %s", ErrScanNotFound, id)
	}
	if err != nil {
		return ScanRecord{}, fmt.Errorf("find scan %q: %w", id, err)
	}

	record.StartedAt, err = time.Parse(time.RFC3339Nano, startedAt)
	if err != nil {
		return ScanRecord{}, fmt.Errorf("decode scan %q start time: %w", id, err)
	}
	if finishedAt.Valid {
		parsed, err := time.Parse(time.RFC3339Nano, finishedAt.String)
		if err != nil {
			return ScanRecord{}, fmt.Errorf("decode scan %q finish time: %w", id, err)
		}
		record.FinishedAt = &parsed
	}
	if err := json.Unmarshal([]byte(roots), &record.Roots); err != nil {
		return ScanRecord{}, fmt.Errorf("decode scan %q roots: %w", id, err)
	}
	return record, nil
}

func (s ScanRecord) validate() error {
	if s.ID == "" {
		return errors.New("scan id is required")
	}
	if s.StartedAt.IsZero() {
		return errors.New("scan start time is required")
	}
	if len(s.Roots) == 0 {
		return errors.New("at least one scan root is required")
	}
	if s.FileCount < 0 || s.ErrorCount < 0 {
		return errors.New("scan counts must not be negative")
	}
	if s.Status == "" {
		return errors.New("scan status is required")
	}
	return nil
}
