package git

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
)

// FindRepoRoot walks up from path through its ancestors looking for a .git
// entry (a directory, or for a linked worktree, a file). found is false, not
// an error, if path isn't inside any Git repository -- that's the common
// case for a project scanned outside version control.
func FindRepoRoot(path string) (root string, found bool, err error) {
	dir := path
	for {
		if _, statErr := os.Stat(filepath.Join(dir, ".git")); statErr == nil {
			return dir, true, nil
		} else if !os.IsNotExist(statErr) {
			return "", false, statErr
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false, nil
		}
		dir = parent
	}
}

// TrackedFilesChecker asks the real git binary whether it tracks any file
// under a directory. It mirrors msbuild.VSWhereToolLocator's shape: Run is
// injectable for tests and defaults to actually running the command.
type TrackedFilesChecker struct {
	// Run executes git with args and returns its stdout. Overridable for
	// tests; defaults to actually running the command via exec.CommandContext
	// (an argument slice, not a shell string, so nothing here is vulnerable
	// to shell injection regardless of what repoRoot/path contain).
	Run func(ctx context.Context, args ...string) ([]byte, error)
}

func (c TrackedFilesChecker) run(ctx context.Context, args ...string) ([]byte, error) {
	if c.Run != nil {
		return c.Run(ctx, args...)
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// HasTrackedFiles reports whether git, inside repoRoot, tracks any file at or
// under path (via `git -C repoRoot ls-files -- path`). Callers that want
// "no repository at all" to mean "vacuously no tracked files" should check
// FindRepoRoot's found result first -- this function always requires a real
// repoRoot and surfaces a missing/failed git invocation as an error rather
// than guessing.
func (c TrackedFilesChecker) HasTrackedFiles(ctx context.Context, repoRoot, path string) (bool, error) {
	output, err := c.run(ctx, "-C", repoRoot, "ls-files", "--", path)
	if err != nil {
		return false, err
	}
	return len(bytes.TrimSpace(output)) > 0, nil
}
