package cmd

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/madcamp-official/26s-w3-c2-01/internal/app"
)

// previousScanFileCount looks up the file count of the most recent scan that
// actually finished, for use as an estimated total on a new scan's progress
// bar. ok is false when there is nothing usable to estimate from -- either
// this is the first-ever scan, or the prior one never completed (its
// FileCount would be zero or mid-scan, not a real total).
func previousScanFileCount(ctx context.Context, scans app.ScanRepository) (total int64, ok bool) {
	record, err := scans.FindLatest(ctx)
	if err != nil {
		return 0, false
	}
	if record.Status != app.ScanStatusCompleted && record.Status != app.ScanStatusCompletedWithErrors {
		return 0, false
	}
	if record.FileCount <= 0 {
		return 0, false
	}
	return record.FileCount, true
}

const (
	progressBarWidth       = 20
	progressRedrawInterval = 100 * time.Millisecond
)

// multiLineDisplay renders a growing block of status lines that redraws
// itself in place: each repaint erases whatever it printed last time and
// reprints the current rows, so a caller can freely change row count and
// row content between calls without tracking per-line diffs itself. It
// depends on the terminal understanding ANSI cursor movement (standard on
// Windows 10 1511+/Windows 11 and every other modern terminal).
type multiLineDisplay struct {
	w           io.Writer
	printedRows int
}

func newMultiLineDisplay(w io.Writer) *multiLineDisplay {
	return &multiLineDisplay{w: w}
}

// repaint erases the previously printed block (if any) and prints rows in
// its place, in a single Write call. Writing the erase sequence and every
// row separately (one Write per line) let the terminal paint each of those
// writes as its own frame -- clear, then blank, then rows filling back in
// one at a time -- which is what caused the visible flicker; one Write
// makes it a single atomic frame.
func (d *multiLineDisplay) repaint(rows []string) {
	var b strings.Builder
	if d.printedRows > 0 {
		fmt.Fprintf(&b, "\x1b[%dA\r\x1b[J", d.printedRows)
	}
	for _, row := range rows {
		b.WriteString(row)
		b.WriteByte('\n')
	}
	io.WriteString(d.w, b.String())
	d.printedRows = len(rows)
}

// clear erases the block entirely, leaving the cursor where the block used
// to start so whatever prints next (the final scan summary, or an error)
// starts clean. A no-op if nothing is currently drawn.
func (d *multiLineDisplay) clear() {
	if d.printedRows == 0 {
		return
	}
	fmt.Fprintf(d.w, "\x1b[%dA\r\x1b[J", d.printedRows)
	d.printedRows = 0
}

// scanPhaseLabels names the phases that run after PhaseDiscoverFiles and
// before PhaseCompleted -- PhaseDiscoverFiles has no label because
// scanProgressDisplay's bar/counts rows already cover it, and PhaseCompleted
// has no work of its own (Run returns right after reaching it).
var scanPhaseLabels = map[app.AnalysisPhase]string{
	app.PhaseDiscoverProjects:        "Analyzing projects...",
	app.PhaseDiscoverSystemResources: "Detecting SDKs and resources...",
	app.PhaseAnalyzeProjectSettings:  "Analyzing project settings...",
	app.PhaseResolveDependencies:     "Resolving dependencies...",
	app.PhaseClassifyArtifacts:       "Classifying artifacts...",
	app.PhaseCalculateRisk:           "Calculating risk...",
	app.PhasePersistResults:          "Persisting results...",
}

// scanProgressDisplay is `libra scan`'s whole live status block:
//
//	Scanning... [============--------] 62%
//	files: 8,432, directories: 1,021
//
// Once file discovery finishes, the bar line collapses to "Scan 100%" and
// stays (along with the final files/directories line), followed by one row
// per remaining phase -- each committed as "<label> 100%" once the next
// phase starts, so a slow phase (e.g. spawning vswhere.exe, or re-walking
// every detected project to size it) is visible instead of the whole
// display just sitting frozen. Done() clears the entire block once the scan
// (or an error) is ready to print.
type scanProgressDisplay struct {
	display *multiLineDisplay

	estimatedTotal int64 // 0 means indeterminate
	frame          int
	lastDraw       time.Time

	discovery        app.ScanProgress
	discoveryStarted bool
	discoveryDone    bool

	donePhases   []string
	currentPhase string
}

