package cmd

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/app"
)

type scanRepositoryStub struct {
	record app.ScanRecord
	err    error
}

func (s scanRepositoryStub) Save(context.Context, app.ScanRecord) error { return nil }
func (s scanRepositoryStub) Find(context.Context, string) (app.ScanRecord, error) {
	return app.ScanRecord{}, errors.New("not implemented")
}
func (s scanRepositoryStub) FindLatest(context.Context) (app.ScanRecord, error) {
	return s.record, s.err
}

func TestPreviousScanFileCount(t *testing.T) {
	tests := []struct {
		name      string
		repo      scanRepositoryStub
		wantTotal int64
		wantOK    bool
	}{
		{
			name: "no prior scan",
			repo: scanRepositoryStub{err: app.ErrNoScans},
		},
		{
			name: "prior scan still running",
			repo: scanRepositoryStub{record: app.ScanRecord{Status: app.ScanStatusRunning, FileCount: 500}},
		},
		{
			name: "prior scan failed",
			repo: scanRepositoryStub{record: app.ScanRecord{Status: app.ScanStatusFailed, FileCount: 500}},
		},
		{
			name:      "prior scan completed",
			repo:      scanRepositoryStub{record: app.ScanRecord{Status: app.ScanStatusCompleted, FileCount: 13500}},
			wantTotal: 13500, wantOK: true,
		},
		{
			name:      "prior scan completed with errors still usable",
			repo:      scanRepositoryStub{record: app.ScanRecord{Status: app.ScanStatusCompletedWithErrors, FileCount: 42}},
			wantTotal: 42, wantOK: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			total, ok := previousScanFileCount(context.Background(), tt.repo)
			if total != tt.wantTotal || ok != tt.wantOK {
				t.Fatalf("previousScanFileCount() = %d, %v; want %d, %v", total, ok, tt.wantTotal, tt.wantOK)
			}
		})
	}
}

func TestDeterminateBarClampsAboveOneHundredPercent(t *testing.T) {
	bar := determinateBar(1.5)
	want := "[" + strings.Repeat("=", progressBarWidth) + "]"
	if bar != want {
		t.Fatalf("determinateBar(1.5) = %q, want fully filled %q", bar, want)
	}
}

// lastBlock returns the block of rows from the most recent repaint, found by
// taking whatever multiLineDisplay printed after its last erase-and-move-up
// escape sequence (or from the start, if it never erased anything yet).
func lastBlock(t *testing.T, out string) []string {
	t.Helper()
	parts := strings.Split(out, "\x1b[J")
	tail := parts[len(parts)-1]
	tail = strings.TrimRight(tail, "\n")
	if tail == "" {
		return nil
	}
	return strings.Split(tail, "\n")
}

func TestMultiLineDisplayRepaintErasesPreviousBlock(t *testing.T) {
	out := &bytes.Buffer{}
	d := newMultiLineDisplay(out)

	d.repaint([]string{"a", "b"})
	if got := out.String(); strings.Contains(got, "\x1b[") {
		t.Fatalf("first repaint should not erase anything (nothing drawn yet), got %q", got)
	}
	if got := lastBlock(t, out.String()); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("first repaint rows = %v, want [a b]", got)
	}

	d.repaint([]string{"c", "d", "e"})
	got := out.String()
	if !strings.Contains(got, "\x1b[2A\r\x1b[J") {
		t.Fatalf("second repaint should move up by the previous row count (2), got %q", got)
	}
	if rows := lastBlock(t, got); len(rows) != 3 || rows[2] != "e" {
		t.Fatalf("second repaint rows = %v, want [c d e]", rows)
	}
}

