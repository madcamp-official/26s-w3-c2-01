package ecosystem

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

func TestFilesystemEcosystemListers(t *testing.T) {
	home := t.TempDir()
	for _, path := range []string{"Android/Sdk", ".gradle/caches", ".cargo/registry", ".cargo/git", ".m2/custom-repo"} {
		if err := os.MkdirAll(filepath.Join(home, filepath.FromSlash(path)), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(home, ".m2", "settings.xml"), []byte(`<settings><localRepository>${user.home}/.m2/custom-repo</localRepository></settings>`), 0o600); err != nil {
		t.Fatal(err)
	}
	paths := Paths{Home: home, Env: func(string) string { return "" }}
	android, err := (AndroidGradleLister{Paths: paths, GOOS: "linux"}).ListResources(context.Background())
	if err != nil || len(android) != 2 || android[0].Type != domain.ResourceTypeAndroidSDK {
		t.Fatalf("Android/Gradle = %#v, %v", android, err)
	}
	cargo, err := (CargoLister{Paths: paths}).ListResources(context.Background())
	if err != nil || len(cargo) != 2 {
		t.Fatalf("Cargo = %#v, %v", cargo, err)
	}
	maven, err := (MavenLister{Paths: paths}).ListResources(context.Background())
	if err != nil || len(maven) != 1 || maven[0].Version != "maven" {
		t.Fatalf("Maven = %#v, %v", maven, err)
	}
}

func TestNodeCacheListerUsesOfficialCLIPaths(t *testing.T) {
	npmCache, pnpmStore := filepath.Join(t.TempDir(), "npm"), filepath.Join(t.TempDir(), "pnpm")
	_ = os.Mkdir(npmCache, 0o700)
	_ = os.Mkdir(pnpmStore, 0o700)
	run := func(_ context.Context, path string, _ ...string) ([]byte, error) {
		if path == "npm" {
			return []byte(npmCache + "\n"), nil
		}
		return []byte(pnpmStore + "\n"), nil
	}
	for _, tool := range []string{"npm", "pnpm"} {
		got, err := (NodeCacheLister{Tool: tool, LookPath: func(name string) (string, error) { return name, nil }, Run: run}).ListResources(context.Background())
		if err != nil || len(got) != 1 || got[0].Version != tool {
			t.Fatalf("%s ListResources = %#v, %v", tool, got, err)
		}
	}
}
