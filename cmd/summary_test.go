package cmd

import (
	"bytes"
	"path/filepath"
	"regexp"
	"testing"
)

func TestSummaryCommandReflectsScannedProjects(t *testing.T) {
	scanRoot = ""
	cfgPath = ""
	summaryType = ""
	summaryDrive = ""

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

	run("scan", "--root", fixture)
	out := run("summary")
	// Match tolerant of tabwriter column width, which shifts as resource
	// rows widen the label column.
	if !regexp.MustCompile(`Projects\s+7\b`).Match(out.Bytes()) {
		t.Fatalf("summary output missing project count:\n%s", out)
	}
}

// TestSummaryCommandShowsLastScanFreshness covers issue #41: summary had no
// indication of when it last scanned, so a stale result looked identical to
// a fresh one. ScanRecord (Roots/FileCount/ErrorCount) is already persisted
// by `scan`; this only exercises rendering it, not measuring anything new.
func TestSummaryCommandShowsLastScanFreshness(t *testing.T) {
	scanRoot = ""
	cfgPath = ""
	summaryType = ""
	summaryDrive = ""

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

	run("scan", "--root", fixture)
	out := run("summary")
	for _, want := range []string{"Last scan", "Coverage", "Files inspected"} {
		if !bytes.Contains(out.Bytes(), []byte(want)) {
			t.Fatalf("summary output missing %q:\n%s", want, out)
		}
	}
}

// TestSummaryCommandOmitsFreshnessBeforeFirstScan ensures the freshness
// section is absent (not an error, not zero-valued placeholders) when no
// scan has ever run -- ScanRepository.FindLatest's ErrNoScans is expected,
// ordinary state here, same as summary's other all-zero counts before a
// first scan.
func TestSummaryCommandOmitsFreshnessBeforeFirstScan(t *testing.T) {
	scanRoot = ""
	cfgPath = ""
	summaryType = ""
	summaryDrive = ""
	t.Chdir(t.TempDir())

	out := &bytes.Buffer{}
	rootCmd.SetOut(out)
	rootCmd.SetErr(out)
	rootCmd.SetArgs([]string{"init"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute(init) error = %v", err)
	}

	out.Reset()
	rootCmd.SetArgs([]string{"summary"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute(summary) error = %v", err)
	}
	if bytes.Contains(out.Bytes(), []byte("Last scan")) {
		t.Fatalf("summary output should omit freshness section before any scan:\n%s", out)
	}
}

func TestSummaryCommandTypeFilterIsCaseInsensitive(t *testing.T) {
	scanRoot = ""
	cfgPath = ""
	summaryType = ""
	summaryDrive = ""

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

	run("scan", "--root", fixture)
	lowerCase := run("summary", "--type", "node-modules")
	summaryType = ""

	// --type must match case-insensitively like --drive already does (same
	// bug class as finding #8 in docs/libra_review_findings_day4.md, found
	// here too): stored resource types are lowercase ("node-modules"), so a
	// differently-cased query is the realistic way a user would type it. A
	// correctly-cased and a differently-cased query for the same type must
	// report the same nonzero resource count.
	mixedCase := run("summary", "--type", "Node-Modules")

	lowerCaseCount := regexp.MustCompile(`Resources\s+(\d+)`).FindSubmatch(lowerCase.Bytes())
	mixedCaseCount := regexp.MustCompile(`Resources\s+(\d+)`).FindSubmatch(mixedCase.Bytes())
	if lowerCaseCount == nil || mixedCaseCount == nil {
		t.Fatalf("could not parse resource counts:\nlowerCase=%s\nmixedCase=%s", lowerCase, mixedCase)
	}
	if string(mixedCaseCount[1]) != string(lowerCaseCount[1]) || string(lowerCaseCount[1]) == "0" {
		t.Fatalf("summary --type Node-Modules resource count = %s, want same as --type node-modules (%s) and nonzero", mixedCaseCount[1], lowerCaseCount[1])
	}
}

func TestSummaryCommandBeforeScanIsAllZero(t *testing.T) {
	scanRoot = ""
	cfgPath = ""
	summaryType = ""
	summaryDrive = ""
	t.Chdir(t.TempDir())

	out := &bytes.Buffer{}
	rootCmd.SetOut(out)
	rootCmd.SetErr(out)
	rootCmd.SetArgs([]string{"init"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute(init) error = %v", err)
	}

	out.Reset()
	rootCmd.SetArgs([]string{"summary"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute(summary) error = %v", err)
	}
	if !regexp.MustCompile(`Projects\s+0\b`).Match(out.Bytes()) {
		t.Fatalf("summary output = %s, want zero projects", out)
	}
}
