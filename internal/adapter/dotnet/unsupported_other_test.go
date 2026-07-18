//go:build !windows

package dotnet

import (
	"context"
	"errors"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/adapter"
)

func TestCLISDKListerReturnsUnsupportedPlatform(t *testing.T) {
	_, err := (CLISDKLister{}).ListSDKs(context.Background())
	if !errors.Is(err, adapter.ErrUnsupportedPlatform) {
		t.Fatalf("ListSDKs() error = %v, want ErrUnsupportedPlatform", err)
	}
}
