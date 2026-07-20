package msbuild

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/scanner"
)

func TestFindDirectoryBuildProps_SameDirectory(t *testing.T) {
	root := t.TempDir()
	propsPath := filepath.Join(root, "Directory.Build.props")
	if err := os.WriteFile(propsPath, []byte(`<Project></Project>`), 0o644); err != nil {
		t.Fatal(err)
	}

	got, found, err := findDirectoryBuildProps(root)
	if err != nil {
		t.Fatalf("findDirectoryBuildProps() error = %v", err)
	}
	if !found || got != propsPath {
		t.Errorf("findDirectoryBuildProps() = (%q, %v), want (%q, true)", got, found, propsPath)
	}
}

func TestFindDirectoryBuildProps_WalksUpToAncestor(t *testing.T) {
	root := t.TempDir()
	propsPath := filepath.Join(root, "Directory.Build.props")
	if err := os.WriteFile(propsPath, []byte(`<Project></Project>`), 0o644); err != nil {
		t.Fatal(err)
	}
	projectDir := filepath.Join(root, "src", "GameClient")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	got, found, err := findDirectoryBuildProps(projectDir)
	if err != nil {
		t.Fatalf("findDirectoryBuildProps() error = %v", err)
	}
	if !found || got != propsPath {
		t.Errorf("findDirectoryBuildProps() = (%q, %v), want (%q, true)", got, found, propsPath)
	}
}

func TestFindDirectoryBuildProps_NoneFound(t *testing.T) {
	// An isolated temp dir with no Directory.Build.props anywhere above it up
	// to its own root is the common case for a standalone project.
	root := t.TempDir()

	_, found, err := findDirectoryBuildProps(root)
	if err != nil {
		t.Fatalf("findDirectoryBuildProps() error = %v", err)
	}
	if found {
		t.Error("findDirectoryBuildProps() found = true, want false (no props file exists)")
	}
}

func TestParseDirectoryBuildProps_TagsSourcePath(t *testing.T) {
	root := t.TempDir()
	propsPath := filepath.Join(root, "Directory.Build.props")
	content := `<Project>
  <PropertyGroup>
    <TargetFramework>net8.0</TargetFramework>
  </PropertyGroup>
</Project>`
	if err := os.WriteFile(propsPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := parseDirectoryBuildProps(propsPath)
	if err != nil {
		t.Fatalf("parseDirectoryBuildProps() error = %v", err)
	}
	want := []DeclaredProperty{{Name: "TargetFramework", Value: "net8.0", SourcePath: propsPath}}
	if len(got) != 1 || got[0] != want[0] {
		t.Errorf("parseDirectoryBuildProps() = %+v, want %+v", got, want)
	}
}

func TestMergeInheritedProperties_OwnDeclarationShadowsInherited(t *testing.T) {
	own := []DeclaredProperty{{Name: "TargetFramework", Value: "net8.0", SourcePath: "project.csproj"}}
	inherited := []DeclaredProperty{
		{Name: "TargetFramework", Value: "net6.0", SourcePath: "Directory.Build.props"},
		{Name: "Company", Value: "Acme", SourcePath: "Directory.Build.props"},
	}

	got := mergeInheritedProperties(own, inherited)

	if len(got) != 2 {
		t.Fatalf("merged = %+v, want 2 properties (inherited TargetFramework shadowed)", got)
	}
	if got[0].Name != "Company" || got[0].Value != "Acme" {
		t.Errorf("merged[0] = %+v, want the surviving inherited Company property", got[0])
	}
	if got[1] != own[0] {
		t.Errorf("merged[1] = %+v, want the project's own TargetFramework = net8.0", got[1])
	}
}

// TestXMLBuildProjectParser_InheritedPropertyOverriddenByProjectItself is an
// end-to-end check that Parse resolves the same override rule
// mergeInheritedProperties implements: when both the project file and its
// Directory.Build.props declare WindowsTargetPlatformVersion, only the
// project's own value survives (so ResolveDependencies can't produce two
// conflicting Dependency edges for the same property).
func TestXMLBuildProjectParser_InheritedPropertyOverriddenByProjectItself(t *testing.T) {
	root := t.TempDir()
	propsContent := `<Project>
  <PropertyGroup>
    <WindowsTargetPlatformVersion>10.0.19041.0</WindowsTargetPlatformVersion>
    <Company>Acme</Company>
  </PropertyGroup>
</Project>`
	if err := os.WriteFile(filepath.Join(root, "Directory.Build.props"), []byte(propsContent), 0o644); err != nil {
		t.Fatal(err)
	}

	projectPath := filepath.Join(root, "Game.vcxproj")
	projectContent := `<?xml version="1.0" encoding="utf-8"?>
<Project>
  <PropertyGroup Label="Globals">
    <WindowsTargetPlatformVersion>10.0.22621.0</WindowsTargetPlatformVersion>
  </PropertyGroup>
</Project>
`
	if err := os.WriteFile(projectPath, []byte(projectContent), 0o644); err != nil {
		t.Fatal(err)
	}

	var parser BuildProjectParser = XMLBuildProjectParser{}
	got, err := parser.Parse(context.Background(), scanner.Entry{Path: projectPath})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d parsed build projects, want 1: %+v", len(got), got)
	}

	declared := got[0].Declared
	byName := map[string]DeclaredProperty{}
	for _, d := range declared {
		byName[d.Name] = d
	}

	sdk, ok := byName["WindowsTargetPlatformVersion"]
	if !ok || sdk.Value != "10.0.22621.0" || sdk.SourcePath != projectPath {
		t.Errorf("WindowsTargetPlatformVersion = %+v, want the project's own 10.0.22621.0 from %q", sdk, projectPath)
	}

	company, ok := byName["Company"]
	if !ok || company.Value != "Acme" {
		t.Errorf("Company = %+v, want the inherited value from Directory.Build.props", company)
	}
}
