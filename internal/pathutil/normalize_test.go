package pathutil

import (
	"errors"
	"path/filepath"
	"runtime"
	"testing"
)

func TestNormalizeReturnsAbsoluteCleanPath(t *testing.T) {
	got, err := Normalize(filepath.Join("relative", "child", "..", "file"))
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("Normalize() = %q, want absolute path", got)
	}
	if filepath.Base(got) != "file" {
		t.Fatalf("Normalize() = %q, want cleaned path", got)
	}
}

func TestNormalizeRejectsEmptyPath(t *testing.T) {
	_, err := Normalize("  ")
	if !errors.Is(err, ErrEmptyPath) {
		t.Fatalf("Normalize() error = %v, want ErrEmptyPath", err)
	}
}

func TestAbsolutePreservesDisplayCase(t *testing.T) {
	got, err := Absolute(filepath.Join("Relative", "MixedCase"))
	if err != nil {
		t.Fatalf("Absolute() error = %v", err)
	}
	if filepath.Base(got) != "MixedCase" {
		t.Fatalf("Absolute() = %q, want display case preserved", got)
	}
}

func TestEqualNormalizesPaths(t *testing.T) {
	root := t.TempDir()
	equal, err := Equal(filepath.Join(root, "child", ".."), root)
	if err != nil {
		t.Fatalf("Equal() error = %v", err)
	}
	if !equal {
		t.Fatal("Equal() = false, want true")
	}
}

func TestIsSameOrChildUsesPathBoundaries(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "same", path: root, want: true},
		{name: "child", path: filepath.Join(root, "SDK", "bin"), want: true},
		{name: "prefix sibling", path: root + "-backup", want: false},
		{name: "parent", path: filepath.Dir(root), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IsSameOrChild(tt.path, root)
			if err != nil {
				t.Fatalf("IsSameOrChild() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("IsSameOrChild() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsSameOrChildReturnsFalseAcrossWindowsVolumes(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows volume semantics")
	}

	got, err := IsSameOrChild(`D:\Projects\app`, `C:\Windows`)
	if err != nil {
		t.Fatalf("IsSameOrChild() error = %v", err)
	}
	if got {
		t.Fatal("IsSameOrChild() = true across different volumes")
	}
}
