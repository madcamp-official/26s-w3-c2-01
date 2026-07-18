package scanner

import (
	"path/filepath"
	"runtime"
	"strings"

	"github.com/madcamp-official/26s-w3-c2-01/internal/pathutil"
)

type excludeMatcher struct {
	roots    []string
	absolute []string
	relative []string
}

func newExcludeMatcher(roots, excludes []string) (excludeMatcher, error) {
	matcher := excludeMatcher{roots: make([]string, 0, len(roots))}
	for _, root := range roots {
		normalized, err := pathutil.Normalize(root)
		if err != nil {
			return excludeMatcher{}, err
		}
		matcher.roots = append(matcher.roots, normalized)
	}

	for _, exclude := range excludes {
		if strings.TrimSpace(exclude) == "" {
			continue
		}
		if filepath.IsAbs(exclude) {
			normalized, err := pathutil.Normalize(exclude)
			if err != nil {
				return excludeMatcher{}, err
			}
			matcher.absolute = append(matcher.absolute, normalized)
			continue
		}
		matcher.relative = append(matcher.relative, canonical(filepath.Clean(exclude)))
	}

	return matcher, nil
}

func (m excludeMatcher) Matches(path string) bool {
	normalized, err := pathutil.Normalize(path)
	if err != nil {
		return false
	}
	for _, excluded := range m.absolute {
		if sameOrChild(normalized, excluded) {
			return true
		}
	}
	for _, root := range m.roots {
		relative, err := filepath.Rel(root, normalized)
		if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			continue
		}
		relative = canonical(relative)
		for _, excluded := range m.relative {
			if sameOrChild(relative, excluded) {
				return true
			}
		}
	}
	return false
}

func canonical(path string) string {
	path = filepath.Clean(path)
	if runtime.GOOS == "windows" {
		return strings.ToLower(path)
	}
	return path
}

func sameOrChild(path, parent string) bool {
	return path == parent || strings.HasPrefix(path, parent+string(filepath.Separator))
}
