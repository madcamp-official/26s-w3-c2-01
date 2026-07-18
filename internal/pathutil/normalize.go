// Package pathutil provides the single path identity contract shared by
// scanners, detectors, repositories, and CLI-facing application services.
package pathutil

import (
	"errors"
	"path/filepath"
	"strings"
)

var ErrEmptyPath = errors.New("path must not be empty")

// Normalize returns an absolute, cleaned path suitable for comparison and DB
// identity. It does not resolve symlinks or junctions.
func Normalize(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", ErrEmptyPath
	}
	absolute, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	return normalizePlatform(absolute), nil
}
