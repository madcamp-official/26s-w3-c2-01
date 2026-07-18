// Package adapter contains contracts shared by platform-specific adapters.
package adapter

import (
	"errors"
	"fmt"
	"runtime"
)

var ErrUnsupportedPlatform = errors.New("unsupported platform")

// RequireWindows returns a descriptive error instead of making a Windows-only
// adapter look like a successful empty detection on another platform.
func RequireWindows(feature string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	return fmt.Errorf("%w: %s requires Windows (current: %s)", ErrUnsupportedPlatform, feature, runtime.GOOS)
}
