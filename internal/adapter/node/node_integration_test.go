package node_test

import (
	"context"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/adapter/node"
	"github.com/madcamp-official/26s-w3-c2-01/internal/app"
	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/safety"
	"github.com/madcamp-official/26s-w3-c2-01/internal/scanner"
	sqlitestore "github.com/madcamp-official/26s-w3-c2-01/internal/store/sqlite"
)

// TestDetectArtifactsPersistThroughResourceService locks down that node
// adapter candidates are compatible end-to-end with the already-confirmed
// Resource pipeline (docs/libra_integration_contracts.md §7.3, §18.4):
// app.ResourceService measures, classifies, and persists them without any
// node-specific handling on its side.
func TestDetectArtifactsPersistThroughResourceService(t *testing.T) {
	db, err := sqlitestore.Open(":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := sqlitestore.Migrate(db); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	repository := sqlitestore.NewResourceRepository(db)
	classifier, err := safety.NewPathClassifier(nil)
	if err != nil {
		t.Fatalf("NewPathClassifier: %v", err)
	}
	service := app.NewResourceService(scanner.New(2), repository, classifier, app.DefaultRiskPolicy{})

	candidates, err := node.DetectArtifacts("../../../testdata/node/basic")
	if err != nil {
		t.Fatalf("DetectArtifacts: %v", err)
	}
	if len(candidates) != 2 {
		t.Fatalf("DetectArtifacts = %d candidates, want 2 (node_modules, dist)", len(candidates))
	}

	observed := map[domain.ResourceType]app.ResourceObservation{}
	for _, candidate := range candidates {
		observation, err := service.Observe(context.Background(), candidate)
		if err != nil {
			t.Fatalf("Observe(%+v): %v", candidate, err)
		}
		observed[candidate.Type] = observation
	}

	nodeModules := observed[domain.ResourceTypeNodeModules].Resource
	if nodeModules.ID == "" || nodeModules.NormalizedPath == "" {
		t.Fatalf("node_modules identity = %#v, want populated ID and normalized path", nodeModules)
	}
	if nodeModules.LogicalSize <= 0 {
		t.Errorf("node_modules LogicalSize = %d, want > 0 (fixture has a tracked file)", nodeModules.LogicalSize)
	}
	if nodeModules.SystemManaged {
		t.Error("node_modules under testdata should not be SystemManaged")
	}
	// DefaultRiskPolicy has no SAFE branch yet -- see risk_policy.go comment
	// and docs/libra_integration_contracts.md §20.3 -- so a non-protected,
	// non-system-managed candidate lands as REVIEW, not SAFE, until the
	// team extends the shared risk formula.
	if nodeModules.Risk != domain.RiskReview {
		t.Errorf("node_modules Risk = %v, want %v", nodeModules.Risk, domain.RiskReview)
	}

	dist := observed[domain.ResourceTypeBuildOutput].Resource
	if dist.LogicalSize <= 0 {
		t.Errorf("dist LogicalSize = %d, want > 0 (fixture has a tracked file)", dist.LogicalSize)
	}

	persisted, err := repository.ListByType(context.Background(), domain.ResourceTypeNodeModules)
	if err != nil {
		t.Fatalf("ListByType: %v", err)
	}
	if len(persisted) != 1 || persisted[0].ID != nodeModules.ID {
		t.Fatalf("persisted node_modules = %+v, want the observed resource", persisted)
	}
}
