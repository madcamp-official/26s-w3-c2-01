//go:build !windows

package safety

import "runtime"

// systemProtectedRoots returns macOS's fixed system-managed roots (the
// sealed system volume and /Applications -- the same "where system software
// lives" role ProgramFiles/WINDIR play on Windows, see roots_windows.go).
// These are plain path prefixes, not SIP-awareness: SIP already blocks
// writes to /System and most of /Library even for root, so this is a
// backstop for the classification/labeling path (BLOCKED vs REVIEW), not
// the only thing standing between libra and deleting them. ~/Library is
// deliberately absent -- that's exactly where the macOS dev-cache adapters
// (xcode/cocoapods/swiftpm/homebrew/simulator) report their resources, and
// blocking it here would relabel all of them BLOCKED instead of REVIEW.
//
// Linux system paths are left unclassified (returns nil): unlike the macOS
// list above, no equivalent has been verified against a real Linux install
// yet, and guessing wrong would be worse than the current REVIEW fallback.
func systemProtectedRoots() []string {
	if runtime.GOOS != "darwin" {
		return nil
	}
	return []string{
		"/System",
		"/Library",
		"/usr",
		"/bin",
		"/sbin",
		"/Applications",
	}
}
