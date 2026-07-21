package android

import (
	"context"
	"path/filepath"
	"runtime"

	"github.com/madcamp-official/26s-w3-c2-01/internal/adapter/cachepath"
	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

type SDKLister struct {
	Environment cachepath.Environment
	GOOS        string
}

func (l SDKLister) ListResources(context.Context) ([]domain.Resource, error) {
	home, err := l.Environment.UserHome()
	if err != nil {
		return nil, err
	}
	path := l.Environment.Lookup("ANDROID_HOME")
	if path == "" {
		path = l.Environment.Lookup("ANDROID_SDK_ROOT")
	}
	goos := l.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}
	if path == "" {
		switch goos {
		case "windows":
			path = filepath.Join(l.Environment.Lookup("LOCALAPPDATA"), "Android", "Sdk")
		case "darwin":
			path = filepath.Join(home, "Library", "Android", "sdk")
		default:
			path = filepath.Join(home, "Android", "Sdk")
		}
	}
	if path == "" || !l.Environment.Directory(path) {
		return nil, nil
	}
	return []domain.Resource{cachepath.Resource("Android SDK", "android-sdk", path, domain.ResourceTypeAndroidSDK, domain.DefaultConfidence[domain.EvidenceDeclared])}, nil
}
