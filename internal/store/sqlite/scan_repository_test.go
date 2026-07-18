package sqlite

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestScanRepositorySavesAndUpdatesResult(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	repository := NewScanRepository(db)
	startedAt := time.Date(2026, 7, 18, 1, 2, 3, 4, time.FixedZone("KST", 9*60*60))
	finishedAt := startedAt.Add(time.Minute)
	record := ScanRecord{
		ID:         "scan-001",
		StartedAt:  startedAt,
		Roots:      []string{`C:\Users\user\source`, `D:\Projects`},
		Status:     "RUNNING",
		FileCount:  0,
		ErrorCount: 0,
	}
	if err := repository.Save(context.Background(), record); err != nil {
		t.Fatalf("Save(running) error = %v", err)
	}

	record.FinishedAt = &finishedAt
	record.Status = "COMPLETED_WITH_ERRORS"
	record.FileCount = 42
	record.ErrorCount = 2
	if err := repository.Save(context.Background(), record); err != nil {
		t.Fatalf("Save(completed) error = %v", err)
	}

	got, err := repository.Find(context.Background(), record.ID)
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}
	if got.ID != record.ID || got.Status != record.Status || got.FileCount != 42 || got.ErrorCount != 2 {
		t.Fatalf("Find() = %#v", got)
	}
	if !got.StartedAt.Equal(startedAt) || got.FinishedAt == nil || !got.FinishedAt.Equal(finishedAt) {
		t.Fatalf("Find() times = %v, %v", got.StartedAt, got.FinishedAt)
	}
	if !reflect.DeepEqual(got.Roots, record.Roots) {
		t.Fatalf("Find() roots = %#v, want %#v", got.Roots, record.Roots)
	}
}

func TestScanRepositoryRejectsInvalidRecord(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })

	err = NewScanRepository(db).Save(context.Background(), ScanRecord{})
	if err == nil {
		t.Fatal("Save() error = nil, want validation error")
	}
}

func TestScanRepositoryReturnsNotFound(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	_, err = NewScanRepository(db).Find(context.Background(), "missing")
	if !errors.Is(err, ErrScanNotFound) {
		t.Fatalf("Find() error = %v, want ErrScanNotFound", err)
	}
}
