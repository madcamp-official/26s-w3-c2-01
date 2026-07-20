package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/app"
)

// scan_repository.go는 internal/app에 정의된 app.ScanRepository
// 인터페이스를 SQLite로 구현한다. 다른 형제 repository 파일들과 달리
// 이 파일에는 var _ app.ScanRepository = (*ScanRepository)(nil) 같은
// 명시적 컴파일타임 assertion이 없지만, ScanRepository가 app.ScanRepository
// 시그니처를 그대로 구현하고 있음은 동일하다. scans 테이블에 스캔 1회
// 실행의 요약(시작/종료 시각, 스캔한 루트, 파일 수, 에러 수, 상태)을
// 저장(Save)하고 조회(Find)한다. Roots는 Windows 경로 구분자와 delimiter가
// 충돌하지 않도록 JSON으로 인코딩해서 저장한다. project_repository.go,
// resource_repository.go, dependency_repository.go, workspace_repository.go가
// scan_id로 이 테이블의 레코드를 참조하는 관계다.
var ErrScanNotFound = errors.New("scan not found")

type ScanRepository struct {
	db *sql.DB
}

func NewScanRepository(db *sql.DB) *ScanRepository {
	return &ScanRepository{db: db}
}

// Save inserts or replaces the mutable summary of a scan. Roots are encoded as
// JSON so Windows paths are preserved without delimiter ambiguity.
func (r *ScanRepository) Save(ctx context.Context, scan app.ScanRecord) error {
	if err := scan.Validate(); err != nil {
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

func (r *ScanRepository) Find(ctx context.Context, id string) (app.ScanRecord, error) {
	var record app.ScanRecord
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
		return app.ScanRecord{}, fmt.Errorf("%w: %s", ErrScanNotFound, id)
	}
	if err != nil {
		return app.ScanRecord{}, fmt.Errorf("find scan %q: %w", id, err)
	}

	record.StartedAt, err = time.Parse(time.RFC3339Nano, startedAt)
	if err != nil {
		return app.ScanRecord{}, fmt.Errorf("decode scan %q start time: %w", id, err)
	}
	if finishedAt.Valid {
		parsed, err := time.Parse(time.RFC3339Nano, finishedAt.String)
		if err != nil {
			return app.ScanRecord{}, fmt.Errorf("decode scan %q finish time: %w", id, err)
		}
		record.FinishedAt = &parsed
	}
	if err := json.Unmarshal([]byte(roots), &record.Roots); err != nil {
		return app.ScanRecord{}, fmt.Errorf("decode scan %q roots: %w", id, err)
	}
	return record, nil
}
