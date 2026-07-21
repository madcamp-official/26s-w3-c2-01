//go:build !windows

package dotnet

import (
	"context"
	"os/exec"
	"testing"
)

func TestCLISDKListerResolvesViaLookPathOnNonWindows(t *testing.T) {
	const sampleOutput = "8.0.404 [/usr/local/share/dotnet/sdk]\n"
	lister := CLISDKLister{
		LookPath: func(name string) (string, error) {
			if name != "dotnet" {
				t.Fatalf("LookPath called with %q, want %q", name, "dotnet")
			}
			return "/usr/local/share/dotnet/dotnet", nil
		},
		Run: func(ctx context.Context, path string, args ...string) ([]byte, error) {
			if path != "/usr/local/share/dotnet/dotnet" {
				t.Fatalf("run path = %q, want resolved LookPath result", path)
			}
			return []byte(sampleOutput), nil
		},
	}

	got, err := lister.ListSDKs(context.Background())
	if err != nil {
		t.Fatalf("ListSDKs() error = %v", err)
	}
	if len(got) != 1 || got[0].Version != "8.0.404" {
		t.Fatalf("got %#v, want one 8.0.404 resource", got)
	}
}

func TestCLISDKListerReturnsNothingWhenDotnetNotOnPath(t *testing.T) {
	lister := CLISDKLister{
		LookPath: func(string) (string, error) { return "", exec.ErrNotFound },
		Run: func(context.Context, string, ...string) ([]byte, error) {
			t.Fatal("Run should not be called when dotnet is not on PATH")
			return nil, nil
		},
	}

	got, err := lister.ListSDKs(context.Background())
	if err != nil {
		t.Fatalf("ListSDKs() error = %v, want nil (no dotnet installed is not an error)", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d resources, want 0: %+v", len(got), got)
	}
}
