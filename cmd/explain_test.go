package cmd

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestExplainCommandDescribesResourceWithEvidence(t *testing.T) {
	scanRoot = ""
	cfgPath = ""

	fixture, err := filepath.Abs("../testdata/msbuild")
	if err != nil {
		t.Fatalf("resolve fixture path: %v", err)
	}
	t.Chdir(t.TempDir())

	run := func(args ...string) *bytes.Buffer {
		t.Helper()
		out := &bytes.Buffer{}
		rootCmd.SetOut(out)
		rootCmd.SetErr(out)
		rootCmd.SetArgs(args)
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("Execute(%v) error = %v; output=%s", args, err, out)
		}
		return out
	}

	run("init")
	run("scan", "--root", fixture)
	seedWindowsSDKDependency(t, "GameClient")

	out := run("explain", "windows-sdk:10.0.22621.0")
	for _, want := range []string{
		"Resource: Windows SDK 10.0.22621.0",
		"GameClient",
		"Evidence: DECLARED",
		"Property: WindowsTargetPlatformVersion",
		"Rebuild: HIGH",
		"Visual Studio debugging: HIGH",
		"Risk: BLOCKED",
		"Confidence: 75%",
		"Recovery:",
	} {
		if !bytes.Contains(out.Bytes(), []byte(want)) {
			t.Fatalf("explain output missing %q:\n%s", want, out)
		}
	}
}

func TestExplainCommandDescribesProject(t *testing.T) {
	scanRoot = ""
	cfgPath = ""

	fixture, err := filepath.Abs("../testdata/msbuild")
	if err != nil {
		t.Fatalf("resolve fixture path: %v", err)
	}
	t.Chdir(t.TempDir())

	run := func(args ...string) *bytes.Buffer {
		t.Helper()
		out := &bytes.Buffer{}
		rootCmd.SetOut(out)
		rootCmd.SetErr(out)
		rootCmd.SetArgs(args)
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("Execute(%v) error = %v; output=%s", args, err, out)
		}
		return out
	}

	run("init")
	run("scan", "--root", fixture)
	_, project := seedWindowsSDKDependency(t, "GameClient")

	out := run("explain", "project:"+project.RootPath)
	for _, want := range []string{
		"Project: GameClient",
		"Uses:",
		"  Requires:",
		"Windows SDK",
		"Evidence: DECLARED",
		"Property: WindowsTargetPlatformVersion",
	} {
		if !bytes.Contains(out.Bytes(), []byte(want)) {
			t.Fatalf("explain project output missing %q:\n%s", want, out)
		}
	}

	// issue #38: project size is now measured (see
	// internal/app/analysis_orchestrator.go), so the line must show a real
	// humanized value instead of the old "—" placeholder.
	if bytes.Contains(out.Bytes(), []byte("Size: —")) {
		t.Fatalf("explain project output must not render the unmeasured-size placeholder:\n%s", out)
	}
}

// TestExplainCommandShowsImpactForOwnedResource covers a project-owned
// resource (RelationOwns), not the windows-sdk RelationRequires case the
// other tests here use. Before internal/app/impact_service.go learned to
// judge RelationOwns edges, "Expected impact" was unconditionally UNKNOWN
// for every OWNS resource -- node_modules, Pods, bin/obj/dist -- which is
// most of what a macOS/Node user ever explains.
func TestExplainCommandShowsImpactForOwnedResource(t *testing.T) {
	scanRoot = ""
	cfgPath = ""

	fixture, err := filepath.Abs("../testdata/node")
	if err != nil {
		t.Fatalf("resolve fixture path: %v", err)
	}
	t.Chdir(t.TempDir())

	run := func(args ...string) *bytes.Buffer {
		t.Helper()
		out := &bytes.Buffer{}
		rootCmd.SetOut(out)
		rootCmd.SetErr(out)
		rootCmd.SetArgs(args)
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("Execute(%v) error = %v; output=%s", args, err, out)
		}
		return out
	}

	run("init")
	run("scan", "--root", fixture)

	out := run("explain", filepath.Join(fixture, "basic", "node_modules"))
	for _, want := range []string{
		"Existing executable launch: LOW",
		"Rebuild: HIGH",
		// Not "Visual Studio debugging" -- node_modules isn't a Visual
		// Studio/MSBuild resource, so the DEBUG label must stay neutral.
		"IDE debugging: HIGH",
	} {
		if !bytes.Contains(out.Bytes(), []byte(want)) {
			t.Fatalf("explain output missing %q (owned node_modules should get a real impact judgment, not UNKNOWN):\n%s", want, out)
		}
	}
}

