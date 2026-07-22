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
		".venv", "venv", "__pycache__", ".pytest_cache", ".mypy_cache", "Pods", ".build",
		"$RECYCLE.BIN", "System Volume Information", "site-packages", "dist-packages", "PackageCache"}
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

func TestLoadCustomExcludeStillProtectsSafetyDirectories(t *testing.T) {
	path := writeConfig(t, `
version: 1
exclude:
  - 'C:\Windows'
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"C:\\Windows", "$RECYCLE.BIN", "System Volume Information"}
	if len(cfg.Exclude) != len(want) {
		t.Fatalf("Exclude = %#v, want %#v", cfg.Exclude, want)
	}
	for i := range want {
		if cfg.Exclude[i] != want[i] {
			t.Fatalf("Exclude = %#v, want %#v", cfg.Exclude, want)
		}
	}
}

func TestEnsureSafetyExcludesIsCaseInsensitiveAndIdempotent(t *testing.T) {
	got := EnsureSafetyExcludes([]string{"node_modules", "$recycle.bin"})
	want := []string{"node_modules", "$recycle.bin", "System Volume Information"}
	if len(got) != len(want) {
		t.Fatalf("EnsureSafetyExcludes() = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("EnsureSafetyExcludes() = %#v, want %#v", got, want)
		}
	}
}

func TestRemoveExcludeRemovesCaseInsensitively(t *testing.T) {
	got, err := RemoveExclude([]string{"node_modules", "my-temp-dir"}, "MY-TEMP-DIR")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"node_modules"}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("RemoveExclude() = %#v, want %#v", got, want)
	}
}

func TestRemoveExcludeRejectsSafetyExclude(t *testing.T) {
	_, err := RemoveExclude([]string{"node_modules", "$RECYCLE.BIN"}, "$recycle.bin")
	if err == nil {
		t.Fatal("expected an error removing a protected exclude")
	}
}

func TestRemoveExcludeRejectsAbsentEntry(t *testing.T) {
	_, err := RemoveExclude([]string{"node_modules"}, "not-there")
	if err == nil {
		t.Fatal("expected an error removing an absent entry")
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
