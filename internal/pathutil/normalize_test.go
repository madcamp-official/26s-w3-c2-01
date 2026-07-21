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

// TestNormalizeNFCAndNFDFormsProduceIdenticalIdentity is a regression test
// for issue #49: a path can reach Normalize as either NFC (composed) or NFD
// (decomposed) Unicode bytes -- e.g. macOS's APFS tends to hand back NFD
// when walking a real directory, while a CLI argument or config file tends
// to stay NFC -- even though both render as the same visible text. Uses
// literal Go string constants (not real filesystem paths from t.TempDir()
// entries) specifically so the two byte forms are deterministic and don't
// depend on the host filesystem's own normalization behavior.
func TestNormalizeNFCAndNFDFormsProduceIdenticalIdentity(t *testing.T) {
	root := t.TempDir()
	nfc := filepath.Join(root, "café")  // 'é' as one composed rune (U+00E9)
	nfd := filepath.Join(root, "café") // 'e' + combining acute accent (U+0301)
	if nfc == nfd {
		t.Fatal("test fixture invalid: NFC and NFD forms must be byte-distinct")
	}

	gotNFC, err := Normalize(nfc)
	if err != nil {
		t.Fatalf("Normalize(NFC) error = %v", err)
	}
	gotNFD, err := Normalize(nfd)
	if err != nil {
		t.Fatalf("Normalize(NFD) error = %v", err)
	}
	if gotNFC != gotNFD {
		t.Fatalf("Normalize(NFC) = %q, Normalize(NFD) = %q, want identical stable identity", gotNFC, gotNFD)
	}
}

// TestAbsolutePreservesOriginalUnicodeForm confirms Absolute (the basis for
// DisplayPath, per docs/libra_integration_contracts.md §3 "UI에는
// DisplayPath") does not NFC-normalize -- only Normalize's comparison/ID
// value does. Changing what a user actually typed or what the OS actually
// returned would be a separate, much larger behavior change than #49 asks
// for.
func TestAbsolutePreservesOriginalUnicodeForm(t *testing.T) {
	root := t.TempDir()
	nfd := filepath.Join(root, "café")

	got, err := Absolute(nfd)
	if err != nil {
		t.Fatalf("Absolute() error = %v", err)
	}
	if filepath.Base(got) != "café" {
		t.Fatalf("Absolute() = %q, want original NFD bytes preserved, not normalized", got)
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
