package dotnet

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

// sampleListSDKsOutput mirrors real `dotnet --list-sdks` output.
const sampleListSDKsOutput = `6.0.428 [C:\Program Files\dotnet\sdk]
8.0.404 [C:\Program Files\dotnet\sdk]
`

func TestCLISDKLister_ListSDKs(t *testing.T) {
	lister := CLISDKLister{
		// Point at a real file so the "is dotnet installed" existence check
		// passes without depending on a real .NET SDK install.
		DotnetPath: "dotnet_test.go",
		Run: func(ctx context.Context, path string, args ...string) ([]byte, error) {
			return []byte(sampleListSDKsOutput), nil
		},
	}

	got, err := lister.ListSDKs(context.Background())
	if err != nil {
		t.Fatalf("ListSDKs returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d resources, want 2: %+v", len(got), got)
	}
	if got[0].Version != "6.0.428" {
		t.Errorf("got[0].Version = %q, want %q", got[0].Version, "6.0.428")
	}
	wantPath, err := filepath.Abs(filepath.Join(`C:\Program Files\dotnet\sdk`, "8.0.404"))
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	if got[1].DisplayPath != wantPath {
		t.Errorf("got[1].DisplayPath = %q, want %q", got[1].DisplayPath, wantPath)
	}
	if got[1].ID == "" {
		t.Error("got[1].ID is empty, want a stable ResourceID")
	}
	wantID := domain.ResourceID(domain.ResourceTypeDotNetSDK, got[1].Version, got[1].NormalizedPath)
	if got[1].ID != wantID {
		t.Errorf("got[1].ID = %q, want %q", got[1].ID, wantID)
	}
}

func TestCLISDKLister_ListSDKs_DotnetNotInstalled(t *testing.T) {
	lister := CLISDKLister{
		DotnetPath: `Z:\does\not\exist\dotnet.exe`,
		Run: func(ctx context.Context, path string, args ...string) ([]byte, error) {
			t.Fatal("Run should not be called when dotnet is not installed")
			return nil, nil
		},
	}

	got, err := lister.ListSDKs(context.Background())
	if err != nil {
		t.Fatalf("ListSDKs returned error: %v, want nil (no dotnet installed is not an error)", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d resources, want 0: %+v", len(got), got)
	}
}
