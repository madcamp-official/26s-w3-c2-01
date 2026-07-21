package npm

import (
	"context"
	"errors"
	"github.com/madcamp-official/26s-w3-c2-01/internal/adapter/cachepath"
	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"os"
	"os/exec"
	"strings"
)

type CacheLister struct {
	LookPath func(string) (string, error)
	Run      func(context.Context, string, ...string) ([]byte, error)
}

func (l CacheLister) ListResources(ctx context.Context) ([]domain.Resource, error) {
	look := l.LookPath
	if look == nil {
		look = exec.LookPath
	}
	path, err := look("npm")
	if errors.Is(err, exec.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	run := l.Run
	if run == nil {
		run = func(ctx context.Context, path string, args ...string) ([]byte, error) {
			return exec.CommandContext(ctx, path, args...).Output()
		}
	}
	value, err := run(ctx, path, "config", "get", "cache")
	if err != nil {
		return nil, err
	}
	cache := strings.TrimSpace(string(value))
	info, err := os.Stat(cache)
	if err != nil || !info.IsDir() {
		return nil, nil
	}
	return []domain.Resource{cachepath.Resource("npm global cache", "npm", cache, domain.ResourceTypeGlobalCache, domain.DefaultConfidence[domain.EvidenceResolved])}, nil
}
