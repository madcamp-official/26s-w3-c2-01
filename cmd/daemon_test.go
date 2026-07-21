package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/config"
	"github.com/madcamp-official/26s-w3-c2-01/internal/output"
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
	if len(after.Files) != len(before.Files)+1 {
		t.Fatalf("before/after = %#v/%#v", before, after)
	}
}

func TestDaemonStatusJSONUsesSharedEnvelope(t *testing.T) {
	dir := t.TempDir()
	cfgPath = filepath.Join(dir, ".libra.yaml")
	jsonOutput = true
	t.Cleanup(func() { cfgPath = ""; jsonOutput = false })

	out := &bytes.Buffer{}
	daemonStatusCmd.SetOut(out)
	if err := showDaemonStatus(daemonStatusCmd, nil); err != nil {
		t.Fatal(err)
	}
	var view output.DaemonStatusView
	envelope, err := output.DecodeEnvelope(out.Bytes(), &view)
	if err != nil {
		t.Fatal(err)
	}
	if envelope.Command != "daemon status" || view.Status != "stopped" {
		t.Fatalf("envelope/view = %#v/%#v", envelope, view)
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

func TestDiffDaemonSnapshotsClassifiesFileEvents(t *testing.T) {
	before := daemonSnapshot{Files: map[string]daemonFileState{"old": {Size: 1, Modified: 1, Root: "root"}, "sized": {Size: 1, Modified: 1, Root: "root"}}}
	after := daemonSnapshot{Files: map[string]daemonFileState{"new": {Size: 1, Modified: 1, Root: "root"}, "sized": {Size: 2, Modified: 2, Root: "root"}, "created": {Size: 3, Modified: 3, Root: "root"}}}
	changes := diffDaemonSnapshots(before, after)
	kinds := map[string]bool{}
	for _, change := range changes {
		kinds[change.Event.Kind] = true
	}
	for _, kind := range []string{"RENAME", "SIZE_CHANGE", "CREATE"} {
		if !kinds[kind] {
			t.Fatalf("missing %s in %#v", kind, changes)
		}
	}
}
