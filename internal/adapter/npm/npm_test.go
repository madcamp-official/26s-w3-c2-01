package npm

import (
	"context"
	"testing"
)

func TestCacheListerUsesCLIPath(t *testing.T) {
	cache := t.TempDir()
	lister := CacheLister{LookPath: func(string) (string, error) { return "npm", nil }, Run: func(context.Context, string, ...string) ([]byte, error) { return []byte(cache), nil }}
	got, err := lister.ListResources(context.Background())
	if err != nil || len(got) != 1 || got[0].Version != "npm" {
		t.Fatalf("got %#v, %v", got, err)
	}
}
