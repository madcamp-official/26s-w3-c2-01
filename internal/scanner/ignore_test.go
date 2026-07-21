package scanner

import (
	"path/filepath"
	"testing"
)

func TestExcludeMatcherSupportsRootRelativeAndAbsolutePaths(t *testing.T) {
	root := t.TempDir()
	absolute := filepath.Join(root, "private")
	matcher, err := newExcludeMatcher(
		[]string{root},
		[]string{filepath.Join(".git", "objects"), absolute},
	)
	if err != nil {
		t.Fatalf("newExcludeMatcher() error = %v", err)
	}

	tests := []struct {
		path string
		want bool
	}{
		{path: filepath.Join(root, ".git", "objects"), want: true},
		{path: filepath.Join(root, ".git", "objects", "pack", "data"), want: true},
		{path: filepath.Join(root, "private", "secret"), want: true},
		{path: filepath.Join(root, ".git", "config"), want: false},
		{path: filepath.Join(root, "src"), want: false},
	}

	for _, tt := range tests {
		if got := matcher.Matches(tt.path); got != tt.want {
			t.Errorf("Matches(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestExcludeMatcherAppliesSingleDirectoryNamesAtAnyDepth(t *testing.T) {
	root := t.TempDir()
	matcher, err := newExcludeMatcher([]string{root}, []string{"node_modules", "dist"})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		filepath.Join(root, "node_modules"),
		filepath.Join(root, "frontend", "node_modules", "react"),
		filepath.Join(root, "packages", "app", "dist", "bundle.js"),
	} {
		if !matcher.Matches(path) {
			t.Errorf("Matches(%q) = false, want true", path)
		}
	}
	if matcher.Matches(filepath.Join(root, "frontend", "node_modules-cache")) {
		t.Fatal("segment prefix was excluded")
	}
}
