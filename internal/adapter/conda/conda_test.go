package conda

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCLIEnvLister_NotInstalled_ReturnsEmptyNotError(t *testing.T) {
	lister := CLIEnvLister{
		LookPath: func(string) (string, error) { return "", errors.New("not found") },
	}
	resources, err := lister.ListEnvs(context.Background())
	if err != nil {
		t.Fatalf("expected no error when conda is not installed, got %v", err)
	}
	if resources != nil {
		t.Errorf("expected nil resources, got %v", resources)
	}
}

func TestCLIEnvLister_ParsesEnvList(t *testing.T) {
	lister := CLIEnvLister{
		CondaPath: "/opt/conda/bin/conda",
		Run: func(ctx context.Context, path string, args ...string) ([]byte, error) {
			return []byte(`{"envs": ["/opt/conda", "/opt/conda/envs/myproject", "/opt/conda/envs/base"]}`), nil
		},
	}
	resources, err := lister.ListEnvs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(resources) != 3 {
		t.Fatalf("got %d resources, want 3", len(resources))
	}
	if resources[0].Name != "base" {
		t.Errorf("root env Name = %q, want base", resources[0].Name)
	}
	if resources[1].Name != "myproject" {
		t.Errorf("named env Name = %q, want myproject", resources[1].Name)
	}
	for _, r := range resources {
		if r.Type != domain.ResourceTypeCondaEnv {
			t.Errorf("Type = %q, want %q", r.Type, domain.ResourceTypeCondaEnv)
		}
	}
}

func TestCLIEnvLister_RunError(t *testing.T) {
	lister := CLIEnvLister{
		CondaPath: "/opt/conda/bin/conda",
		Run: func(ctx context.Context, path string, args ...string) ([]byte, error) {
			return nil, errors.New("conda: command failed")
		},
	}
	if _, err := lister.ListEnvs(context.Background()); err == nil {
		t.Error("expected an error when conda is installed but fails to run")
	}
}

func TestIsGenericEnvName(t *testing.T) {
	cases := map[string]bool{
		"base": true, "env": true, "root": true, "py39": true, "python311": true,
		"myproject": false, "libra-scan": false,
	}
	for name, want := range cases {
		if got := IsGenericEnvName(name); got != want {
			t.Errorf("IsGenericEnvName(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestDeclaredEnvironmentName(t *testing.T) {
	t.Run("environment.yml present", func(t *testing.T) {
		root := t.TempDir()
		write(t, root, EnvironmentFile, "name: myproject\ndependencies:\n  - python=3.11\n")
		name, sourcePath, ok, err := DeclaredEnvironmentName(root)
		if err != nil {
			t.Fatal(err)
		}
		if !ok || name != "myproject" {
			t.Errorf("got (%q, %v), want (myproject, true)", name, ok)
		}
		if filepath.Base(sourcePath) != EnvironmentFile {
			t.Errorf("sourcePath = %q, want to end in %q", sourcePath, EnvironmentFile)
		}
	})

	t.Run("no environment file", func(t *testing.T) {
		root := t.TempDir()
		_, _, ok, err := DeclaredEnvironmentName(root)
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			t.Error("expected ok=false when no environment.yml exists")
		}
	})
}

func TestDetectLocalPrefixEnvs(t *testing.T) {
	root := t.TempDir()
	write(t, root, filepath.Join("envs", "conda-meta", "history"), "")
	if err := os.MkdirAll(filepath.Join(root, "not-an-env"), 0o755); err != nil {
		t.Fatal(err)
	}

	resources, err := DetectLocalPrefixEnvs(root, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(resources) != 1 {
		t.Fatalf("got %d resources, want 1 (envs only, not-an-env excluded)", len(resources))
	}
	r := resources[0]
	if r.Name != "envs" || r.Type != domain.ResourceTypeCondaEnv {
		t.Errorf("resource = %+v, want Name=envs Type=%q", r, domain.ResourceTypeCondaEnv)
	}
	if !r.Regenerable || r.RegenerationCommand == "" {
		t.Errorf("resource = %+v, want Regenerable=true with a RegenerationCommand (hasEnvironmentFile=true)", r)
	}
}

func TestDetectLocalPrefixEnvs_NoEnvironmentFile_NotRegenerable(t *testing.T) {
	root := t.TempDir()
	write(t, root, filepath.Join("envs", "conda-meta", "history"), "")

	resources, err := DetectLocalPrefixEnvs(root, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(resources) != 1 || resources[0].Regenerable {
		t.Errorf("resources = %+v, want one non-regenerable entry", resources)
	}
}
