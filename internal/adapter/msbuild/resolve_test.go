package msbuild

import (
	"testing"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

func installedSDKs(versions ...string) []domain.Resource {
	resources := make([]domain.Resource, len(versions))
	for i, v := range versions {
		resources[i] = domain.Resource{
			ID:      "sdk-" + v,
			Type:    domain.ResourceTypeWindowsSDK,
			Version: v,
		}
	}
	return resources
}

func installedDotNetSDKs(versions ...string) []domain.Resource {
	resources := make([]domain.Resource, len(versions))
	for i, v := range versions {
		resources[i] = domain.Resource{
			ID:      "dotnet-sdk-" + v,
			Type:    domain.ResourceTypeDotNetSDK,
			Version: v,
		}
	}
	return resources
}

func TestMatchWindowsSDK(t *testing.T) {
	cases := []struct {
		name        string
		declared    string
		installed   []domain.Resource
		wantVersion string
		wantKind    domain.EvidenceKind
		wantOK      bool
	}{
		{
			name:        "exact version match",
			declared:    "10.0.22621.0",
			installed:   installedSDKs("10.0.19041.0", "10.0.22621.0"),
			wantVersion: "10.0.22621.0",
			wantKind:    domain.EvidenceDeclared,
			wantOK:      true,
		},
		{
			name:        "major.minor prefix picks the highest matching build",
			declared:    "10.0",
			installed:   installedSDKs("10.0.19041.0", "10.0.22621.0"),
			wantVersion: "10.0.22621.0",
			wantKind:    domain.EvidenceResolved,
			wantOK:      true,
		},
		{
			name:        "Latest picks the highest version overall",
			declared:    "Latest",
			installed:   installedSDKs("8.1", "10.0.19041.0", "10.0.22621.0"),
			wantVersion: "10.0.22621.0",
			wantKind:    domain.EvidenceResolved,
			wantOK:      true,
		},
		{
			name:      "declared SDK is not installed",
			declared:  "11.0",
			installed: installedSDKs("10.0.19041.0", "10.0.22621.0"),
			wantOK:    false,
		},
		{
			name:      "no windows SDKs installed at all",
			declared:  "10.0",
			installed: nil,
			wantOK:    false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, kind, ok := MatchWindowsSDK(tc.declared, tc.installed)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if got.Version != tc.wantVersion {
				t.Errorf("Version = %q, want %q", got.Version, tc.wantVersion)
			}
			if kind != tc.wantKind {
				t.Errorf("Kind = %v, want %v", kind, tc.wantKind)
			}
		})
	}
}

func TestResolveWindowsSDKDependency(t *testing.T) {
	installed := installedSDKs("10.0.19041.0", "10.0.22621.0")
	installed[1].ID = "resource-10.0.22621.0"
	declared := DeclaredProperty{Name: "WindowsTargetPlatformVersion", Value: "10.0"}
	collectedAt := time.Date(2026, 7, 19, 3, 0, 0, 0, time.UTC)

	dependency, evidence, ok := ResolveWindowsSDKDependency("project-1", `D:\Projects\Game\Game.vcxproj`, declared, installed, collectedAt)
	if !ok {
		t.Fatal("ResolveWindowsSDKDependency() ok = false, want true")
	}

	if dependency.SourceType != domain.NodeProject || dependency.SourceID != "project-1" {
		t.Errorf("dependency source = %v/%v, want PROJECT/project-1", dependency.SourceType, dependency.SourceID)
	}
	if dependency.TargetType != domain.NodeResource || dependency.TargetID != "resource-10.0.22621.0" {
		t.Errorf("dependency target = %v/%v, want RESOURCE/resource-10.0.22621.0", dependency.TargetType, dependency.TargetID)
	}
	if dependency.Relation != domain.RelationRequires {
		t.Errorf("dependency relation = %v, want REQUIRES", dependency.Relation)
	}
	wantDependencyID := domain.DependencyID(dependency.SourceType, dependency.SourceID, dependency.Relation, dependency.TargetType, dependency.TargetID)
	if dependency.ID != wantDependencyID {
		t.Errorf("dependency.ID = %q, want %q", dependency.ID, wantDependencyID)
	}

	if evidence.DependencyID != dependency.ID {
		t.Errorf("evidence.DependencyID = %q, want %q", evidence.DependencyID, dependency.ID)
	}
	if evidence.Kind != domain.EvidenceResolved {
		t.Errorf("evidence.Kind = %v, want RESOLVED", evidence.Kind)
	}
	if evidence.RawValue != "10.0" || evidence.ResolvedValue != "10.0.22621.0" {
		t.Errorf("evidence raw/resolved = %q/%q, want 10.0/10.0.22621.0", evidence.RawValue, evidence.ResolvedValue)
	}
	if !evidence.CollectedAt.Equal(collectedAt) {
		t.Errorf("evidence.CollectedAt = %v, want %v", evidence.CollectedAt, collectedAt)
	}
}

