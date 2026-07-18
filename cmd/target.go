package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/madcamp-official/26s-w3-c2-01/internal/app"
	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/pathutil"
)

// resourceTypePrefixes lists the explicit "<type>:<version>" prefixes
// accepted by explain/impact target arguments (domain.ResourceType keyed by
// its own string form).
var resourceTypePrefixes = map[string]domain.ResourceType{
	string(domain.ResourceTypeWindowsSDK):   domain.ResourceTypeWindowsSDK,
	string(domain.ResourceTypeNetFXSDK):     domain.ResourceTypeNetFXSDK,
	string(domain.ResourceTypeVisualStudio): domain.ResourceTypeVisualStudio,
	string(domain.ResourceTypeMSBuild):      domain.ResourceTypeMSBuild,
	string(domain.ResourceTypeDotNetSDK):    domain.ResourceTypeDotNetSDK,
	string(domain.ResourceTypeNodeModules):  domain.ResourceTypeNodeModules,
	string(domain.ResourceTypeBuildOutput):  domain.ResourceTypeBuildOutput,
	string(domain.ResourceTypeGlobalCache):  domain.ResourceTypeGlobalCache,
	string(domain.ResourceTypeDockerCache):  domain.ResourceTypeDockerCache,
}

// ErrTargetNotFound is returned by resolveTarget when no resource or project
// matches the given argument.
var ErrTargetNotFound = errors.New("no matching project or resource")

// ErrTargetAmbiguous is returned by resolveTarget when more than one
// resource or project matches the given argument and a more specific ID or
// path is required to disambiguate.
var ErrTargetAmbiguous = errors.New("multiple matches; give an exact ID or path")

// impactScopes is the fixed set of scopes shown for every affected project
// by both `explain` and `impact`. Only BUILD is currently judged by
// app.ImpactService (direct dependency edge -> rebuild fails); RUN and
// DEBUG are rendered as UNKNOWN rather than guessed, since evaluating them
// needs more than a declared/resolved SDK edge (see
// internal/app/impact_service.go's doc comment and the note this adds to
// docs/libra_integration_contracts.md §20.4).
var impactScopes = []domain.ImpactScope{domain.ImpactScopeRun, domain.ImpactScopeBuild, domain.ImpactScopeDebug}

type targetKind int

const (
	targetKindResource targetKind = iota
	targetKindProject
)

// target is a resolved `explain`/`impact` argument: exactly one of Resource
// or Project is populated, selected by Kind.
type target struct {
	Kind     targetKind
	Resource domain.Resource
	Project  domain.BuildProject
}

// resolveTarget identifies the resource or project a CLI argument refers to,
// per docs/libra_integration_contracts.md §21.4:
//
//	explicit "<type>:<value>" prefix -> that resource type, matched by version
//	"project:<value>" prefix         -> a project, matched by path or name
//	a path (contains a separator)    -> path search
//	anything else                    -> exact ID, then name, search
//
// Ambiguous matches are never auto-selected; the caller must narrow with an
// exact ID or path.
func resolveTarget(ctx context.Context, resources app.ResourceRepository, projects app.ProjectRepository, raw string) (target, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return target{}, fmt.Errorf("%w: empty target", ErrTargetNotFound)
	}

	if rest, ok := strings.CutPrefix(raw, "project:"); ok {
		return resolveProjectTarget(ctx, projects, strings.Trim(rest, `"`))
	}
	if idx := strings.Index(raw, ":"); idx > 0 {
		if resourceType, ok := resourceTypePrefixes[raw[:idx]]; ok {
			return resolveResourceByTypeVersion(ctx, resources, resourceType, raw[idx+1:])
		}
	}

	trimmed := strings.Trim(raw, `"`)
	if looksLikePath(trimmed) {
		return resolvePathTarget(ctx, resources, projects, trimmed)
	}
	return resolveByIDOrName(ctx, resources, projects, trimmed)
}

