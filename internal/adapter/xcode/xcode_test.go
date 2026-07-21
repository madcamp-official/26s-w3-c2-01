package xcode

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/adapter/cachepath"
)

func TestDerivedDataListerFindsExistingCache(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "Library", "Developer", "Xcode", "DerivedData")
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
	lister := DerivedDataLister{Environment: cachepath.Environment{Home: home}}
	got, err := lister.ListResources(context.Background())
	if err != nil || len(got) != 1 || got[0].DisplayPath != path || got[0].Version != "xcode-deriveddata" {
		t.Fatalf("got %#v, %v", got, err)
	}
}

func TestDerivedDataListerReturnsNothingWhenAbsent(t *testing.T) {
	lister := DerivedDataLister{Environment: cachepath.Environment{Home: t.TempDir()}}
	got, err := lister.ListResources(context.Background())
	if err != nil || len(got) != 0 {
		t.Fatalf("got %#v, %v", got, err)
	}
}
