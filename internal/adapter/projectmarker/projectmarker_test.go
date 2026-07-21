package projectmarker

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/scanner"
)

func TestDetectEcosystemProjects(t *testing.T) {
	cases := []struct {
		name, contents string
		want           domain.ProjectType
	}{
		{"pom.xml", "<project/>", domain.ProjectTypeMaven}, {"Cargo.toml", "[package]", domain.ProjectTypeCargo},
		{"go.mod", "module example.com/app", domain.ProjectTypeGo}, {"build.gradle.kts", "plugins { java }", domain.ProjectTypeGradle},
		{"build.gradle", "plugins { id 'com.android.application' }", domain.ProjectTypeAndroid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, tc.name)
			if err := os.WriteFile(path, []byte(tc.contents), 0o600); err != nil {
				t.Fatal(err)
			}
			got, ok, err := (Detector{}).Detect(context.Background(), scanner.Entry{Path: path, Kind: scanner.EntryFile, ModifiedAt: time.Now()})
			if err != nil || !ok || got.Type != tc.want || got.RootPath != root {
				t.Fatalf("Detect() = %#v, %v, %v", got, ok, err)
			}
		})
	}
}
