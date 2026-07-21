package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadUsesDefaultsAndReadsWindowsPaths(t *testing.T) {
	path := writeConfig(t, `
version: 1
project_roots:
  - 'C:\Users\user\source'
  - 'D:\Projects'
exclude:
  - 'C:\Windows'
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.ProjectRoots) != 2 || cfg.ProjectRoots[1] != `D:\Projects` {
		t.Fatalf("ProjectRoots = %#v", cfg.ProjectRoots)
	}
	if cfg.Scan.MaxDepth != 20 || cfg.Scan.StaleDays != 90 {
		t.Fatalf("Scan defaults = %#v", cfg.Scan)
	}
	if cfg.Cleanup.DefaultMode != "dry-run" || cfg.Cleanup.QuarantineDays != 7 {
		t.Fatalf("Cleanup defaults = %#v", cfg.Cleanup)
	}
}

func TestDefaultExcludesGeneratedAndVendoredDirectories(t *testing.T) {
	want := []string{"node_modules", ".next", "dist", "build", "bin", "obj", ".git", ".libra-quarantine",
		".venv", "venv", "__pycache__", ".pytest_cache", ".mypy_cache"}
	got := Default().Exclude
	if len(got) != len(want) {
		t.Fatalf("Default().Exclude = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Default().Exclude[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	got[0] = "mutated"
	if Default().Exclude[0] != "node_modules" {
		t.Fatal("Default().Exclude shares mutable backing storage")
	}
}

func TestLoadWithoutExcludeKeepsDefaultExcludes(t *testing.T) {
	path := writeConfig(t, "version: 1\n")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Exclude) != len(defaultExcludes) || cfg.Exclude[0] != "node_modules" {
		t.Fatalf("Exclude = %#v, want defaults", cfg.Exclude)
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	path := writeConfig(t, "version: 1\nunknown: true\n")

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "field unknown not found") {
		t.Fatalf("Load() error = %v, want unknown field error", err)
	}
}

func TestLoadRejectsUnsafeCleanupDefault(t *testing.T) {
	path := writeConfig(t, "version: 1\ncleanup:\n  default_mode: delete\n")

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "must be dry-run") {
		t.Fatalf("Load() error = %v, want dry-run validation error", err)
	}
}

func TestSaveWritesLoadableConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "libra.yaml")

	if err := Save(path, Default()); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Version != Default().Version || cfg.Scan != Default().Scan || cfg.Cleanup != Default().Cleanup {
		t.Fatalf("Load() after Save() = %#v, want %#v", cfg, Default())
	}
}

func writeConfig(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "libra.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