// TestExplainCommandLabelsDebugByEcosystem locks down that the DEBUG scope's
// label follows the resource's own ecosystem instead of the old fixed
// "Visual Studio debugging" text -- a CocoaPods Pods/ directory always
// implies Xcode, never Visual Studio, so explaining it on macOS must not
// show a Windows-IDE-specific label.
func TestExplainCommandLabelsDebugByEcosystem(t *testing.T) {
	scanRoot = ""
	cfgPath = ""

	fixture, err := filepath.Abs("../testdata/xcode")
	if err != nil {
		t.Fatalf("resolve fixture path: %v", err)
	}
	t.Chdir(t.TempDir())

	run := func(args ...string) *bytes.Buffer {
		t.Helper()
		out := &bytes.Buffer{}
		rootCmd.SetOut(out)
		rootCmd.SetErr(out)
		rootCmd.SetArgs(args)
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("Execute(%v) error = %v; output=%s", args, err, out)
		}
		return out
	}

	run("init")
	run("scan", "--root", fixture)

	out := run("explain", filepath.Join(fixture, "basic", "Pods"))
	if !bytes.Contains(out.Bytes(), []byte("Xcode debugging: HIGH")) {
		t.Fatalf("explain output missing %q:\n%s", "Xcode debugging: HIGH", out)
	}
	if bytes.Contains(out.Bytes(), []byte("Visual Studio")) {
		t.Fatalf("explain output for a CocoaPods resource must not mention Visual Studio:\n%s", out)
	}
}

// TestExplainCommandJSONOmitsLastModifiedAtForResource locks down that
// explaining a resource never leaks last_modified_at into JSON. That field
// is project-only (see output.ExplainView's doc comment), but when it was a
// bare time.Time, JSON's "omitempty" silently had no effect on it -- a
// resource-kind view marshaled the Go zero value
// ("0001-01-01T00:00:00Z") unconditionally, even though the text renderer
// never prints "Last modified" for a resource at all.
func TestExplainCommandJSONOmitsLastModifiedAtForResource(t *testing.T) {
	scanRoot = ""
	cfgPath = ""
	jsonOutput = false
	t.Cleanup(func() { jsonOutput = false })

	fixture, err := filepath.Abs("../testdata/node")
	if err != nil {
		t.Fatalf("resolve fixture path: %v", err)
	}
	t.Chdir(t.TempDir())

	run := func(args ...string) *bytes.Buffer {
		t.Helper()
		out := &bytes.Buffer{}
		rootCmd.SetOut(out)
		rootCmd.SetErr(out)
		rootCmd.SetArgs(args)
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("Execute(%v) error = %v; output=%s", args, err, out)
		}
		return out
	}

	run("init")
	run("scan", "--root", fixture)

	out := run("explain", "--json", filepath.Join(fixture, "basic", "node_modules"))
	if bytes.Contains(out.Bytes(), []byte("last_modified_at")) {
		t.Fatalf("resource explain JSON must omit last_modified_at entirely, got:\n%s", out)
	}
}

// TestExplainCommandJSONKeepsLastModifiedAtForProject is the project-kind
// counterpart: last_modified_at must still be present and real once the
// field became a pointer.
func TestExplainCommandJSONKeepsLastModifiedAtForProject(t *testing.T) {
	scanRoot = ""
	cfgPath = ""
	jsonOutput = false
	t.Cleanup(func() { jsonOutput = false })

	fixture, err := filepath.Abs("../testdata/node")
	if err != nil {
		t.Fatalf("resolve fixture path: %v", err)
	}
	t.Chdir(t.TempDir())

	run := func(args ...string) *bytes.Buffer {
		t.Helper()
		out := &bytes.Buffer{}
		rootCmd.SetOut(out)
		rootCmd.SetErr(out)
		rootCmd.SetArgs(args)
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("Execute(%v) error = %v; output=%s", args, err, out)
		}
		return out
	}

	run("init")
	run("scan", "--root", fixture)

	out := run("explain", "--json", "project:"+filepath.Join(fixture, "basic"))
	if bytes.Contains(out.Bytes(), []byte(`"last_modified_at":"0001-01-01T00:00:00Z"`)) {
		t.Fatalf("project explain JSON must not report the zero-value timestamp:\n%s", out)
	}
	if !bytes.Contains(out.Bytes(), []byte("last_modified_at")) {
		t.Fatalf("project explain JSON must still report last_modified_at:\n%s", out)
	}
}

func TestExplainCommandUnknownTargetErrors(t *testing.T) {
	scanRoot = ""
	cfgPath = ""
	t.Chdir(t.TempDir())

	out := &bytes.Buffer{}
	rootCmd.SetOut(out)
	rootCmd.SetErr(out)
	rootCmd.SetArgs([]string{"init"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute(init) error = %v", err)
	}

	rootCmd.SetArgs([]string{"explain", "does-not-exist"})
	if err := rootCmd.Execute(); err == nil {
		t.Fatalf("Execute(explain does-not-exist) error = nil, want ErrTargetNotFound")
	}
}
