package xcode

import (
	"context"
	"os/exec"
	"testing"
)

const sampleXcodebuildVersionOutput = "Xcode 15.4\nBuild version 15F31d\n"

func TestInstallListerReportsFullXcode(t *testing.T) {
	lister := InstallLister{
		LookPath: func(name string) (string, error) { return "/usr/bin/" + name, nil },
		Run: func(ctx context.Context, path string, args ...string) ([]byte, error) {
			switch {
			case path == "/usr/bin/xcode-select":
				return []byte("/Applications/Xcode.app/Contents/Developer\n"), nil
			case path == "/usr/bin/xcodebuild":
				return []byte(sampleXcodebuildVersionOutput), nil
			}
			t.Fatalf("unexpected run path %q", path)
			return nil, nil
		},
	}

	got, err := lister.ListResources(context.Background())
	if err != nil {
		t.Fatalf("ListResources() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %#v, want one Xcode resource", got)
	}
	if got[0].Version != "15.4" {
		t.Errorf("Version = %q, want %q", got[0].Version, "15.4")
	}
	if got[0].DisplayPath != "/Applications/Xcode.app" {
		t.Errorf("DisplayPath = %q, want %q", got[0].DisplayPath, "/Applications/Xcode.app")
	}
	if !got[0].SystemManaged {
		t.Error("want SystemManaged=true regardless of install location")
	}
}

func TestInstallListerReturnsNothingWithCommandLineToolsOnly(t *testing.T) {
	lister := InstallLister{
		LookPath: func(name string) (string, error) { return "/usr/bin/" + name, nil },
		Run: func(ctx context.Context, path string, args ...string) ([]byte, error) {
			if path == "/usr/bin/xcode-select" {
				return []byte("/Library/Developer/CommandLineTools\n"), nil
			}
			// xcodebuild -version fails when only Command Line Tools are active.
			return nil, &exec.ExitError{}
		},
	}

	got, err := lister.ListResources(context.Background())
	if err != nil {
		t.Fatalf("ListResources() error = %v, want nil (CLT-only is a valid absence)", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %#v, want none", got)
	}
}

func TestInstallListerReturnsNothingWhenToolMissing(t *testing.T) {
	lister := InstallLister{
		LookPath: func(string) (string, error) { return "", exec.ErrNotFound },
	}
	got, err := lister.ListResources(context.Background())
	if err != nil || len(got) != 0 {
		t.Fatalf("got %#v, %v", got, err)
	}
}
