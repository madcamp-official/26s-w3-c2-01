// Package cocoapods detects CocoaPods' download cache, the local mirror of
// pod sources kept outside any project so repeated `pod install` runs
// (across every project on the machine) skip re-downloading. Like the
// npm/pnpm/Gradle/Maven caches in sibling adapter packages, it is a global
// cache with no project ownership, so it is classified ResourceTypeGlobalCache
// and stays REVIEW-only (internal/app/risk_policy.go).
package cocoapods

import (
	"context"
	"path/filepath"

	"github.com/madcamp-official/26s-w3-c2-01/internal/adapter/cachepath"
	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

type CacheLister struct{ Environment cachepath.Environment }

func (l CacheLister) ListResources(context.Context) ([]domain.Resource, error) {
	home, err := l.Environment.UserHome()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(home, "Library", "Caches", "CocoaPods")
	if !l.Environment.Directory(path) {
		return nil, nil
	}
	return []domain.Resource{cachepath.Resource("CocoaPods cache", "cocoapods-cache", path, domain.ResourceTypeGlobalCache, domain.DefaultConfidence[domain.EvidenceDeclared])}, nil
}
