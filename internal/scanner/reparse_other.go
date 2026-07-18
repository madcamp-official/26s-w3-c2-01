//go:build !windows

package scanner

import "io/fs"

func isReparsePoint(fs.FileInfo) bool {
	return false
}
