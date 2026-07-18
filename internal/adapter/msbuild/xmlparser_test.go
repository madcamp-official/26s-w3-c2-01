package msbuild

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/scanner"
)

func TestXMLBuildProjectParser_CarriesConditionOnConditionalPropertyGroups(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Conditional.vcxproj")
	content := `<?xml version="1.0" encoding="utf-8"?>
<Project>
  <PropertyGroup Label="Globals">
    <WindowsTargetPlatformVersion>10.0.22621.0</WindowsTargetPlatformVersion>
  </PropertyGroup>
  <PropertyGroup Condition="'$(Configuration)|$(Platform)'=='Debug|x64'">
    <OutDir>bin\Debug\</OutDir>
  </PropertyGroup>
  <PropertyGroup Condition="'$(Configuration)|$(Platform)'=='Release|x64'">
    <OutDir>bin\Release\</OutDir>
  </PropertyGroup>
</Project>
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var parser BuildProjectParser = XMLBuildProjectParser{}
	got, err := parser.Parse(context.Background(), scanner.Entry{Path: path})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d parsed build projects, want 1: %+v", len(got), got)
	}

	declared := got[0].Declared
	if len(declared) != 3 {
		t.Fatalf("Declared = %+v, want 3 properties (1 unconditional + 2 conditional)", declared)
	}

	byName := map[string][]DeclaredProperty{}
	for _, d := range declared {
		byName[d.Name] = append(byName[d.Name], d)
	}

	sdk := byName["WindowsTargetPlatformVersion"]
	if len(sdk) != 1 || sdk[0].Condition != "" {
		t.Errorf("WindowsTargetPlatformVersion = %+v, want one unconditional entry", sdk)
	}

	outDirs := byName["OutDir"]
	if len(outDirs) != 2 {
		t.Fatalf("OutDir = %+v, want 2 conditional entries", outDirs)
	}
	for _, d := range outDirs {
		if d.Condition == "" {
			t.Errorf("OutDir entry %+v, want a non-empty Condition", d)
		}
	}
}
