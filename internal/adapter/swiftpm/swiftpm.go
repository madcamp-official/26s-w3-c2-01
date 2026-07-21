// Package swiftpm detects the Swift Package Manager's global repository
// cache, where SwiftPM keeps cloned package git histories so resolving the
// same dependency across multiple packages does not re-clone it. It is a
// global cache like npm/pnpm/Gradle/Maven's, classified
// ResourceTypeGlobalCache and REVIEW-only (internal/app/risk_policy.go).
//
// Only the macOS cache location is resolved for now -- Linux's
// (~/.cache/org.swift.swiftpm) is left for a follow-up once SwiftPM-on-Linux
// usage is actually in scope, rather than guessing at an unverified path.
package swiftpm

import (
	"context"
	"path/filepath"
	"runtime"

	"github.com/madcamp-official/26s-w3-c2-01/internal/adapter/cachepath"
	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

type CacheLister struct {
	Environment cachepath.Environment
	GOOS        string
}

func (l CacheLister) ListResources(context.Context) ([]domain.Resource, error) {
	goos := l.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}
	if goos != "darwin" {
		return nil, nil
	}
	home, err := l.Environment.UserHome()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(home, "Library", "Caches", "org.swift.swiftpm")
	if !l.Environment.Directory(path) {
		return nil, nil
	}
	return []domain.Resource{cachepath.Resource("Swift Package Manager cache", "swiftpm-cache", path, domain.ResourceTypeGlobalCache, domain.DefaultConfidence[domain.EvidenceDeclared])}, nil
}
