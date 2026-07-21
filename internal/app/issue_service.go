package app

import (
	"context"
	"fmt"
	"strings"
)

// IssueFilter narrows persisted scan issues. ScanID is resolved by
// IssueService when omitted; repositories always receive an explicit scan.
type IssueFilter struct {
	ScanID   string
	Code     IssueCode
	Severity IssueSeverity
}

// ScanIssueRepository persists and queries the structured warnings and errors
// produced during a scan. Cause is intentionally not persisted because it is
// an in-process error chain rather than stable user-facing data.
type ScanIssueRepository interface {
	Replace(context.Context, string, []Issue) error
	List(context.Context, IssueFilter) ([]Issue, error)
}

type IssueService struct {
	issues ScanIssueRepository
	scans  ScanRepository
}

func NewIssueService(issues ScanIssueRepository, scans ScanRepository) *IssueService {
	return &IssueService{issues: issues, scans: scans}
}

func (s *IssueService) List(ctx context.Context, filter IssueFilter) (string, []Issue, error) {
	filter.ScanID = strings.TrimSpace(filter.ScanID)
	if filter.ScanID == "" {
		scan, err := s.scans.FindLatest(ctx)
		if err != nil {
			return "", nil, err
		}
		filter.ScanID = scan.ID
	} else if _, err := s.scans.Find(ctx, filter.ScanID); err != nil {
		return "", nil, err
	}

	issues, err := s.issues.List(ctx, filter)
	if err != nil {
		return "", nil, fmt.Errorf("list scan issues: %w", err)
	}
	return filter.ScanID, issues, nil
}
