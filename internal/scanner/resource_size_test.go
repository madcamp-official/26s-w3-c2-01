package scanner

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMeasureResourceAggregatesKnownPath(t *testing.T) {
	root := t.TempDir()
	firstPath := filepath.Join(root, "Include", "first.bin")
	secondPath := filepath.Join(root, "second.bin")
	if err := os.MkdirAll(filepath.Dir(firstPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(firstPath, []byte("12345"), 0o644); err != nil {
		t.Fatalf("WriteFile(first) error = %v", err)
	}
	if err := os.WriteFile(secondPath, []byte("1234567"), 0o644); err != nil {
		t.Fatalf("WriteFile(second) error = %v", err)
	}

	latest := time.Date(2030, 7, 18, 12, 0, 0, 0, time.Local)
	if err := os.Chtimes(secondPath, latest, latest); err != nil {
		t.Fatalf("Chtimes() error = %v", err)
	}

	got, err := MeasureResource(context.Background(), New(2), root)
	if err != nil {
		t.Fatalf("MeasureResource() error = %v", err)
	}
	if got.LogicalSize != 12 || got.FilesInspected != 2 {
		t.Fatalf("MeasureResource() = %#v, want 12 bytes across 2 files", got)
	}
	if !got.SizeKnown {
		t.Fatal("SizeKnown = false, want true for a complete measurement")
	}
	if got.LastModifiedAt == nil || !got.LastModifiedAt.Equal(latest) {
		t.Fatalf("LastModifiedAt = %v, want %v", got.LastModifiedAt, latest)
	}
	if len(got.Issues) != 0 {
		t.Fatalf("Issues = %v, want none", got.Issues)
	}
}

func TestMeasureResourceReportsMissingPathAsIssue(t *testing.T) {
	got, err := MeasureResource(context.Background(), New(1), filepath.Join(t.TempDir(), "missing"))
	if err != nil {
		t.Fatalf("MeasureResource() error = %v", err)
	}
	if len(got.Issues) != 1 {
		t.Fatalf("Issues = %v, want one missing-root issue", got.Issues)
	}
	if got.SizeKnown {
		t.Fatal("SizeKnown = true, want false when the resource path is missing")
	}
}
