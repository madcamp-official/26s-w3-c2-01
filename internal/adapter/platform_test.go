package adapter

import (
	"errors"
	"runtime"
	"testing"
)

func TestRequireWindowsMatchesCurrentPlatform(t *testing.T) {
	err := RequireWindows("test feature")
	if runtime.GOOS == "windows" && err != nil {
		t.Fatalf("RequireWindows() error = %v on Windows", err)
	}
	if runtime.GOOS != "windows" && !errors.Is(err, ErrUnsupportedPlatform) {
		t.Fatalf("RequireWindows() error = %v, want ErrUnsupportedPlatform", err)
	}
}
