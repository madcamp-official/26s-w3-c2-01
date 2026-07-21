// Package xcode detects Xcode's DerivedData cache. DerivedData holds build
// intermediates, indexes, and previews Xcode regenerates from scratch on
// the next build -- it is a pure cache, never a source of user data, so
// unlike the project-owned artifacts elsewhere in libra it carries no
// OWNS edge and is only ever offered for manual review (see
// internal/app/risk_policy.go's ResourceTypeGlobalCache handling).
package xcode

import (
	"context"
	"path/filepath"

	"github.com/madcamp-official/26s-w3-c2-01/internal/adapter/cachepath"
	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

type DerivedDataLister struct{ Environment cachepath.Environment }

func (l DerivedDataLister) ListResources(context.Context) ([]domain.Resource, error) {
	home, err := l.Environment.UserHome()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(home, "Library", "Developer", "Xcode", "DerivedData")
	if !l.Environment.Directory(path) {
		return nil, nil
	}
	return []domain.Resource{cachepath.Resource("Xcode DerivedData", "xcode-deriveddata", path, domain.ResourceTypeGlobalCache, domain.DefaultConfidence[domain.EvidenceDeclared])}, nil
}
