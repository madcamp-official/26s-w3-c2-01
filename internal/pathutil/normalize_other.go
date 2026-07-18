//go:build !windows

package pathutil

import "path/filepath"

func normalizePlatform(path string) string {
	return filepath.Clean(path)
}
