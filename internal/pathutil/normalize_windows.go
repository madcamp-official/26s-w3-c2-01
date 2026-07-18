//go:build windows

package pathutil

import (
	"path/filepath"
	"strings"
)

func normalizePlatform(path string) string {
	return strings.ToLower(filepath.Clean(path))
}