func TestMultiLineDisplayClearErasesAndIsIdempotent(t *testing.T) {
	out := &bytes.Buffer{}
	d := newMultiLineDisplay(out)
	d.repaint([]string{"a", "b", "c"})
	d.clear()

	got := out.String()
	if !strings.Contains(got, "\x1b[3A\r\x1b[J") {
		t.Fatalf("clear() should move up by the printed row count (3), got %q", got)
	}
	if d.printedRows != 0 {
		t.Fatalf("printedRows after clear() = %d, want 0", d.printedRows)
	}

	afterFirstClear := out.String()
	d.clear() // nothing printed since the last clear -- must be a no-op
	if out.String() != afterFirstClear {
		t.Fatalf("clear() wrote again with nothing to clear: %q -> %q", afterFirstClear, out.String())
	}
}

func TestScanProgressDisplayDuringDiscoveryShowsBarThenFilesAndDirectoriesOnOneLine(t *testing.T) {
	out := &bytes.Buffer{}
	d := newScanProgressDisplay(out, 0) // indeterminate: no prior scan to estimate from
	d.Update(app.ScanProgress{FilesInspected: 8432, Directories: 1021})

	rows := lastBlock(t, out.String())
	if len(rows) != 2 {
		t.Fatalf("rows = %v, want 2 (bar, files+directories)", rows)
	}
	if strings.Contains(rows[0], "files") || strings.Contains(rows[0], "directories") {
		t.Fatalf("bar row = %q, files/directories belong on the line below it", rows[0])
	}
	if rows[1] != "files: 8,432, directories: 1,021" {
		t.Fatalf("rows[1] = %q, want %q", rows[1], "files: 8,432, directories: 1,021")
	}
}

func TestScanProgressDisplayDeterminateBarShowsPercentage(t *testing.T) {
	out := &bytes.Buffer{}
	d := newScanProgressDisplay(out, 13500) // prior scan estimate available
	d.Update(app.ScanProgress{FilesInspected: 8370})

	rows := lastBlock(t, out.String())
	if !strings.Contains(rows[0], "62%") {
		t.Fatalf("bar row = %q, want ~62%%", rows[0])
	}
}

// TestScanProgressDisplayFinishesDiscoveryAtOneHundredPercent covers the
// freeze reported after file discovery: once the first post-discovery phase
// starts, the bar must collapse to "Scan 100%" (no more animated graphic)
// while the files/directories row stays visible with its final counts.
func TestScanProgressDisplayFinishesDiscoveryAtOneHundredPercent(t *testing.T) {
	out := &bytes.Buffer{}
	d := newScanProgressDisplay(out, 0)
	d.Update(app.ScanProgress{FilesInspected: 13502, Directories: 1842})
	d.Phase(app.PhaseDiscoverProjects, app.ScanProgress{FilesInspected: 13502, Directories: 1842})

	rows := lastBlock(t, out.String())
	if rows[0] != green("Scan 100%") {
		t.Fatalf("rows[0] = %q, want the bar replaced by a green \"Scan 100%%\"", rows[0])
	}
	if rows[1] != "files: 13,502, directories: 1,842" {
		t.Fatalf("rows[1] = %q, want the final file/directory counts to remain", rows[1])
	}
	// The first post-discovery phase (DiscoverProjects) must already be
	// showing as the current (not-yet-done) phase.
	if rows[len(rows)-1] != "Analyzing projects..." {
		t.Fatalf("last row = %q, want the DiscoverProjects label", rows[len(rows)-1])
	}
}

// TestScanProgressDisplayNeverShowsProjectsOrResourcesRows: an earlier
// iteration added "projects: N" / "resources: M" rows as those phases
// finished; that was explicitly dropped, so this guards against it coming
// back even though the orchestrator still reports Projects/Resources on
// ScanProgress (other callers may still want them).
func TestScanProgressDisplayNeverShowsProjectsOrResourcesRows(t *testing.T) {
	out := &bytes.Buffer{}
	d := newScanProgressDisplay(out, 0)
	d.Phase(app.PhaseDiscoverProjects, app.ScanProgress{})
	d.Phase(app.PhaseDiscoverSystemResources, app.ScanProgress{Projects: 7})
	d.Phase(app.PhaseAnalyzeProjectSettings, app.ScanProgress{Projects: 7, Resources: 11})

	rows := lastBlock(t, out.String())
	if containsPrefix(rows, "projects:") || containsPrefix(rows, "resources:") {
		t.Fatalf("rows = %v, projects/resources rows should not appear", rows)
	}
}

