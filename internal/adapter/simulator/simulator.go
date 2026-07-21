// Package simulator detects the iOS/watchOS/tvOS Simulator's cache
// directory. Like the npm/pnpm/Gradle/Xcode/CocoaPods/SwiftPM caches in
// sibling adapter packages, it is a global cache with no project ownership,
// classified ResourceTypeGlobalCache and REVIEW-only
// (internal/app/risk_policy.go).
//
// The `Devices/` directory (each simulator's installed apps and app data --
// typically the single largest macOS dev-space consumer, tens of GB) is
// detected read-only by DevicesLister below. Because it can hold state a
// developer seeded on purpose (test fixtures, logged-in sessions), it is
// deliberately NOT a cleanup target: it is classified REVIEW-only, like a
// global cache and like a Docker volume, and never quarantined or purged --
// libra only surfaces it and points at the official `xcrun simctl` cleanup.
// Distinguishing "unavailable" (safe to prune) from "available" (in use)
// devices would need `xcrun simctl list devices`, deferred until it can be
// verified on a machine with a real Simulator install.
//
// Still out of scope: the runtime images under `/Library/Developer/
// CoreSimulator` (system-wide, not per-user, managed via Xcode's Platforms
// settings -- the same "system component, not a cache" reasoning that
// excludes the Homebrew Cellar in the homebrew adapter).
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

// DevicesLister reports the CoreSimulator Devices directory as a read-only
// REVIEW resource. It reuses ResourceTypeGlobalCache -- not because it is a
// cache (it holds potentially-seeded app data), but because that type is
// structurally always REVIEW in DefaultRiskPolicy and can never reach the
// SAFE/quarantine path, which is exactly the safety property this
// user-data-bearing directory needs. Its distinct version "simulator-devices"
// selects a cautionary simctl guidance message (risk_policy.go).
type DevicesLister struct{ Environment cachepath.Environment }

func (l DevicesLister) ListResources(context.Context) ([]domain.Resource, error) {
	home, err := l.Environment.UserHome()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(home, "Library", "Developer", "CoreSimulator", "Devices")
	if !l.Environment.Directory(path) {
		return nil, nil
	}
	return []domain.Resource{cachepath.Resource("iOS Simulator devices", "simulator-devices", path, domain.ResourceTypeGlobalCache, domain.DefaultConfidence[domain.EvidenceDeclared])}, nil
}
