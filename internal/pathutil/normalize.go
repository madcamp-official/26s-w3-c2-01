// Package pathutil provides the single path identity contract shared by
// scanners, detectors, repositories, and CLI-facing application services.
package pathutil

import (
	"errors"
	"path/filepath"
	"strings"

	"golang.org/x/text/unicode/norm"
)

var ErrEmptyPath = errors.New("path must not be empty")

// Normalize returns an absolute, cleaned path suitable for comparison and DB
// identity. It does not resolve symlinks or junctions.
//
// The result is also Unicode-normalized to NFC (issue #49): a path
// containing a decomposable character (Hangul syllables, Latin letters with
// combining diacritics like é/ü) can reach this function as either NFC
// (composed) or NFD (decomposed) byte sequences depending on how it
// entered -- a CLI argument or config file tends to stay NFC, while
// walking a real directory on macOS's APFS tends to yield NFD -- even
// though both render identically and refer to the same file. Every stable
// ID (§3 of docs/libra_integration_contracts.md) is computed from this
// value, so leaving the two byte forms distinct would let the same
// path/project/resource collide under two different identities depending
// on which entry path produced it. Absolute/DisplayPath deliberately keep
// the original bytes -- only the comparison/identity value normalizes.
func Normalize(path string) (string, error) {
	absolute, err := Absolute(path)
	if err != nil {
		return "", err
	}
	return normalizePlatform(norm.NFC.String(absolute)), nil
}

// Absolute returns a cleaned absolute path while preserving its display case.
// It does not resolve symlinks or junctions.
func Absolute(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", ErrEmptyPath
	}
	absolute, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	return filepath.Clean(absolute), nil
}

// Equal reports whether two paths have the same normalized identity.
func Equal(a, b string) (bool, error) {
	normalizedA, err := Normalize(a)
	if err != nil {
		return false, err
	}
	normalizedB, err := Normalize(b)
	if err != nil {
		return false, err
	}
	return normalizedA == normalizedB, nil
}

// IsSameOrChild reports whether path is parent itself or is contained below
// parent. It compares path components, not raw string prefixes.
func IsSameOrChild(path, parent string) (bool, error) {
	normalizedPath, err := Normalize(path)
	if err != nil {
		return false, err
	}
	normalizedParent, err := Normalize(parent)
	if err != nil {
		return false, err
	}
	// filepath.Rel returns an error on Windows when the paths are on
	// different volumes. Different volumes cannot have an ancestor/child
	// relationship, so treat that case as a normal negative result.
	if !strings.EqualFold(filepath.VolumeName(normalizedPath), filepath.VolumeName(normalizedParent)) {
		return false, nil
	}
	relative, err := filepath.Rel(normalizedParent, normalizedPath)
	if err != nil {
		return false, err
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))), nil
}
