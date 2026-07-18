//go:build windows

package msbuild

import (
	"context"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

// sampleVSWhereOutput mirrors the shape of real `vswhere.exe -format json
// -utf8` output, trimmed to the fields VSWhereToolLocator reads.
const sampleVSWhereOutput = `[
  {
    "installationPath": "C:\\Program Files\\Microsoft Visual Studio\\2022\\Community",
    "installationVersion": "17.5.33627.172",
    "displayName": "Visual Studio Community 2022"
  },
  {
    "installationPath": "C:\\Program Files (x86)\\Microsoft Visual Studio\\2019\\Community",
    "installationVersion": "16.11.33529.622",
    "displayName": "Visual Studio Community 2019"
  }
]`

func TestVSWhereToolLocator_Locate(t *testing.T) {
	locator := VSWhereToolLocator{
		VSWherePath: "fake-vswhere.exe", // must exist on disk for os.Stat, see below
		Run: func(ctx context.Context, path string, args ...string) ([]byte, error) {
			return []byte(sampleVSWhereOutput), nil
		},
	}
	// os.Stat needs a real file to exist; point VSWherePath at this test file
	// itself so the "does vswhere.exe exist" check passes without touching a
	// real Visual Studio Installer.
	locator.VSWherePath = vswhereTestFile(t)

	got, err := locator.Locate(context.Background())
	if err != nil {
		t.Fatalf("Locate returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d resources, want 2: %+v", len(got), got)
	}

	for _, r := range got {
		if r.Type != domain.ResourceTypeVisualStudio {
			t.Errorf("Type = %v, want %v", r.Type, domain.ResourceTypeVisualStudio)
		}
	}
	if got[0].Version != "17.5.33627.172" {
		t.Errorf("got[0].Version = %q, want %q", got[0].Version, "17.5.33627.172")
	}
	if got[1].Name != "Visual Studio Community 2019" {
		t.Errorf("got[1].Name = %q, want %q", got[1].Name, "Visual Studio Community 2019")
	}
}

func TestVSWhereToolLocator_Locate_VSWhereNotInstalled(t *testing.T) {
	locator := VSWhereToolLocator{
		VSWherePath: "Z:\\does\\not\\exist\\vswhere.exe",
		Run: func(ctx context.Context, path string, args ...string) ([]byte, error) {
			t.Fatal("Run should not be called when vswhere.exe does not exist")
			return nil, nil
		},
	}

	got, err := locator.Locate(context.Background())
	if err != nil {
		t.Fatalf("Locate returned error: %v, want nil (no VS installed is not an error)", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d resources, want 0: %+v", len(got), got)
	}
}

// vswhereTestFile returns a path to a real file on disk (this test file
// itself), so VSWherePath's existence check passes in tests without
// depending on a real Visual Studio Installer.
func vswhereTestFile(t *testing.T) string {
	t.Helper()
	return "vswhere_test.go"
}
