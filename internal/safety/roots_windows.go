//go:build windows

package safety

import "os"

func systemProtectedRoots() []string {
	names := []string{"WINDIR", "ProgramFiles", "ProgramFiles(x86)", "ProgramData"}
	roots := make([]string, 0, len(names))
	for _, name := range names {
		if root := os.Getenv(name); root != "" {
			roots = append(roots, root)
		}
	}
	return roots
}
