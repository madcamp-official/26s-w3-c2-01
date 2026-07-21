//go:build !windows

package safety

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSystemProtectedRootsBlocksMacOSSystemPathsNotUserLibrary(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS system root list is darwin-only; see roots_other.go")
	}
	classifier, err := NewSystemPathClassifier()
	if err != nil {
		t.Fatalf("NewSystemPathClassifier() error = %v", err)
	}

	got, err := classifier.Classify(filepath.Join("/Library", "Application Support", "something"))
	if err != nil {
		t.Fatalf("Classify(/Library/...) error = %v", err)
	}
	if !got.SystemManaged {
		t.Fatalf("Classify(/Library/...) = %#v, want system-managed", got)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	// The macOS dev-cache adapters (xcode/cocoapods/swiftpm/homebrew/
	// simulator) all report resources under ~/Library -- this must stay
	// REVIEW-eligible, not get relabeled BLOCKED by the system root list.
	got, err = classifier.Classify(filepath.Join(home, "Library", "Caches", "Homebrew"))
	if err != nil {
		t.Fatalf("Classify(~/Library/...) error = %v", err)
	}
	if got.SystemManaged {
		t.Fatalf("Classify(~/Library/...) = %#v, want NOT system-managed", got)
	}
}
