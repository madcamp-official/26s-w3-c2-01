package msbuild

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestProjectRoot(t *testing.T) {
	cases := []struct {
		name     string
		path     string
		wantName string
	}{
		{"cpp project", "../../../testdata/msbuild/GameClient/GameClient.vcxproj", "GameClient"},
		{"dotnet project", "../../../testdata/msbuild/SampleDotNetApp/SampleDotNetApp.csproj", "SampleDotNetApp"},
		{"solution file", "../../../testdata/msbuild/GameClient/GameClient.sln", "GameClient"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wantRoot, err := filepath.Abs(filepath.Dir(tc.path))
			if err != nil {
				t.Fatalf("filepath.Abs: %v", err)
			}

			root, name, drive, err := ProjectRoot(tc.path)
			if err != nil {
				t.Fatalf("ProjectRoot returned error: %v", err)
			}

			if root != wantRoot {
				t.Errorf("root = %q, want %q", root, wantRoot)
			}
			if name != tc.wantName {
				t.Errorf("name = %q, want %q", name, tc.wantName)
			}
			// filepath.VolumeName only returns a non-empty value on Windows;
			// POSIX paths (macOS/Linux) have no drive-letter concept.
			if runtime.GOOS == "windows" {
				if drive == "" {
					t.Errorf("drive is empty for %q", tc.path)
				}
			} else if drive != "" {
				t.Errorf("drive = %q, want empty on %s (no drive-letter concept)", drive, runtime.GOOS)
			}
		})
	}
}
