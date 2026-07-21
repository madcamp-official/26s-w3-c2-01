package android

import (
	"context"
	"github.com/madcamp-official/26s-w3-c2-01/internal/adapter/cachepath"
	"os"
	"path/filepath"
	"testing"
)

func TestSDKListerUsesAndroidHome(t *testing.T) {
	root := filepath.Join(t.TempDir(), "sdk")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	lister := SDKLister{Environment: cachepath.Environment{Home: t.TempDir(), Env: func(key string) string {
		if key == "ANDROID_HOME" {
			return root
		}
		return ""
	}}}
	got, err := lister.ListResources(context.Background())
	if err != nil || len(got) != 1 || got[0].DisplayPath != root {
		t.Fatalf("got %#v, %v", got, err)
	}
}
