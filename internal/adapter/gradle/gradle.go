package gradle

import (
	"context"
	"github.com/madcamp-official/26s-w3-c2-01/internal/adapter/cachepath"
	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"path/filepath"
)

type CacheLister struct{ Environment cachepath.Environment }

func (l CacheLister) ListResources(context.Context) ([]domain.Resource, error) {
	home, err := l.Environment.UserHome()
	if err != nil {
		return nil, err
	}
	root := l.Environment.Lookup("GRADLE_USER_HOME")
	if root == "" {
		root = filepath.Join(home, ".gradle")
	}
	path := filepath.Join(root, "caches")
	if !l.Environment.Directory(path) {
		return nil, nil
	}
	return []domain.Resource{cachepath.Resource("Gradle global cache", "gradle", path, domain.ResourceTypeGlobalCache, domain.DefaultConfidence[domain.EvidenceDeclared])}, nil
}
