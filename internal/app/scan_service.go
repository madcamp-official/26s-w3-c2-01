// [파일 역할] ScanService/ScanService.Run은 analysis_orchestrator.go의
// AnalysisOrchestrator.Run보다 먼저 만들어진 것으로 보이는 스캔 파이프라인이지만,
// grep으로 직접 확인한 결과("app.NewScanService" 검색 시 cmd 어디에서도 매치 없음)
// cmd/scan.go는 AnalysisOrchestrator만 생성해서 쓰고 NewScanService를 호출하는
// 프로덕션 코드는 이 파일 자신의 테스트 외에는 없다 — 사실상 죽은 코드다.
// 다만 이 파일에 정의된 ScanRecord / ScanRepository / ScanStatus* 상수는
// AnalysisOrchestrator가 그대로 재사용하므로(analysis_orchestrator.go의
// AnalysisOrchestrator.scans 필드) 살아있다. docs/libra_review_findings_day4.md에
// 기록된 이슈이며, 이 파일 자체 테스트가 여전히 ScanService.Run을 검증하고 있어
// 문서화만으로 남겨 두고 삭제하지 않았다.
package app

import (
	"context"
	"errors"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/scanner"
)

// ScanService.Run (below) predates AnalysisOrchestrator.Run
// (analysis_orchestrator.go) and appears to have been superseded by it:
// cmd/scan.go builds an AnalysisOrchestrator, not a ScanService, and
// grepping the whole repo finds no production caller of NewScanService --
// only this file's own tests. The ScanStatus*/ScanRecord/ScanRepository
// declarations here are still live, though: AnalysisOrchestrator embeds a
// ScanRepository field and reuses these same types. Flagged in
// docs/libra_review_findings_day4.md rather than deleted here, since
// removing a type this file's own tests still exercise isn't a
// documentation-only change.
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
