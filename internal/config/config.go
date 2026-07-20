// Package config loads and validates Libra's YAML configuration.
package config

import (
	"errors"
	"fmt"
	"io"
	"os"

	"go.yaml.in/yaml/v3"
)

// config.go는 internal/config 패키지의 유일한 소스 파일로, 패키지가 다루는
// 모든 것 -- YAML 설정 스키마(Config/ScanConfig/CleanupConfig), 누락된
// 필드를 채우는 안전한 기본값(Default), 파일에서 읽어와 검증까지 마치는
// 로딩(Load), 그리고 저장(Save) -- 을 한 곳에 모아 둔다. 스키마 정의와
// 로드/검증 로직이 서로 강하게 얽혀 있어 별도 파일로 쪼갤 이유가 없었다.
// cmd/init.go는 Save로 초기 설정 파일을 만들고, cmd/db.go 등 다른
// 커맨드들은 Load를 통해 이 설정을 읽어 사용한다.
const CurrentVersion = 1

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