// TestScanProgressDisplayCompletedCommitsLastPhaseWithoutAddingARow ensures
// PhaseCompleted (which has no label of its own) still finalizes whatever
// phase was last active, instead of leaving it stuck mid-progress forever.
func TestScanProgressDisplayCompletedCommitsLastPhaseWithoutAddingARow(t *testing.T) {
	out := &bytes.Buffer{}
	d := newScanProgressDisplay(out, 0)
	d.Phase(app.PhaseDiscoverProjects, app.ScanProgress{})
	d.Phase(app.PhaseDiscoverSystemResources, app.ScanProgress{})
	d.Phase(app.PhaseAnalyzeProjectSettings, app.ScanProgress{})
	d.Phase(app.PhaseResolveDependencies, app.ScanProgress{})
	d.Phase(app.PhaseClassifyArtifacts, app.ScanProgress{})
	d.Phase(app.PhaseCalculateRisk, app.ScanProgress{})
	d.Phase(app.PhasePersistResults, app.ScanProgress{})
	d.Phase(app.PhaseCompleted, app.ScanProgress{})

	rows := lastBlock(t, out.String())
	if !containsExact(rows, green("Persisting results... 100%")) {
		t.Fatalf("rows = %v, want the last phase committed at a green 100%%", rows)
	}
	if d.currentPhase != "" {
		t.Fatalf("currentPhase = %q after PhaseCompleted, want empty", d.currentPhase)
	}
}

// TestScanProgressDisplayOnlyColorsTheLineThatJustFinished checks the
// request precisely: green applies to the "Scan 100%" row and each
// "<phase> 100%" row individually, and only those -- not the files/
// directories row, and not whichever phase is still in progress.
func TestScanProgressDisplayOnlyColorsTheLineThatJustFinished(t *testing.T) {
	out := &bytes.Buffer{}
	d := newScanProgressDisplay(out, 0)
	d.Update(app.ScanProgress{FilesInspected: 5, Directories: 1})
	d.Phase(app.PhaseDiscoverProjects, app.ScanProgress{FilesInspected: 5, Directories: 1})
	d.Phase(app.PhaseDiscoverSystemResources, app.ScanProgress{FilesInspected: 5, Directories: 1})

	rows := lastBlock(t, out.String())
	if rows[0] != green("Scan 100%") {
		t.Fatalf("rows[0] = %q, want it green", rows[0])
	}
	if strings.Contains(rows[1], ansiGreen) {
		t.Fatalf("files/directories row = %q, should not be colored", rows[1])
	}
	if !containsExact(rows, green("Analyzing projects... 100%")) {
		t.Fatalf("rows = %v, want the finished phase green", rows)
	}
	last := rows[len(rows)-1]
	if last != "Detecting SDKs and resources..." || strings.Contains(last, ansiGreen) {
		t.Fatalf("last row = %q, the in-progress phase should not be colored yet", last)
	}
}

func TestScanProgressDisplayDoneClearsEverything(t *testing.T) {
	out := &bytes.Buffer{}
	d := newScanProgressDisplay(out, 0)
	d.Update(app.ScanProgress{FilesInspected: 5, Directories: 1})
	d.Done()

	if got := lastBlock(t, out.String()); len(got) != 0 {
		t.Fatalf("after Done(), lastBlock = %v, want nothing left on screen", got)
	}
}

func containsExact(rows []string, want string) bool {
	for _, row := range rows {
		if row == want {
			return true
		}
	}
	return false
}

func containsPrefix(rows []string, prefix string) bool {
	for _, row := range rows {
		if strings.HasPrefix(row, prefix) {
			return true
		}
	}
	return false
}
