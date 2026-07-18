package msbuild

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/scanner"
)

func TestXMLBuildProjectParser_SkipsConditionalPropertyGroups(t *testing.T) {
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
	if len(declared) != 1 {
		t.Fatalf("Declared = %+v, want exactly the one unconditional property", declared)
	}
	if declared[0].Name != "WindowsTargetPlatformVersion" || declared[0].Value != "10.0.22621.0" {
		t.Errorf("Declared[0] = %+v, want WindowsTargetPlatformVersion=10.0.22621.0", declared[0])
	}
	for _, d := range declared {
		if d.Name == "OutDir" {
			t.Errorf("Declared contains conditional property %+v, want it skipped", d)
		}
	}
}
