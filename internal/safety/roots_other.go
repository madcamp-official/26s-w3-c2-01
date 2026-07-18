//go:build !windows

package safety

func systemProtectedRoots() []string {
	return nil
}
