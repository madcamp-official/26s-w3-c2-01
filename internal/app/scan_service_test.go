package app

import (
	"context"
	"errors"
	"io/fs"
	"testing"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/scanner"
)

type stubScanner struct {
	result scanner.Result
	err    error
}

func (s stubScanner) Scan(context.Context, scanner.Options, scanner.Visitor) (scanner.Result, error) {
	return s.result, s.err
}

type recordingScanRepository struct {
	records []ScanRecord
	err     error
}

func (r *recordingScanRepository) Save(_ context.Context, record ScanRecord) error {
	r.records = append(r.records, record)
	return r.err
}

func TestScanServicePersistsCompletedWithErrorsResult(t *testing.T) {
	repository := &recordingScanRepository{}
	service := NewScanService(stubScanner{result: scanner.Result{
		FilesInspected: 12,
		Issues:         []scanner.Issue{{Path: "denied", Operation: "read", Err: fs.ErrPermission}},
	}}, repository)
	times := []time.Time{time.Unix(100, 0), time.Unix(200, 0)}
	service.now = func() time.Time {
		now := times[0]
		times = times[1:]
		return now
	}

	_, err := service.Run(context.Background(), "scan-1", scanner.Options{Roots: []string{"root"}, MaxDepth: 20}, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(repository.records) != 2 {
		t.Fatalf("saved records = %d, want 2", len(repository.records))
	}
	final := repository.records[1]
	if final.Status != ScanStatusCompletedWithErrors || final.FileCount != 12 || final.ErrorCount != 1 {
		t.Fatalf("final record = %#v", final)
	}
	if final.FinishedAt == nil || !final.FinishedAt.Equal(time.Unix(200, 0)) {
		t.Fatalf("finished at = %v", final.FinishedAt)
	}
}

func TestScanServicePersistsFailedResult(t *testing.T) {
	scanErr := errors.New("visitor failed")
	repository := &recordingScanRepository{}
	service := NewScanService(stubScanner{err: scanErr}, repository)

	_, err := service.Run(context.Background(), "scan-1", scanner.Options{Roots: []string{"root"}, MaxDepth: 20}, nil)
	if !errors.Is(err, scanErr) {
		t.Fatalf("Run() error = %v, want scan error", err)
	}
	if got := repository.records[1]; got.Status != ScanStatusFailed || got.ErrorCount != 1 {
		t.Fatalf("final record = %#v", got)
	}
}

func TestScanServiceDoesNotScanWhenInitialSaveFails(t *testing.T) {
	saveErr := errors.New("database unavailable")
	repository := &recordingScanRepository{err: saveErr}
	service := NewScanService(stubScanner{}, repository)

	_, err := service.Run(context.Background(), "scan-1", scanner.Options{Roots: []string{"root"}, MaxDepth: 20}, nil)
	if !errors.Is(err, saveErr) {
		t.Fatalf("Run() error = %v, want save error", err)
	}
	if len(repository.records) != 1 {
		t.Fatalf("saved records = %d, want 1", len(repository.records))
	}
}
