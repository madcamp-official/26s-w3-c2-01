package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestFindRepoRoot_FindsAncestorGitDir(t *testing.T) {
	repoRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(repoRoot, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(repoRoot, "src", "GameClient", "bin")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	root, found, err := FindRepoRoot(nested)
	if err != nil {
		t.Fatalf("FindRepoRoot() error = %v", err)
	}
	if !found || root != repoRoot {
		t.Errorf("FindRepoRoot() = (%q, %v), want (%q, true)", root, found, repoRoot)
	}
}

func TestFindRepoRoot_NotFound(t *testing.T) {
	dir := t.TempDir()

	_, found, err := FindRepoRoot(dir)
	if err != nil {
		t.Fatalf("FindRepoRoot() error = %v", err)
	}
	if found {
		t.Error("FindRepoRoot() found = true, want false (no .git anywhere above)")
	}
}

func TestTrackedFilesChecker_HasTrackedFiles(t *testing.T) {
	cases := []struct {
		name   string
		output string
		want   bool
	}{
		{"no tracked files", "", false},
		{"blank output", "\n  \n", false},
		{"one tracked file", "obj/Licenses.txt\n", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotArgs []string
			checker := TrackedFilesChecker{
				Run: func(_ context.Context, args ...string) ([]byte, error) {
					gotArgs = args
					return []byte(tc.output), nil
				},
			}

			got, err := checker.HasTrackedFiles(context.Background(), `D:\Repo`, `D:\Repo\GameClient\obj`)
			if err != nil {
				t.Fatalf("HasTrackedFiles() error = %v", err)
			}
			if got != tc.want {
				t.Errorf("HasTrackedFiles() = %v, want %v", got, tc.want)
			}
			pathspec := `D:\Repo\GameClient\obj`
			if runtime.GOOS == "windows" {
				pathspec = "GameClient/obj"
			}
			wantArgs := []string{"-C", `D:\Repo`, "ls-files", "--", pathspec}
			if !reflect.DeepEqual(gotArgs, wantArgs) {
				t.Errorf("git args = %v, want %v", gotArgs, wantArgs)
			}
		})
	}
}

func TestTrackedFilesChecker_RunErrorPropagates(t *testing.T) {
	runErr := errors.New("exec: \"git\": executable file not found in $PATH")
	checker := TrackedFilesChecker{
		Run: func(context.Context, ...string) ([]byte, error) { return nil, runErr },
	}

	_, err := checker.HasTrackedFiles(context.Background(), `D:\Repo`, `D:\Repo\GameClient\obj`)
	if !errors.Is(err, runErr) {
		t.Errorf("HasTrackedFiles() error = %v, want %v", err, runErr)
	}
}