func TestResolveWindowsSDKDependency_NoMatch(t *testing.T) {
	installed := installedSDKs("10.0.19041.0")
	declared := DeclaredProperty{Name: "WindowsTargetPlatformVersion", Value: "11.0"}

	_, _, ok := ResolveWindowsSDKDependency("project-1", `D:\Projects\Game\Game.vcxproj`, declared, installed, time.Now())
	if ok {
		t.Fatal("ResolveWindowsSDKDependency() ok = true, want false (no installed SDK matches)")
	}
}

func TestParseTargetFramework(t *testing.T) {
	cases := []struct {
		tfm        string
		wantPrefix string
		wantOK     bool
	}{
		{"net8.0", "8.0", true},
		{"net8.0-windows", "8.0", true},
		{"net6.0", "6.0", true},
		{"net472", "", false},         // legacy .NET Framework moniker, no dot
		{"netstandard2.0", "", false}, // not an SDK version at all
		{"netcoreapp3.1", "", false},  // legacy moniker
		{"", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.tfm, func(t *testing.T) {
			gotPrefix, ok := ParseTargetFramework(tc.tfm)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if ok && gotPrefix != tc.wantPrefix {
				t.Errorf("prefix = %q, want %q", gotPrefix, tc.wantPrefix)
			}
		})
	}
}

func TestMatchDotNetSDK(t *testing.T) {
	installed := installedDotNetSDKs("6.0.428", "8.0.404")

	got, kind, ok := MatchDotNetSDK("net8.0", installed)
	if !ok {
		t.Fatal("MatchDotNetSDK() ok = false, want true")
	}
	if got.Version != "8.0.404" {
		t.Errorf("Version = %q, want %q", got.Version, "8.0.404")
	}
	if kind != domain.EvidenceResolved {
		t.Errorf("Kind = %v, want RESOLVED", kind)
	}

	if _, _, ok := MatchDotNetSDK("net9.0", installed); ok {
		t.Error("MatchDotNetSDK(net9.0) ok = true, want false (no installed 9.0 SDK)")
	}
	if _, _, ok := MatchDotNetSDK("netstandard2.0", installed); ok {
		t.Error("MatchDotNetSDK(netstandard2.0) ok = true, want false (not an SDK moniker)")
	}
}

func TestResolveDependencies(t *testing.T) {
	installed := append(installedSDKs("10.0.22621.0"), installedDotNetSDKs("8.0.404")...)
	declared := []DeclaredProperty{
		{Name: "WindowsTargetPlatformVersion", Value: "10.0"},
		{Name: "TargetFramework", Value: "net8.0"},
		{Name: "PlatformToolset", Value: "v143"}, // unrecognized, should be skipped
	}
	collectedAt := time.Date(2026, 7, 19, 3, 0, 0, 0, time.UTC)

	got := ResolveDependencies("project-1", `D:\Projects\Game\Game.vcxproj`, declared, installed, collectedAt)
	if len(got) != 2 {
		t.Fatalf("got %d resolved dependencies, want 2: %+v", len(got), got)
	}

	byProperty := map[string]ResolvedDependency{}
	for _, rd := range got {
		byProperty[rd.Evidence[0].Property] = rd
	}
	if rd, ok := byProperty["WindowsTargetPlatformVersion"]; !ok || rd.Evidence[0].ResolvedValue != "10.0.22621.0" {
		t.Errorf("WindowsTargetPlatformVersion resolution = %+v", rd)
	}
	if rd, ok := byProperty["TargetFramework"]; !ok || rd.Evidence[0].ResolvedValue != "8.0.404" {
		t.Errorf("TargetFramework resolution = %+v", rd)
	}
}
