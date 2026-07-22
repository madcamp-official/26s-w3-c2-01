package homebrew

import (
	"context"
	"os/exec"
	"testing"
)

func TestCacheListerUsesCLIPath(t *testing.T) {
	cache := t.TempDir()
	lister := CacheLister{LookPath: func(string) (string, error) { return "brew", nil }, Run: func(context.Context, string, ...string) ([]byte, error) { return []byte(cache), nil }}
	got, err := lister.ListResources(context.Background())
	if err != nil || len(got) != 1 || got[0].Version != "homebrew-cache" {
		t.Fatalf("got %#v, %v", got, err)
	}
}

func TestCacheListerReturnsNothingWhenBrewMissing(t *testing.T) {
	lister := CacheLister{LookPath: func(string) (string, error) { return "", exec.ErrNotFound }}
	got, err := lister.ListResources(context.Background())
	if err != nil || len(got) != 0 {
		t.Fatalf("got %#v, %v", got, err)
	}
}
