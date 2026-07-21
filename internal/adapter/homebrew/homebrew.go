// Package homebrew detects Homebrew's download cache via `brew --cache`,
// the same CLI-resolved-path approach npm and pnpm use for their global
// caches. It is a global cache with no project ownership, classified
// ResourceTypeGlobalCache and REVIEW-only (internal/app/risk_policy.go) --
// the Homebrew Cellar itself (installed formulae/casks) is a system
// component, not a cache, and is deliberately out of scope here.
package homebrew

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"

	"github.com/madcamp-official/26s-w3-c2-01/internal/adapter/cachepath"
	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
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
	path, err := look("brew")
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
	value, err := run(ctx, path, "--cache")
	if err != nil {
		return nil, err
	}
	cache := strings.TrimSpace(string(value))
	info, err := os.Stat(cache)
	if err != nil || !info.IsDir() {
		return nil, nil
	}
	return []domain.Resource{cachepath.Resource("Homebrew cache", "homebrew-cache", cache, domain.ResourceTypeGlobalCache, domain.DefaultConfidence[domain.EvidenceResolved])}, nil
}
