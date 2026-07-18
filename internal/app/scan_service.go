package app

import (
	"context"
	"errors"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/scanner"
)

const (
	ScanStatusRunning             = "RUNNING"
	ScanStatusCompleted           = "COMPLETED"
	ScanStatusCompletedWithErrors = "COMPLETED_WITH_ERRORS"
	ScanStatusFailed              = "FAILED"
)

type ScanRecord struct {
	ID         string
	StartedAt  time.Time
	FinishedAt *time.Time
	Roots      []string
	FileCount  int64
	ErrorCount int64
	Status     string
}

func (s ScanRecord) Validate() error {
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

type ScanRepository interface {
	Save(context.Context, ScanRecord) error
}

type ScanService struct {
	scanner    scanner.Scanner
	repository ScanRepository
	now        func() time.Time
}

func NewScanService(filesystem scanner.Scanner, repository ScanRepository) *ScanService {
	return &ScanService{scanner: filesystem, repository: repository, now: time.Now}
}

// Run scans the requested roots and persists both the running marker and final
// summary. Individual filesystem issues produce a completed-with-errors scan;
// terminal scanner or visitor errors produce a failed scan.
func (s *ScanService) Run(ctx context.Context, id string, options scanner.Options, visit scanner.Visitor) (scanner.Result, error) {
	record := ScanRecord{
		ID:        id,
		StartedAt: s.now(),
		Roots:     append([]string(nil), options.Roots...),
		Status:    ScanStatusRunning,
	}
	if err := s.repository.Save(ctx, record); err != nil {
		return scanner.Result{}, err
	}

	result, scanErr := s.scanner.Scan(ctx, options, visit)
	finishedAt := s.now()
	record.FinishedAt = &finishedAt
	record.FileCount = result.FilesInspected
	record.ErrorCount = int64(len(result.Issues))
	record.Status = ScanStatusCompleted
	if record.ErrorCount > 0 {
		record.Status = ScanStatusCompletedWithErrors
	}
	if scanErr != nil {
		record.Status = ScanStatusFailed
		record.ErrorCount++
	}

	if saveErr := s.repository.Save(context.WithoutCancel(ctx), record); saveErr != nil {
		return result, errors.Join(scanErr, saveErr)
	}
	return result, scanErr
}
