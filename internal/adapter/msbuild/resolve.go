package msbuild

import (
	"strings"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

// MatchWindowsSDK finds the installed Windows SDK a project's declared
// WindowsTargetPlatformVersion resolves to, per the version matching rule:
//
//   - a full version ("10.0.22621.0"): matched only against an installed SDK
//     with that exact version -> EvidenceDeclared
//   - a Major.Minor prefix ("10.0") or "Latest": resolved to the highest
//     installed version sharing that prefix (or highest overall for
//     "Latest") -> EvidenceResolved
//
// ok is false if declared cannot be parsed as a version (and isn't "Latest"),
// or if no installed resource matches.
func MatchWindowsSDK(declared string, installed []domain.Resource) (resource domain.Resource, kind domain.EvidenceKind, ok bool) {
	candidates := resourcesOfType(installed, domain.ResourceTypeWindowsSDK)
	if len(candidates) == 0 {
		return domain.Resource{}, "", false
	}

	if declared == "Latest" {
		best, found := highestVersion(candidates)
		if !found {
			return domain.Resource{}, "", false
		}
		return best, domain.EvidenceResolved, true
	}

	declaredSegments, isVersion := parseVersion(declared)
	if !isVersion {
		return domain.Resource{}, "", false
	}

	var prefixMatches []domain.Resource
	for _, c := range candidates {
		if c.Version == declared {
			return c, domain.EvidenceDeclared, true
		}
		if segments, ok := parseVersion(c.Version); ok && hasVersionPrefix(segments, declaredSegments) {
			prefixMatches = append(prefixMatches, c)
		}
	}
	return bestMatch(prefixMatches)
}

// ParseTargetFramework extracts the SDK version prefix (e.g. "8.0") from a
// modern .NET TargetFramework moniker (e.g. "net8.0", "net8.0-windows"). It
// deliberately rejects legacy monikers with no dot (net472, .NET Framework)
// and non-numeric monikers (netstandard2.0, netcoreapp3.1), since neither
// corresponds to a .NET (Core) SDK version the same way "net8.0" does.
func ParseTargetFramework(tfm string) (string, bool) {
	rest, hasPrefix := strings.CutPrefix(tfm, "net")
	if !hasPrefix {
		return "", false
	}
	if dash := strings.Index(rest, "-"); dash >= 0 {
		rest = rest[:dash]
	}
	if !strings.Contains(rest, ".") {
		return "", false
	}
	if _, ok := parseVersion(rest); !ok {
		return "", false
	}
	return rest, true
}

// MatchDotNetSDK finds the installed .NET SDK a project's declared
// TargetFramework resolves to: the highest installed SDK version sharing the
// TargetFramework's Major.Minor prefix. Unlike MatchWindowsSDK, this is
// always a resolution (never an exact string match), since a moniker like
// "net8.0" never equals an SDK version like "8.0.404" -> EvidenceResolved.
func MatchDotNetSDK(declaredTFM string, installed []domain.Resource) (resource domain.Resource, kind domain.EvidenceKind, ok bool) {
	prefix, isTFM := ParseTargetFramework(declaredTFM)
	if !isTFM {
		return domain.Resource{}, "", false
	}
	prefixSegments, _ := parseVersion(prefix)

	candidates := resourcesOfType(installed, domain.ResourceTypeDotNetSDK)
	var prefixMatches []domain.Resource
	for _, c := range candidates {
		if segments, ok := parseVersion(c.Version); ok && hasVersionPrefix(segments, prefixSegments) {
			prefixMatches = append(prefixMatches, c)
		}
	}
	return bestMatch(prefixMatches)
}

func resourcesOfType(resources []domain.Resource, resourceType domain.ResourceType) []domain.Resource {
	var matches []domain.Resource
	for _, r := range resources {
		if r.Type == resourceType {
			matches = append(matches, r)
		}
	}
	return matches
}

// bestMatch picks the highest version among candidates, reporting it as an
// EvidenceResolved match.
func bestMatch(candidates []domain.Resource) (domain.Resource, domain.EvidenceKind, bool) {
	if len(candidates) == 0 {
		return domain.Resource{}, "", false
	}
	best, found := highestVersion(candidates)
	if !found {
		return domain.Resource{}, "", false
	}
	return best, domain.EvidenceResolved, true
}

// highestVersion returns the candidate with the numerically greatest
// version. Candidates whose version doesn't parse are skipped.
func highestVersion(candidates []domain.Resource) (domain.Resource, bool) {
	var best domain.Resource
	var bestSegments []int
	found := false
	for _, c := range candidates {
		segments, ok := parseVersion(c.Version)
		if !ok {
			continue
		}
		if !found || compareVersions(segments, bestSegments) > 0 {
			best, bestSegments, found = c, segments, true
		}
	}
	return best, found
}

// resolveDependency builds the Dependency and Evidence linking projectID to
// whatever match finds among installed for declared, if any. sourcePath is
// the project file the declaration came from (e.g. a .vcxproj path).
func resolveDependency(
	projectID string,
	sourcePath string,
	declared DeclaredProperty,
	installed []domain.Resource,
	collectedAt time.Time,
	match func(string, []domain.Resource) (domain.Resource, domain.EvidenceKind, bool),
) (domain.Dependency, domain.Evidence, bool) {
	resource, kind, ok := match(declared.Value, installed)
	if !ok {
		return domain.Dependency{}, domain.Evidence{}, false
	}

	dependency := domain.Dependency{
		SourceType: domain.NodeProject,
		SourceID:   projectID,
		TargetType: domain.NodeResource,
		TargetID:   resource.ID,
		Relation:   domain.RelationRequires,
		Confidence: domain.DefaultConfidence[kind],
	}
	dependency.ID = domain.DependencyID(dependency.SourceType, dependency.SourceID, dependency.Relation, dependency.TargetType, dependency.TargetID)

	evidence := domain.Evidence{
		DependencyID:  dependency.ID,
		Kind:          kind,
		SourcePath:    sourcePath,
		Property:      declared.Name,
		RawValue:      declared.Value,
		ResolvedValue: resource.Version,
		CollectedAt:   collectedAt,
	}
	evidence.ID = domain.EvidenceID(evidence.DependencyID, evidence.Kind, evidence.SourcePath, evidence.Property, evidence.RawValue, evidence.ResolvedValue)

	return dependency, evidence, true
}

// ResolveWindowsSDKDependency builds the Dependency and Evidence linking
// projectID to the Windows SDK its WindowsTargetPlatformVersion declaration
// resolves to, if a match is found among installed.
func ResolveWindowsSDKDependency(
	projectID string,
	sourcePath string,
	declared DeclaredProperty,
	installed []domain.Resource,
	collectedAt time.Time,
) (domain.Dependency, domain.Evidence, bool) {
	return resolveDependency(projectID, sourcePath, declared, installed, collectedAt, MatchWindowsSDK)
}

// ResolveDotNetSDKDependency builds the Dependency and Evidence linking
// projectID to the .NET SDK its TargetFramework declaration resolves to, if
// a match is found among installed.
func ResolveDotNetSDKDependency(
	projectID string,
	sourcePath string,
	declared DeclaredProperty,
	installed []domain.Resource,
	collectedAt time.Time,
) (domain.Dependency, domain.Evidence, bool) {
	return resolveDependency(projectID, sourcePath, declared, installed, collectedAt, MatchDotNetSDK)
}

// ResolvedDependency pairs a Dependency edge with the Evidence backing it,
// ready to hand to app.DependencyRepository.UpsertGraph.
type ResolvedDependency struct {
	Dependency domain.Dependency
	Evidence   []domain.Evidence
}

// ResolveDependencies matches every recognized declared property --
// currently WindowsTargetPlatformVersion and TargetFramework -- against
// installed resources, returning one ResolvedDependency per successful
// match.
//
// A recognized property gated by a Configuration/Platform Condition is not
// matched at all: evaluating the Condition isn't implemented, and matching
// it unconditionally would silently prefer one configuration over another.
// It is reported as an UnverifiedScope instead, so callers can distinguish
// "checked, no dependency" from "not checked" (see
// docs/libra_integration_contracts.md §19.1).
//
// A recognized, unconditional property with no matching installed resource,
// and any unrecognized property name, are silently skipped: neither
// represents a scope of analysis libra didn't attempt.
func ResolveDependencies(
	projectID string,
	sourcePath string,
	declared []DeclaredProperty,
	installed []domain.Resource,
	collectedAt time.Time,
) (resolved []ResolvedDependency, unverified []domain.UnverifiedScope) {
	for _, d := range declared {
		if !isRecognizedProperty(d.Name) {
			continue
		}
		if d.Condition != "" {
			unverified = append(unverified, domain.UnverifiedScope{
				BuildProjectID: projectID,
				Source:         sourcePath,
				Property:       d.Name,
				RawValue:       d.Value,
				Condition:      d.Condition,
				Reason:         "MSBUILD_CONDITION_NOT_EVALUATED",
			})
			continue
		}

		var dependency domain.Dependency
		var evidence domain.Evidence
		var ok bool
		switch d.Name {
		case "WindowsTargetPlatformVersion":
			dependency, evidence, ok = ResolveWindowsSDKDependency(projectID, sourcePath, d, installed, collectedAt)
		case "TargetFramework":
			dependency, evidence, ok = ResolveDotNetSDKDependency(projectID, sourcePath, d, installed, collectedAt)
		}
		if !ok {
			continue
		}
		resolved = append(resolved, ResolvedDependency{Dependency: dependency, Evidence: []domain.Evidence{evidence}})
	}
	return resolved, unverified
}

func isRecognizedProperty(name string) bool {
	switch name {
	case "WindowsTargetPlatformVersion", "TargetFramework":
		return true
	default:
		return false
	}
}
