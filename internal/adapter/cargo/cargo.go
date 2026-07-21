package cargo

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
	root := l.Environment.Lookup("CARGO_HOME")
	if root == "" {
		root = filepath.Join(home, ".cargo")
	}
	var out []domain.Resource
	for _, item := range []struct{ name, version, sub string }{{"Cargo registry cache", "cargo-registry", "registry"}, {"Cargo git cache", "cargo-git", "git"}} {
		path := filepath.Join(root, item.sub)
		if l.Environment.Directory(path) {
			out = append(out, cachepath.Resource(item.name, item.version, path, domain.ResourceTypeGlobalCache, domain.DefaultConfidence[domain.EvidenceDeclared]))
		}
	}
	return out, nil
}
