//go:build windows

package windowsdk

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

func TestFilesystemDetector_Detect(t *testing.T) {
	root := t.TempDir()

	includeDir := filepath.Join(root, "10", "Include")
	for _, version := range []string{"10.0.19041.0", "10.0.22000.0"} {
		if err := os.MkdirAll(filepath.Join(includeDir, version), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "8.1", "References"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	netfxDir := filepath.Join(root, "NETFXSDK")
	if err := os.MkdirAll(filepath.Join(netfxDir, "4.6.1", "Include"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	detector := FilesystemDetector{KitsRoot: root}

	got, err := detector.Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect returned error: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("got %d resources, want 4: %+v", len(got), got)
	}

	byVersion := map[string]domain.Resource{}
	for _, r := range got {
		byVersion[r.Version] = r
	}

	for _, version := range []string{"10.0.19041.0", "10.0.22000.0", "8.1"} {
		r, ok := byVersion[version]
		if !ok {
			t.Errorf("missing resource for version %q", version)
			continue
		}
		if r.Type != domain.ResourceTypeWindowsSDK {
			t.Errorf("version %q: Type = %v, want %v", version, r.Type, domain.ResourceTypeWindowsSDK)
		}
	}

	netfx, ok := byVersion["4.6.1"]
	if !ok {
		t.Errorf("missing resource for version %q", "4.6.1")
	} else if netfx.Type != domain.ResourceTypeNetFXSDK {
		t.Errorf("version %q: Type = %v, want %v", "4.6.1", netfx.Type, domain.ResourceTypeNetFXSDK)
	}
}

func TestFilesystemDetector_Detect_NoSDKInstalled(t *testing.T) {
	root := t.TempDir() // none of 10\Include, 8.1, or NETFXSDK exist under here

	detector := FilesystemDetector{KitsRoot: root}

	got, err := detector.Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect returned error: %v, want nil (no SDK installed is not an error)", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d resources, want 0: %+v", len(got), got)
	}
}
