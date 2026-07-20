package cmd

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/pathutil"
	"github.com/madcamp-official/26s-w3-c2-01/internal/store/sqlite"
)

// seedWindowsSDKDependency wires a synthetic PROJECT -> RESOURCE dependency
// edge (with DECLARED evidence) directly into the database, standing in for
// the msbuild DependencyAnalyzer that scan orchestration does not wire up
// yet (see docs/libra_integration_contracts.md's note on this and the
// tracking issue referenced from the impact/explain command PRs). It lets
// explain/impact command tests exercise real evidence and impact rendering
// without depending on that still-open cross-team gap.
//
// It returns the seeded resource and the project it attaches to (found by
// name among whatever `scan` already persisted).
func seedWindowsSDKDependency(t *testing.T, projectName string) (domain.Resource, domain.BuildProject) {
	t.Helper()

	db, err := openDatabase()
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	ctx := context.Background()

	projects, err := sqlite.NewProjectRepository(db).List(ctx)
	if err != nil {
		t.Fatalf("list projects: %v", err)
	}
	var project domain.BuildProject
	for _, p := range projects {
		if p.Name == projectName {
			project = p
			break
		}
	}
	if project.ID == "" {
		t.Fatalf("project %q not found among scanned projects", projectName)
	}

	displayPath := filepath.Join(t.TempDir(), "WindowsKits10")
	normalizedPath, err := pathutil.Normalize(displayPath)
	if err != nil {
		t.Fatalf("normalize resource path: %v", err)
	}
	resource := domain.Resource{
		Type: domain.ResourceTypeWindowsSDK, Name: "Windows SDK", Version: "10.0.22621.0",
		DisplayPath: displayPath, NormalizedPath: normalizedPath,
		LogicalSize: 1024, SizeKnown: true, Regenerable: false, SystemManaged: false,
		LastObservedAt: time.Now().UTC(), Risk: domain.RiskBlocked, Confidence: 75,
	}
	resource.ID = domain.ResourceID(resource.Type, resource.Version, resource.NormalizedPath)
	if err := sqlite.NewResourceRepository(db).Upsert(ctx, resource); err != nil {
		t.Fatalf("seed resource: %v", err)
	}

	dependency := domain.Dependency{
		SourceType: domain.NodeProject, SourceID: project.ID,
		TargetType: domain.NodeResource, TargetID: resource.ID,
		Relation: domain.RelationRequires, Confidence: 75,
	}
	dependency.ID = domain.DependencyID(dependency.SourceType, dependency.SourceID, dependency.Relation, dependency.TargetType, dependency.TargetID)

	evidence := domain.Evidence{
		DependencyID: dependency.ID, Kind: domain.EvidenceDeclared,
		SourcePath: project.ManifestPath, Property: "WindowsTargetPlatformVersion",
		RawValue: "10.0.22621.0", ResolvedValue: "10.0.22621.0", CollectedAt: time.Now().UTC(),
	}
	evidence.ID = domain.EvidenceID(evidence.DependencyID, evidence.Kind, evidence.SourcePath,
		evidence.Property, evidence.RawValue, evidence.ResolvedValue)

	// evidence.scan_id has a foreign key to scans(id), so reuse the scan row
	// that the test's own `scan` run already created instead of a made-up ID.
	var scanID string
	if err := db.QueryRowContext(ctx, `SELECT id FROM scans ORDER BY started_at DESC LIMIT 1`).Scan(&scanID); err != nil {
		t.Fatalf("find latest scan id: %v", err)
	}

	if err := sqlite.NewDependencyRepository(db).UpsertGraph(ctx, scanID, dependency, []domain.Evidence{evidence}); err != nil {
		t.Fatalf("seed dependency graph: %v", err)
	}

	return resource, project
}

// seedSafeResource inserts a resource directly with Risk=SAFE, standing in
// for a fully verified CleanupEvidence result that no detector produces
// yet: real Node/MSBuild artifact detectors only fill 2 of the 4
// CleanupEvidence flags today (see docs/libra_integration_contracts.md
// §19.3), so a real `libra scan` never yields a SAFE resource. This lets
// plan/clean command tests exercise the SAFE selection and dry-run preview
// paths without waiting on that adapter work.
func seedSafeResource(t *testing.T, name string, reclaimableSize int64) domain.Resource {
	t.Helper()

	db, err := openDatabase()
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	ctx := context.Background()

	displayPath := filepath.Join(t.TempDir(), name)
	normalizedPath, err := pathutil.Normalize(displayPath)
	if err != nil {
		t.Fatalf("normalize resource path: %v", err)
	}
	resource := domain.Resource{
		Type: domain.ResourceTypeNodeModules, Name: name,
		DisplayPath: displayPath, NormalizedPath: normalizedPath,
		LogicalSize: reclaimableSize, SizeKnown: true, ReclaimableSize: reclaimableSize,
		Regenerable: true, SystemManaged: false,
		LastObservedAt: time.Now().UTC(), Risk: domain.RiskSafe, Confidence: 90,
	}
	resource.ID = domain.ResourceID(resource.Type, resource.Version, resource.NormalizedPath)
	if err := sqlite.NewResourceRepository(db).Upsert(ctx, resource); err != nil {
		t.Fatalf("seed resource: %v", err)
	}
	return resource
}
