package pnpm

import (
	"context"
	"errors"
	"github.com/madcamp-official/26s-w3-c2-01/internal/adapter/cachepath"
	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"os"
	"os/exec"
	"strings"
)

type StoreLister struct {
	LookPath func(string) (string, error)
	Run      func(context.Context, string, ...string) ([]byte, error)
}

func (l StoreLister) ListResources(ctx context.Context) ([]domain.Resource, error) {
	look := l.LookPath
	if look == nil {
		look = exec.LookPath
	}
	path, err := look("pnpm")
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
	value, err := run(ctx, path, "store", "path")
	if err != nil {
		return nil, err
	}
	store := strings.TrimSpace(string(value))
	info, err := os.Stat(store)
	if err != nil || !info.IsDir() {
		return nil, nil
	}
	return []domain.Resource{cachepath.Resource("pnpm global store", "pnpm", store, domain.ResourceTypeGlobalCache, domain.DefaultConfidence[domain.EvidenceResolved])}, nil
}
