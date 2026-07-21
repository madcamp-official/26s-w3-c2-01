package cmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/config"
)

func TestSnapshotRootsDetectsChangesAndHonorsExclude(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "project.txt"), []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "node_modules"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "node_modules", "ignored"), []byte("ignored"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.ProjectRoots = []string{root}
	before, err := snapshotRoots(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "added.txt"), []byte("bb"), 0o600); err != nil {
		t.Fatal(err)
	}
	after, err := snapshotRoots(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if before == after || after.Files != before.Files+1 {
		t.Fatalf("before/after = %#v/%#v", before, after)
	}
}

func TestDaemonStateFresh(t *testing.T) {
	now := time.Now()
	if !daemonStateFresh(daemonState{PID: 1, Heartbeat: now.Add(-daemonPollInterval)}, now) {
		t.Fatal("recent heartbeat should be fresh")
	}
	if daemonStateFresh(daemonState{PID: 1, Heartbeat: now.Add(-daemonStaleAfter - time.Second)}, now) {
		t.Fatal("old heartbeat should be stale")
	}
}
