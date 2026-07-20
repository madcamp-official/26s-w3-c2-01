package app

import (
	"context"
	"errors"
	"time"
)

// Scan status values and ScanRecord form the persistence contract shared by
// AnalysisOrchestrator and the SQLite scan repository.
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
