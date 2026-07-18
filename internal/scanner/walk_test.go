package scanner

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestParallelScannerAggregatesSizeAndHonorsExcludes(t *testing.T) {
	root := t.TempDir()
	writeSizedFile(t, filepath.Join(root, "root.bin"), 3)
	writeSizedFile(t, filepath.Join(root, "src", "nested.bin"), 5)
	writeSizedFile(t, filepath.Join(root, "ignored", "large.bin"), 100)

	scanner := New(2)
	var visited = map[string]Entry{}
	result, err := scanner.Scan(context.Background(), Options{
		Roots:    []string{root},
		Exclude:  []string{"ignored"},
		MaxDepth: 20,
	}, func(_ context.Context, entry Entry) error {
		visited[entry.Path] = entry
		return nil
	})
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if result.RootsScanned != 1 || result.FilesInspected != 2 || result.LogicalSize != 8 {
		t.Fatalf("Scan() result = %#v", result)
	}
	if len(result.Issues) != 0 {
		t.Fatalf("Scan() issues = %v", result.Issues)
	}
	if _, found := visited[filepath.Join(root, "ignored")]; found {
		t.Fatal("excluded directory was visited")
	}
}

func TestParallelScannerDoesNotFollowSymlink(t *testing.T) {
	root := t.TempDir()
	target := t.TempDir()
	writeSizedFile(t, filepath.Join(target, "outside.bin"), 100)
	link := filepath.Join(root, "outside-link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("creating symlink is not permitted: %v", err)
	}

	scanner := New(2)
	result, err := scanner.Scan(context.Background(), Options{Roots: []string{root}, MaxDepth: 20}, nil)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if result.LogicalSize != 0 || result.FilesInspected != 0 {
		t.Fatalf("symlink target was counted: %#v", result)
	}
}

func TestParallelScannerCollectsPermissionErrorsAndContinues(t *testing.T) {
	root := t.TempDir()
	denied := filepath.Join(root, "denied")
	allowed := filepath.Join(root, "allowed")
	writeSizedFile(t, filepath.Join(denied, "hidden.bin"), 20)
	writeSizedFile(t, filepath.Join(allowed, "visible.bin"), 7)

	scanner := New(2)
	readDir := scanner.readDir
	scanner.readDir = func(path string) ([]os.DirEntry, error) {
		if canonical(path) == canonical(denied) {
			return nil, fs.ErrPermission
		}
		return readDir(path)
	}

	result, err := scanner.Scan(context.Background(), Options{Roots: []string{root}, MaxDepth: 20}, nil)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if result.LogicalSize != 7 || result.FilesInspected != 1 {
		t.Fatalf("Scan() result = %#v", result)
	}
	if len(result.Issues) != 1 || !errors.Is(result.Issues[0], fs.ErrPermission) {
		t.Fatalf("Scan() issues = %v", result.Issues)
	}
}

func TestParallelScannerUsesMultipleWorkers(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"one", "two", "three"} {
		writeSizedFile(t, filepath.Join(root, name, "file.bin"), 1)
	}

	scanner := New(3)
	readDir := scanner.readDir
	var active atomic.Int32
	var maximum atomic.Int32
	scanner.readDir = func(path string) ([]os.DirEntry, error) {
		if canonical(path) != canonical(root) {
			current := active.Add(1)
			defer active.Add(-1)
			for {
				previous := maximum.Load()
				if current <= previous || maximum.CompareAndSwap(previous, current) {
					break
				}
			}
			time.Sleep(25 * time.Millisecond)
		}
		return readDir(path)
	}

	_, err := scanner.Scan(context.Background(), Options{Roots: []string{root}, MaxDepth: 20}, nil)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if maximum.Load() < 2 {
		t.Fatalf("maximum concurrent workers = %d, want at least 2", maximum.Load())
	}
}

func TestParallelScannerStopsWhenContextIsCancelled(t *testing.T) {
	root := t.TempDir()
	writeSizedFile(t, filepath.Join(root, "file.bin"), 1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := New(1).Scan(ctx, Options{Roots: []string{root}, MaxDepth: 20}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Scan() error = %v, want context.Canceled", err)
	}
}

func writeSizedFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create parent: %v", err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
}
