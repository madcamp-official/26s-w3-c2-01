package pathutil

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestNormalizeReturnsAbsoluteCleanPath(t *testing.T) {
	got, err := Normalize(filepath.Join("relative", "child", "..", "file"))
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("Normalize() = %q, want absolute path", got)
	}
	if filepath.Base(got) != "file" {
		t.Fatalf("Normalize() = %q, want cleaned path", got)
	}
}

func TestNormalizeRejectsEmptyPath(t *testing.T) {
	_, err := Normalize("  ")
	if !errors.Is(err, ErrEmptyPath) {
		t.Fatalf("Normalize() error = %v, want ErrEmptyPath", err)
	}
}

func TestAbsolutePreservesDisplayCase(t *testing.T) {
	got, err := Absolute(filepath.Join("Relative", "MixedCase"))
	if err != nil {
		t.Fatalf("Absolute() error = %v", err)
	}
	if filepath.Base(got) != "MixedCase" {
		t.Fatalf("Absolute() = %q, want display case preserved", got)
	}
}
