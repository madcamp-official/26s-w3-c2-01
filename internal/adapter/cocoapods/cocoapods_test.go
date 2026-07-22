package cocoapods

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/adapter/cachepath"
)

func TestCacheListerFindsExistingCache(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "Library", "Caches", "CocoaPods")
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
	lister := CacheLister{Environment: cachepath.Environment{Home: home}}
	got, err := lister.ListResources(context.Background())
	if err != nil || len(got) != 1 || got[0].DisplayPath != path || got[0].Version != "cocoapods-cache" {
		t.Fatalf("got %#v, %v", got, err)
	}
}

func TestCacheListerReturnsNothingWhenAbsent(t *testing.T) {
	lister := CacheLister{Environment: cachepath.Environment{Home: t.TempDir()}}
	got, err := lister.ListResources(context.Background())
	if err != nil || len(got) != 0 {
		t.Fatalf("got %#v, %v", got, err)
	}
}
