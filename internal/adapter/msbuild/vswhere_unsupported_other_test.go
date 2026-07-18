//go:build !windows

package msbuild

import (
	"context"
	"errors"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/adapter"
)

func TestVSWhereToolLocatorReturnsUnsupportedPlatform(t *testing.T) {
	_, err := (VSWhereToolLocator{}).Locate(context.Background())
	if !errors.Is(err, adapter.ErrUnsupportedPlatform) {
		t.Fatalf("Locate() error = %v, want ErrUnsupportedPlatform", err)
	}
}
