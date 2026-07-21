// Package config loads and validates Libra's YAML configuration.
package config

import (
	"errors"
	"fmt"
	"io"
	"os"

	"go.yaml.in/yaml/v3"
)

const CurrentVersion = 1

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
}

type Config struct {
	Version      int           `yaml:"version"`
	ProjectRoots []string      `yaml:"project_roots"`
	Exclude      []string      `yaml:"exclude"`
	Scan         ScanConfig    `yaml:"scan"`
	Cleanup      CleanupConfig `yaml:"cleanup"`
}

type ScanConfig struct {
	MaxDepth            int  `yaml:"max_depth"`
	FollowReparsePoints bool `yaml:"follow_reparse_points"`
	StaleDays           int  `yaml:"stale_days"`
}

type CleanupConfig struct {
	DefaultMode    string `yaml:"default_mode"`
	QuarantineDays int    `yaml:"quarantine_days"`
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
