// Package simulator detects the iOS/watchOS/tvOS Simulator's cache
// directory. Like the npm/pnpm/Gradle/Xcode/CocoaPods/SwiftPM caches in
// sibling adapter packages, it is a global cache with no project ownership,
// classified ResourceTypeGlobalCache and REVIEW-only
// (internal/app/risk_policy.go).
//
// Deliberately out of scope here: `Devices/` (each simulator's installed
// apps and app data -- Apple frames it as disposable via `simctl erase`, but
// unlike a pure cache it can hold state a developer seeded on purpose, so
// classifying it needs its own decision) and the runtime images under
// `/Library/Developer/CoreSimulator` (system-wide, not per-user, managed via
// Xcode's Platforms settings -- the same "system component, not a cache"
// reasoning that already excludes the Homebrew Cellar in the homebrew
// adapter).
package simulator

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
	path := filepath.Join(home, "Library", "Developer", "CoreSimulator", "Caches")
	if !l.Environment.Directory(path) {
		return nil, nil
	}
	return []domain.Resource{cachepath.Resource("iOS Simulator cache", "simulator-cache", path, domain.ResourceTypeGlobalCache, domain.DefaultConfidence[domain.EvidenceDeclared])}, nil
}
