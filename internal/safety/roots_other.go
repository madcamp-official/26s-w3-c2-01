//go:build !windows

package safety

// systemProtectedRoots returns no protected paths on non-Windows: MVP scope
// (docs/libra_integration_contracts.md §20.3) only defines protection for
// Windows env vars (%WINDIR% etc, see roots_windows.go) since Windows is
// libra's primary target. This is a scope decision, not a bug -- macOS/
// Linux system paths simply aren't classified as SystemManaged yet.
func systemProtectedRoots() []string {
	return nil
}
