package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/pathutil"
)

// PlanOptions selects the scope and target size for PlanService.Build.
type PlanOptions struct {
	// TargetBytes is the amount of space the plan should try to reclaim.
	// Zero means unlimited: every SAFE candidate in scope is selected.
	TargetBytes int64
	// ProjectRootNormalized restricts candidates to resources located at or
	// under this normalized path. Empty means no restriction.
	ProjectRootNormalized string
}

// PlanResult is everything a caller needs to both persist and display a
// planning run: the storable snapshot (SAFE candidates only, per
// docs/libra_integration_contracts.md §23.1) plus the REVIEW/BLOCKED
// candidates that were considered but not selected, so a CLI command can
// show the user why.
type PlanResult struct {
	Plan    domain.CleanupPlan
	Review  []domain.Resource
	Blocked []domain.Resource
}

// PlanService selects cleanup candidates for `libra plan`. It does not
// persist anything -- the caller (cmd/plan.go) owns calling
// CleanupPlanRepository.Create with the returned Plan, matching this
// package's app-decides/store-persists split (see risk_policy.go).
type PlanService struct {
	resources ResourceRepository
	projects  ProjectRepository
	scans     ScanRepository
	now       func() time.Time
}

func NewPlanService(resources ResourceRepository, projects ProjectRepository, scans ScanRepository) *PlanService {
	return &PlanService{resources: resources, projects: projects, scans: scans, now: time.Now}
}

// Build implements the greedy candidate-selection algorithm confirmed in
// docs/libra_integration_contracts.md §23.1:
//
//  1. BLOCKED resources are excluded entirely.
//  2. Only SAFE resources are auto-selected; REVIEW is never included by
//     default.
//  3. SAFE candidates are ordered by Confidence desc, then ReclaimableSize
//     desc, then stable Resource ID asc.
//  4. Candidates are accepted in that order until TargetBytes is met (or
//     all are taken, if TargetBytes is 0/unlimited).
func (s *PlanService) Build(ctx context.Context, opts PlanOptions) (PlanResult, error) {
	scan, err := s.scans.FindLatest(ctx)
	if err != nil {
		return PlanResult{}, fmt.Errorf("find latest scan: %w", err)
	}

	all, err := s.resources.List(ctx)
	if err != nil {
		return PlanResult{}, fmt.Errorf("list resources: %w", err)
	}
	if opts.ProjectRootNormalized != "" {
		scoped := make([]domain.Resource, 0, len(all))
		for _, r := range all {
			within, err := pathutil.IsSameOrChild(r.NormalizedPath, opts.ProjectRootNormalized)
			if err != nil {
				return PlanResult{}, fmt.Errorf("match resource %q against project scope: %w", r.ID, err)
			}
			if within {
				scoped = append(scoped, r)
			}
		}
		all = scoped
	}

	var safe, review, blocked []domain.Resource
	for _, r := range all {
		switch r.Risk {
		case domain.RiskSafe:
			safe = append(safe, r)
		case domain.RiskReview:
			review = append(review, r)
		default:
			blocked = append(blocked, r)
		}
	}
	sort.Slice(safe, func(i, j int) bool {
		if safe[i].Confidence != safe[j].Confidence {
			return safe[i].Confidence > safe[j].Confidence
		}
		if safe[i].ReclaimableSize != safe[j].ReclaimableSize {
			return safe[i].ReclaimableSize > safe[j].ReclaimableSize
		}
		return safe[i].ID < safe[j].ID
	})

	projects, err := s.projects.List(ctx)
	if err != nil {
		return PlanResult{}, fmt.Errorf("list projects: %w", err)
	}

	planID, err := newPlanID(s.now)
	if err != nil {
		return PlanResult{}, fmt.Errorf("generate plan ID: %w", err)
	}
	plan := domain.CleanupPlan{
		ID:          planID,
		CreatedAt:   s.now().UTC(),
		TargetBytes: opts.TargetBytes,
	}
	var selected int64
	for _, r := range safe {
		if opts.TargetBytes > 0 && selected >= opts.TargetBytes {
			break
		}
		plan.Items = append(plan.Items, domain.CleanupPlanItem{
			ID:                   planID + ":" + r.ID,
			ResourceID:           r.ID,
			NormalizedPath:       r.NormalizedPath,
			ExpectedType:         r.Type,
			ExpectedSize:         r.ReclaimableSize,
			ExpectedModifiedTime: expectedModifiedTime(r),
			RiskAtPlanning:       r.Risk,
			ConfidenceAtPlanning: r.Confidence,
			OwnerProjectID:       closestOwningProjectID(r, projects),
			ScanID:               scan.ID,
		})
		selected += r.ReclaimableSize
	}
	plan.SelectedBytes = selected
	plan.Status = domain.CleanupPlanReady
	if opts.TargetBytes > 0 && selected < opts.TargetBytes {
		plan.Status = domain.CleanupPlanInsufficientCandidates
	}

	return PlanResult{Plan: plan, Review: review, Blocked: blocked}, nil
}

// newPlanID builds a plan ID from the current time plus a short random
// suffix. The timestamp alone is only second-granular, so two plans built
// in the same second (e.g. re-running `libra plan` with different flags to
// compare options) would otherwise collide against cleanup_plans' unique ID
// constraint; the random suffix makes that practically impossible without
// requiring a database round-trip to check for collisions first.
func newPlanID(now func() time.Time) (string, error) {
	suffix := make([]byte, 4)
	if _, err := rand.Read(suffix); err != nil {
		return "", err
	}
	return fmt.Sprintf("plan-%s-%s", now().UTC().Format("20060102-150405"), hex.EncodeToString(suffix)), nil
}

// expectedModifiedTime falls back to LastObservedAt when a resource has no
// known modification time (domain.Resource.LastModifiedAt is nil), since
// CleanupPlanItem.ExpectedModifiedTime is a required snapshot field.
func expectedModifiedTime(r domain.Resource) time.Time {
	if r.LastModifiedAt != nil {
		return *r.LastModifiedAt
	}
	return r.LastObservedAt
}

// closestOwningProjectID returns the ID of the project whose root most
// tightly contains the resource, or "" if none does. This is a path-prefix
// stand-in for the PROJECT -> RESOURCE OWNS graph edge that
// docs/libra_integration_contracts.md §23.1 describes but that isn't
// persisted yet (still DECISION_REQUIRED, blocked on stable BuildProject ID
// work). A malformed candidate path is treated as "does not own" rather
// than failing the whole plan, since this is a best-effort display field,
// unlike the user-supplied --project scope filter above.
func closestOwningProjectID(r domain.Resource, projects []domain.BuildProject) string {
	var ownerID string
	bestRootLen := -1
	for _, p := range projects {
		within, err := pathutil.IsSameOrChild(r.NormalizedPath, p.NormalizedRootPath)
		if err != nil || !within {
			continue
		}
		if len(p.NormalizedRootPath) > bestRootLen {
			ownerID = p.ID
			bestRootLen = len(p.NormalizedRootPath)
		}
	}
	return ownerID
}
