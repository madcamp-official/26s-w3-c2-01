//go:build windows

package pathutil

import (
	"strings"
	"testing"
)

func TestNormalizeIsCaseInsensitiveOnWindows(t *testing.T) {
	upper, err := Normalize(`Relative\MixedCase`)
	if err != nil {
		t.Fatalf("Normalize(upper) error = %v", err)
	}
	lower, err := Normalize(`relative/mixedcase/`)
	if err != nil {
		t.Fatalf("Normalize(lower) error = %v", err)
	}
	if upper != lower {
		t.Fatalf("normalized paths differ: %q != %q", upper, lower)
	}
	if upper != strings.ToLower(upper) {
		t.Fatalf("Normalize() = %q, want lowercase comparison key", upper)
	}
}
