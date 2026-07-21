package app

import (
	"context"
	"fmt"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

type ExportReport struct {
	GeneratedAt  time.Time                   `json:"generated_at"`
	Scan         ScanRecord                  `json:"scan"`
	Projects     []domain.BuildProject       `json:"projects"`
	Resources    []domain.Resource           `json:"resources"`
	Issues       []Issue                     `json:"issues"`
	Transactions []domain.CleanupTransaction `json:"transactions"`
}

type ExportService struct {
	scans        ScanRepository
	projects     ProjectRepository
	resources    ResourceRepository
	issues       ScanIssueRepository
	transactions CleanupTransactionRepository
	now          func() time.Time
}

func NewExportService(scans ScanRepository, projects ProjectRepository, resources ResourceRepository, issues ScanIssueRepository, transactions CleanupTransactionRepository) *ExportService {
	return &ExportService{scans: scans, projects: projects, resources: resources, issues: issues, transactions: transactions, now: time.Now}
}

func (s *ExportService) Build(ctx context.Context) (ExportReport, error) {
	scan, err := s.scans.FindLatest(ctx)
	if err != nil {
		return ExportReport{}, fmt.Errorf("find latest scan: %w", err)
	}
	projects, err := s.projects.List(ctx)
	if err != nil {
		return ExportReport{}, fmt.Errorf("list projects: %w", err)
	}
	resources, err := s.resources.List(ctx)
	if err != nil {
		return ExportReport{}, fmt.Errorf("list resources: %w", err)
	}
	issues, err := s.issues.List(ctx, IssueFilter{ScanID: scan.ID})
	if err != nil {
		return ExportReport{}, fmt.Errorf("list issues: %w", err)
	}
	transactions, err := s.transactions.List(ctx)
	if err != nil {
		return ExportReport{}, fmt.Errorf("list transactions: %w", err)
	}
	return ExportReport{GeneratedAt: s.now().UTC(), Scan: scan, Projects: projects, Resources: resources, Issues: issues, Transactions: transactions}, nil
}
