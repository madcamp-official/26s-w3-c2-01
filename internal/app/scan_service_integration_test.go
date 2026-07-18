package app_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/app"
	"github.com/madcamp-official/26s-w3-c2-01/internal/scanner"
	sqlitestore "github.com/madcamp-official/26s-w3-c2-01/internal/store/sqlite"
)

func TestScanServicePersistsRealFilesystemResult(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "one.bin"), []byte("123"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	db, err := sqlitestore.Open(":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := sqlitestore.Migrate(db); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	repository := sqlitestore.NewScanRepository(db)
	service := app.NewScanService(scanner.New(2), repository)
	result, err := service.Run(context.Background(), "scan-integration", scanner.Options{
		Roots:    []string{root},
		MaxDepth: 20,
	}, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.FilesInspected != 1 || result.LogicalSize != 3 {
		t.Fatalf("Run() result = %#v", result)
	}

	record, err := repository.Find(context.Background(), "scan-integration")
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}
	if record.Status != app.ScanStatusCompleted || record.FileCount != 1 || record.ErrorCount != 0 {
		t.Fatalf("persisted record = %#v", record)
	}
}
