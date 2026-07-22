// Package config loads and validates Libra's YAML configuration.
package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"go.yaml.in/yaml/v3"
)

const CurrentVersion = 1

// safetyExcludes are the entries in defaultExcludes that guard against real
// harm (resurfacing deleted data as if it were an active project), not just
// against misdetecting vendored/generated directories as projects. Unlike
// the rest of defaultExcludes, a user-supplied exclude list must never be
// able to drop these -- see EnsureSafetyExcludes, which Load and `config set
// exclude` both call because YAML/CLI exclude overrides replace the list
// wholesale rather than merging with it.
var safetyExcludes = []string{"$RECYCLE.BIN", "System Volume Information"}

var defaultExcludes = []string{
	"node_modules",
	".next",
	"dist",
	"build",
	"bin",
	"obj",
	".git",
	".libra-quarantine",
	// Python (docs/libra_integration_contracts.md §19.4): dist/build are
	// already covered above and shared with Node's mapping.
	".venv",
	"venv",
	"__pycache__",
	".pytest_cache",
	".mypy_cache",
	// macOS: CocoaPods' installed-pods directory and SwiftPM's build output
	// -- both exact directory names, unlike .xcodeproj/.xcworkspace (a
	// variable-named prefix + fixed suffix, which this exact-match exclude
	// list can't express; those are walked into, but their contents are
	// small IDE-only metadata, not a correctness concern).
	"Pods",
	".build",
	// Windows trash/system directories: not projects, and walking into
	// $RECYCLE.BIN in particular resurfaces deleted projects as if they were
	// still ACTIVE.
	"$RECYCLE.BIN",
	"System Volume Information",
	// Python: installed third-party packages under a venv/site install,
	// mirrors node_modules above -- without this, any dependency that ships
	// its own setup.py/pyproject.toml (e.g. numpy) is walked into and
	// misdetected as an authored top-level project.
	"site-packages",
	"dist-packages",
	// Unity's own package manager cache (Library/PackageCache/com.unity.*)
	// ships a package.json per package that isn't an npm manifest; excluding
	// the directory keeps the walker from ever reaching those files.
	"PackageCache",
}

// EnsureSafetyExcludes appends any safetyExcludes missing from excludes.
// Callers that accept a fully user-supplied exclude list (config.Load,
// `libra config set exclude`) must run it through here so that list can
// never drop Windows trash/system directory protection, even though it
// otherwise replaces defaultExcludes wholesale rather than merging with it.
func EnsureSafetyExcludes(excludes []string) []string {
	result := excludes
	for _, safe := range safetyExcludes {
		found := false
		for _, existing := range excludes {
			if strings.EqualFold(existing, safe) {
				found = true
				break
			}
		}
		if !found {
			result = append(result, safe)
		}
	}
	return result
}

// RemoveExclude returns excludes with name removed (case-insensitive). It
// refuses to remove a safetyExcludes entry -- those are enforced regardless
// of user configuration by EnsureSafetyExcludes, so silently dropping them
// here would just have EnsureSafetyExcludes add them back on the next Load
// with no explanation -- and errors if name isn't present, so a typo in
// `config set exclude <name> -d` doesn't silently no-op.
func RemoveExclude(excludes []string, name string) ([]string, error) {
	for _, safe := range safetyExcludes {
		if strings.EqualFold(name, safe) {
			return nil, fmt.Errorf("%q is a protected exclude and cannot be removed", safe)
		}
	}
	result := make([]string, 0, len(excludes))
	removed := false
	for _, existing := range excludes {
		if strings.EqualFold(existing, name) {
			removed = true
			continue
		}
		result = append(result, existing)
	}
	if !removed {
		return nil, fmt.Errorf("%q is not in the exclude list", name)
	}
	return result, nil
}

type Config struct {
	Version      int           `yaml:"version" json:"version"`
	ProjectRoots []string      `yaml:"project_roots" json:"project_roots"`
	Exclude      []string      `yaml:"exclude" json:"exclude"`
	Scan         ScanConfig    `yaml:"scan" json:"scan"`
	Cleanup      CleanupConfig `yaml:"cleanup" json:"cleanup"`
}

type ScanConfig struct {
	MaxDepth            int  `yaml:"max_depth" json:"max_depth"`
	FollowReparsePoints bool `yaml:"follow_reparse_points" json:"follow_reparse_points"`
	StaleDays           int  `yaml:"stale_days" json:"stale_days"`
}

type CleanupConfig struct {
	DefaultMode    string `yaml:"default_mode" json:"default_mode"`
	QuarantineDays int    `yaml:"quarantine_days" json:"quarantine_days"`
}

// Default returns the safe baseline configuration used for omitted fields.
func Default() Config {
	return Config{
		Version: CurrentVersion,
		Exclude: append([]string(nil), defaultExcludes...),
		Scan: ScanConfig{
			MaxDepth:  20,
			StaleDays: 90,
		},
		Cleanup: CleanupConfig{
			DefaultMode:    "dry-run",
			QuarantineDays: 7,
		},
	}
}

// Load reads a YAML file, rejects unknown fields, and validates its values.
func Load(path string) (Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("open config %q: %w", path, err)
	}
	defer file.Close()

	cfg := Default()
	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode config %q: %w", path, err)
	}
	cfg.Exclude = EnsureSafetyExcludes(cfg.Exclude)

	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return Config{}, fmt.Errorf("decode config %q: multiple YAML documents are not supported", path)
		}
		return Config{}, fmt.Errorf("decode config %q: %w", path, err)
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("validate config %q: %w", path, err)
	}
	return cfg, nil
}

// Save writes cfg to path as YAML, creating or truncating the file.
func Save(path string, cfg Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write config %q: %w", path, err)
	}
	return nil
}

func (c Config) Validate() error {
	if c.Version != CurrentVersion {
		return fmt.Errorf("version must be %d", CurrentVersion)
	}
	if c.Scan.MaxDepth <= 0 {
		return errors.New("scan.max_depth must be greater than zero")
	}
	if c.Scan.StaleDays <= 0 {
		return errors.New("scan.stale_days must be greater than zero")
	}
	if c.Cleanup.DefaultMode != "dry-run" {
		return errors.New("cleanup.default_mode must be dry-run")
	}
	if c.Cleanup.QuarantineDays <= 0 {
		return errors.New("cleanup.quarantine_days must be greater than zero")
	}
	return nil
}
