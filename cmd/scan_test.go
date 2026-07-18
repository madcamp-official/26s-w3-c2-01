package cmd

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/store/sqlite"
)

func TestScanCommandDetectsAndPersistsNodeProjects(t *testing.T) {
	scanRoot = ""
	cfgPath = ""

	fixture, err := filepath.Abs("../testdata/node")
	if err != nil {
		t.Fatalf("resolve fixture path: %v", err)
	}

	dir := t.TempDir()
	t.Chdir(dir)

	out := &bytes.Buffer{}
	rootCmd.SetOut(out)
	rootCmd.SetErr(out)
	rootCmd.SetArgs([]string{"scan", "--root", fixture})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v; output=%s", err, out)
	}
	if !bytes.Contains(out.Bytes(), []byte("Projects found:  7")) {
		t.Fatalf("unexpected output:\n%s", out)
	}

	db, err := sqlite.Open(filepath.Join(dir, ".libra.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM projects WHERE project_type = 'node'").Scan(&count); err != nil {
		t.Fatalf("count projects: %v", err)
	}
	if count != 7 {
		t.Fatalf("persisted node projects = %d, want 7", count)
	}
}

func TestScanCommandRequiresRootsWithoutConfig(t *testing.T) {
	scanRoot = ""
	cfgPath = ""
	t.Chdir(t.TempDir())

	out := &bytes.Buffer{}
	rootCmd.SetOut(out)
	rootCmd.SetErr(out)
	rootCmd.SetArgs([]string{"scan"})
	if err := rootCmd.Execute(); err == nil {
		t.Fatal("Execute() error = nil, want an error about missing project roots")
	}
}
