//go:build windows

// Real-filesystem NTFS edge cases for QuarantineEngine, beyond the happy
// path covered by quarantine_test.go: a file another process holds open, a
// path the current user has been explicitly denied access to, a junction
// (reparse point) directory, and a quarantine record whose on-disk item has
// since disappeared out from under it. These exercise actual Windows I/O
// (real locks, real icacls ACLs, real mklink junctions) rather than
// synthetic errors, per docs/libra_cli_commands_and_schedule.md Day 6 "실제
// 파일시스템 cleanup/restore 검증".
package safety

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

// lockFileExclusive opens path with no sharing (FILE_SHARE_MODE = 0), the
// same condition a running build tool or editor leaves behind when it holds
// a file open, and blocks any rename/delete of it until closed.
func lockFileExclusive(t *testing.T, path string) {
	t.Helper()
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatalf("UTF16PtrFromString(%q): %v", path, err)
	}
	handle, err := syscall.CreateFile(pathPtr,
		syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		0, // no sharing: rename/delete of this path must fail while held
		nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatalf("CreateFile(%q) exclusive: %v", path, err)
	}
	t.Cleanup(func() { _ = syscall.CloseHandle(handle) })
}

func TestQuarantineEngineMoveFailsGracefullyWhenFileIsLockedOpen(t *testing.T) {
	root := t.TempDir()
	original := filepath.Join(root, "project", "dist", "bundle.js")
	if err := os.MkdirAll(filepath.Dir(original), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(original, []byte("output"), 0o644); err != nil {
		t.Fatal(err)
	}
	lockFileExclusive(t, original)

	engine := QuarantineEngine{RootForPath: func(_, transactionID string) string { return filepath.Join(root, "quarantine", transactionID) }}
	transaction := domain.CleanupTransaction{ID: "tx-locked", PlanID: "plan-locked", Items: []domain.CleanupTransactionItem{
		{ID: "item-1", OriginalPath: original, Status: domain.TransactionItemPending},
	}}
	if err := engine.Prepare(&transaction); err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}

	engine.Move(context.Background(), &transaction)

	if transaction.Items[0].Status != domain.TransactionItemFailed {
		t.Fatalf("status = %s, want FAILED (file is locked open)", transaction.Items[0].Status)
	}
	if transaction.Items[0].Reason == "" {
		t.Fatal("want a non-empty failure reason so the user knows why cleanup was refused")
	}
	if _, err := os.Stat(original); err != nil {
		t.Fatalf("locked file must remain exactly where it was, not partially moved: %v", err)
	}
}

// currentUserForACL returns DOMAIN\User or just User the way icacls expects
// it on the command line.
func currentUserForACL(t *testing.T) string {
	t.Helper()
	if user := os.Getenv("USERNAME"); user != "" {
		return user
	}
	t.Skip("USERNAME env var not set; cannot target an ACL deny at the current user")
	return ""
}

func TestQuarantineEngineMoveFailsGracefullyWhenAccessIsDenied(t *testing.T) {
	root := t.TempDir()
	original := filepath.Join(root, "project", "build", "output.dll")
	if err := os.MkdirAll(filepath.Dir(original), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(original, []byte("binary"), 0o644); err != nil {
		t.Fatal(err)
	}
	user := currentUserForACL(t)

	if out, err := exec.Command("icacls", original, "/inheritance:r", "/deny", user+":(F)").CombinedOutput(); err != nil {
		t.Skipf("icacls deny unsupported in this environment: %v: %s", err, out)
	}
	t.Cleanup(func() {
		_ = exec.Command("icacls", original, "/reset").Run()
	})

	engine := QuarantineEngine{RootForPath: func(_, transactionID string) string { return filepath.Join(root, "quarantine", transactionID) }}
	transaction := domain.CleanupTransaction{ID: "tx-denied", PlanID: "plan-denied", Items: []domain.CleanupTransactionItem{
		{ID: "item-1", OriginalPath: original, Status: domain.TransactionItemPending},
	}}
	if err := engine.Prepare(&transaction); err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}

	engine.Move(context.Background(), &transaction)

	if transaction.Items[0].Status != domain.TransactionItemFailed {
		t.Fatalf("status = %s, want FAILED (access denied by ACL)", transaction.Items[0].Status)
	}
	if transaction.Items[0].Reason == "" {
		t.Fatal("want a non-empty failure reason")
	}
	// The ACL deny must be lifted (via t.Cleanup above) before TempDir's own
	// cleanup tries to remove this file, otherwise TestMain fails for an
	// unrelated reason and masks the actual assertion above.
}

// createJunction shells out to mklink /J (a cmd.exe builtin), which -- unlike
// os.Symlink's directory-symlink mode -- does not require
// SeCreateSymbolicLinkPrivilege, so it works on an unprivileged developer
// account.
func createJunction(t *testing.T, link, target string) {
	t.Helper()
	out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput()
	if err != nil {
		t.Skipf("mklink /J unsupported in this environment: %v: %s", err, out)
	}
}

