package safety

import (
	"path/filepath"
	"testing"
)

func TestPathClassifierMarksProtectedRootAndChildren(t *testing.T) {
	protectedRoot := filepath.Join(t.TempDir(), "Program Files")
	classifier, err := NewPathClassifier([]string{protectedRoot, protectedRoot})
	if err != nil {
		t.Fatalf("NewPathClassifier() error = %v", err)
	}

	got, err := classifier.Classify(filepath.Join(protectedRoot, "Windows Kits", "10"))
	if err != nil {
		t.Fatalf("Classify() error = %v", err)
	}
	if !got.SystemManaged || got.ProtectedRoot == "" {
		t.Fatalf("Classify() = %#v, want system-managed classification", got)
	}
}

func TestPathClassifierDoesNotUseStringPrefix(t *testing.T) {
	protectedRoot := filepath.Join(t.TempDir(), "Program")
	classifier, err := NewPathClassifier([]string{protectedRoot})
	if err != nil {
		t.Fatalf("NewPathClassifier() error = %v", err)
	}

	got, err := classifier.Classify(protectedRoot + " Files")
	if err != nil {
		t.Fatalf("Classify() error = %v", err)
	}
	if got.SystemManaged {
		t.Fatalf("Classify() = %#v, want unmanaged prefix sibling", got)
	}
}