func newScanProgressDisplay(w io.Writer, estimatedTotal int64) *scanProgressDisplay {
	return &scanProgressDisplay{display: newMultiLineDisplay(w), estimatedTotal: estimatedTotal}
}

// Update is the callback passed to AnalysisOrchestrator.WithProgress. It
// throttles actual redraws to progressRedrawInterval so a scan visiting
// tens of thousands of entries doesn't spend its time repainting the
// terminal.
func (d *scanProgressDisplay) Update(progress app.ScanProgress) {
	d.discovery = progress
	d.discoveryStarted = true
	now := time.Now()
	if !d.lastDraw.IsZero() && now.Sub(d.lastDraw) < progressRedrawInterval {
		return
	}
	d.lastDraw = now
	d.frame++
	d.display.repaint(d.rows())
}

// Phase is the callback passed to AnalysisOrchestrator.WithPhaseHook. Phase
// transitions are rare (a handful per scan), so every one redraws
// immediately -- no throttling.
func (d *scanProgressDisplay) Phase(phase app.AnalysisPhase, progress app.ScanProgress) {
	d.discovery = progress
	if phase == app.PhaseDiscoverFiles {
		return
	}
	d.discoveryStarted = true
	d.discoveryDone = true
	if d.currentPhase != "" {
		d.donePhases = append(d.donePhases, d.currentPhase)
	}
	d.currentPhase = scanPhaseLabels[phase] // "" for PhaseCompleted: nothing follows it
	d.display.repaint(d.rows())
}

// Done clears the whole display so the final scan summary (or an error)
// prints on a clean screen.
func (d *scanProgressDisplay) Done() {
	d.display.clear()
}

func (d *scanProgressDisplay) rows() []string {
	var rows []string
	if d.discoveryDone {
		rows = append(rows, green("Scan 100%"))
	} else {
		rows = append(rows, d.barRow())
	}
	if d.discoveryStarted {
		rows = append(rows, fmt.Sprintf("files: %s, directories: %s",
			humanize.Comma(d.discovery.FilesInspected), humanize.Comma(d.discovery.Directories)))
	}
	for _, done := range d.donePhases {
		rows = append(rows, green(done+" 100%"))
	}
	if d.currentPhase != "" {
		rows = append(rows, d.currentPhase)
	}
	return rows
}

// ansiGreen/ansiReset color a line that just finished (the "Scan 100%" row,
// and each phase row once committed at "<label> 100%") without affecting
// anything else on the same or adjacent rows.
const (
	ansiGreen = "\x1b[32m"
	ansiReset = "\x1b[0m"
)

func green(s string) string {
	return ansiGreen + s + ansiReset
}

func (d *scanProgressDisplay) barRow() string {
	if d.estimatedTotal > 0 {
		fraction := float64(d.discovery.FilesInspected) / float64(d.estimatedTotal)
		if fraction > 1 {
			fraction = 1
		}
		return fmt.Sprintf("Scanning... %s %3.0f%%", determinateBar(fraction), fraction*100)
	}
	return fmt.Sprintf("Scanning... %s", indeterminateBar(d.frame))
}

func determinateBar(fraction float64) string {
	if fraction > 1 {
		fraction = 1
	}
	if fraction < 0 {
		fraction = 0
	}
	filled := int(fraction * progressBarWidth)
	return "[" + strings.Repeat("=", filled) + strings.Repeat("-", progressBarWidth-filled) + "]"
}

// indeterminateBar draws a fixed-length segment bouncing back and forth
// across the bar, advancing one step per call -- there is no known total to
// measure real progress against, so this only signals "still scanning".
func indeterminateBar(frame int) string {
	const segment = 6
	span := progressBarWidth - segment
	period := 2 * span
	if period <= 0 {
		return "[" + strings.Repeat("=", progressBarWidth) + "]"
	}
	pos := frame % period
	if pos > span {
		pos = period - pos
	}
	return "[" + strings.Repeat("-", pos) + strings.Repeat("=", segment) + strings.Repeat("-", span-pos) + "]"
}