func TestQuarantineEngineMovesJunctionAsOpaqueUnitWithoutTouchingTarget(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "real-sdk-install")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	targetFile := filepath.Join(target, "payload.txt")
	if err := os.WriteFile(targetFile, []byte("do not move me"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "project", "node_modules")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	createJunction(t, link, target)

	engine := QuarantineEngine{RootForPath: func(_, transactionID string) string { return filepath.Join(root, "quarantine", transactionID) }}
	transaction := domain.CleanupTransaction{ID: "tx-junction", PlanID: "plan-junction", Items: []domain.CleanupTransactionItem{
		{ID: "item-1", OriginalPath: link, Status: domain.TransactionItemPending},
	}}
	if err := engine.Prepare(&transaction); err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}

	engine.Move(context.Background(), &transaction)

	if transaction.Items[0].Status != domain.TransactionItemMoved {
		t.Fatalf("status = %s, want MOVED (junction point itself relocates)", transaction.Items[0].Status)
	}
	// The critical safety property: moving the junction must never touch
	// what it points at. If the engine ever followed the reparse point
	// instead of renaming it as an opaque node, this file would be gone.
	if data, err := os.ReadFile(targetFile); err != nil || string(data) != "do not move me" {
		t.Fatalf("junction target was modified by quarantine move: data=%q err=%v", data, err)
	}
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Fatalf("junction still present at original path: %v", err)
	}
	quarantined := transaction.Items[0].QuarantinePath
	info, err := os.Lstat(quarantined)
	if err != nil {
		t.Fatalf("quarantined junction missing: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		reparse, rErr := IsReparsePoint(quarantined)
		if rErr != nil || !reparse {
			t.Fatalf("quarantined entry is no longer a reparse point (junction was dereferenced): mode=%v reparse=%v err=%v", info.Mode(), reparse, rErr)
		}
	}

	engine.Restore(context.Background(), &transaction)
	if transaction.Items[0].Status != domain.TransactionItemRestored {
		t.Fatalf("restore status = %s, want RESTORED", transaction.Items[0].Status)
	}
	if data, err := os.ReadFile(filepath.Join(link, "payload.txt")); err != nil || string(data) != "do not move me" {
		t.Fatalf("restored junction does not resolve to original target: data=%q err=%v", data, err)
	}
}

func TestQuarantineEngineRestoreFailsGracefullyWhenQuarantineItemVanishedFromDisk(t *testing.T) {
	root := t.TempDir()
	original := filepath.Join(root, "project", "dist")
	if err := os.MkdirAll(original, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(original, "bundle.js"), []byte("output"), 0o644); err != nil {
		t.Fatal(err)
	}
	engine := QuarantineEngine{RootForPath: func(_, transactionID string) string { return filepath.Join(root, "quarantine", transactionID) }}
	transaction := domain.CleanupTransaction{ID: "tx-mismatch", PlanID: "plan-mismatch", Items: []domain.CleanupTransactionItem{
		{ID: "item-1", OriginalPath: original, Status: domain.TransactionItemPending},
	}}
	if err := engine.Prepare(&transaction); err != nil {
		t.Fatal(err)
	}
	engine.Move(context.Background(), &transaction)
	if transaction.Items[0].Status != domain.TransactionItemMoved {
		t.Fatalf("setup: move status = %s", transaction.Items[0].Status)
	}

	// Simulate the transaction record (what the DB/manifest believes
	// happened) drifting from real disk state: something outside libra --
	// antivirus, a manual cleanup, disk corruption -- removed the
	// quarantined copy after it was moved but before restore runs.
	if err := os.RemoveAll(transaction.Items[0].QuarantinePath); err != nil {
		t.Fatal(err)
	}

	engine.Restore(context.Background(), &transaction)

	if transaction.Items[0].Status != domain.TransactionItemFailed {
		t.Fatalf("status = %s, want FAILED (quarantine copy missing from disk)", transaction.Items[0].Status)
	}
	if !strings.Contains(transaction.Items[0].Reason, "missing") {
		t.Fatalf("reason = %q, want it to mention the quarantine copy is missing", transaction.Items[0].Reason)
	}
	// Nothing should have been fabricated at the original path either.
	if _, err := os.Stat(original); !os.IsNotExist(err) {
		t.Fatalf("original path should stay absent after a failed restore: %v", err)
	}
}

func TestQuarantineEngineDefaultRootStaysOnSameVolumeAsOriginal(t *testing.T) {
	root := t.TempDir()
	original := filepath.Join(root, "project", "dist")
	if err := os.MkdirAll(original, 0o755); err != nil {
		t.Fatal(err)
	}
	// No RootForPath override: exercise the engine's real production
	// default, which must never place quarantine on a different volume
	// than the file being quarantined (a cross-volume rename would fail,
	// or worse, silently degrade to copy+delete and lose atomicity).
	engine := QuarantineEngine{}
	transaction := domain.CleanupTransaction{ID: "tx-samevol", PlanID: "plan-samevol", Items: []domain.CleanupTransactionItem{
		{ID: "item-1", OriginalPath: original, Status: domain.TransactionItemPending},
	}}
	if err := engine.Prepare(&transaction); err != nil {
		t.Fatal(err)
	}

	wantVolume := filepath.VolumeName(original)
	gotVolume := filepath.VolumeName(transaction.Items[0].QuarantinePath)
	if wantVolume == "" || gotVolume != wantVolume {
		t.Fatalf("quarantine path volume = %q, want %q (same volume as original %q)", gotVolume, wantVolume, original)
	}
}