func looksLikePath(s string) bool {
	return strings.ContainsAny(s, `/\`)
}

func resolveResourceByTypeVersion(ctx context.Context, resources app.ResourceRepository, resourceType domain.ResourceType, version string) (target, error) {
	all, err := resources.List(ctx)
	if err != nil {
		return target{}, fmt.Errorf("list resources: %w", err)
	}
	var matches []domain.Resource
	for _, r := range all {
		if r.Type == resourceType && r.Version == version {
			matches = append(matches, r)
		}
	}
	return pickOneResource(matches, fmt.Sprintf("%s:%s", resourceType, version))
}

func resolveProjectTarget(ctx context.Context, projects app.ProjectRepository, raw string) (target, error) {
	all, err := projects.List(ctx)
	if err != nil {
		return target{}, fmt.Errorf("list projects: %w", err)
	}

	if looksLikePath(raw) {
		normalized, err := pathutil.Normalize(raw)
		if err != nil {
			return target{}, fmt.Errorf("normalize target path: %w", err)
		}
		var matches []domain.BuildProject
		for _, p := range all {
			if p.NormalizedRootPath == normalized || p.NormalizedManifestPath == normalized {
				matches = append(matches, p)
			}
		}
		return pickOneProject(matches, raw)
	}

	var exact, partial []domain.BuildProject
	for _, p := range all {
		if p.ID == raw {
			return target{Kind: targetKindProject, Project: p}, nil
		}
		if strings.EqualFold(p.Name, raw) {
			exact = append(exact, p)
		} else if strings.Contains(strings.ToLower(p.Name), strings.ToLower(raw)) {
			partial = append(partial, p)
		}
	}
	if len(exact) > 0 {
		return pickOneProject(exact, raw)
	}
	return pickOneProject(partial, raw)
}

func resolvePathTarget(ctx context.Context, resources app.ResourceRepository, projects app.ProjectRepository, raw string) (target, error) {
	normalized, err := pathutil.Normalize(raw)
	if err != nil {
		return target{}, fmt.Errorf("normalize target path: %w", err)
	}

	allResources, err := resources.List(ctx)
	if err != nil {
		return target{}, fmt.Errorf("list resources: %w", err)
	}
	var resourceMatches []domain.Resource
	for _, r := range allResources {
		if r.NormalizedPath == normalized {
			resourceMatches = append(resourceMatches, r)
		}
	}
	if len(resourceMatches) > 0 {
		return pickOneResource(resourceMatches, raw)
	}

	allProjects, err := projects.List(ctx)
	if err != nil {
		return target{}, fmt.Errorf("list projects: %w", err)
	}
	var projectMatches []domain.BuildProject
	for _, p := range allProjects {
		if p.NormalizedRootPath == normalized || p.NormalizedManifestPath == normalized {
			projectMatches = append(projectMatches, p)
		}
	}
	return pickOneProject(projectMatches, raw)
}

func resolveByIDOrName(ctx context.Context, resources app.ResourceRepository, projects app.ProjectRepository, raw string) (target, error) {
	if resource, err := resources.FindByID(ctx, raw); err == nil {
		return target{Kind: targetKindResource, Resource: resource}, nil
	}
	if project, err := projects.FindByID(ctx, raw); err == nil {
		return target{Kind: targetKindProject, Project: project}, nil
	}

	allResources, err := resources.List(ctx)
	if err != nil {
		return target{}, fmt.Errorf("list resources: %w", err)
	}
	allProjects, err := projects.List(ctx)
	if err != nil {
		return target{}, fmt.Errorf("list projects: %w", err)
	}

	var resourceMatches []domain.Resource
	for _, r := range allResources {
		if strings.EqualFold(r.Name, raw) {
			resourceMatches = append(resourceMatches, r)
		}
	}
	var projectMatches []domain.BuildProject
	for _, p := range allProjects {
		if strings.EqualFold(p.Name, raw) {
			projectMatches = append(projectMatches, p)
		}
	}

	total := len(resourceMatches) + len(projectMatches)
	if total == 0 {
		return target{}, fmt.Errorf("%w: %q", ErrTargetNotFound, raw)
	}
	if total > 1 {
		return target{}, fmt.Errorf("%w: %q matches %d items", ErrTargetAmbiguous, raw, total)
	}
	if len(resourceMatches) == 1 {
		return target{Kind: targetKindResource, Resource: resourceMatches[0]}, nil
	}
	return target{Kind: targetKindProject, Project: projectMatches[0]}, nil
}

func pickOneResource(matches []domain.Resource, raw string) (target, error) {
	switch len(matches) {
	case 0:
		return target{}, fmt.Errorf("%w: %q", ErrTargetNotFound, raw)
	case 1:
		return target{Kind: targetKindResource, Resource: matches[0]}, nil
	default:
		return target{}, fmt.Errorf("%w: %q matches %d resources", ErrTargetAmbiguous, raw, len(matches))
	}
}

func pickOneProject(matches []domain.BuildProject, raw string) (target, error) {
	switch len(matches) {
	case 0:
		return target{}, fmt.Errorf("%w: %q", ErrTargetNotFound, raw)
	case 1:
		return target{Kind: targetKindProject, Project: matches[0]}, nil
	default:
		return target{}, fmt.Errorf("%w: %q matches %d projects", ErrTargetAmbiguous, raw, len(matches))
	}
}
