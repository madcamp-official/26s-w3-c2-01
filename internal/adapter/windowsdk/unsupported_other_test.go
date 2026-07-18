//go:build !windows

package windowsdk

import (
	"context"
	"errors"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/adapter"
)

func TestFilesystemDetectorReturnsUnsupportedPlatform(t *testing.T) {
	_, err := (FilesystemDetector{}).Detect(context.Background())
	if !errors.Is(err, adapter.ErrUnsupportedPlatform) {
		t.Fatalf("Detect() error = %v, want ErrUnsupportedPlatform", err)
	}
}
