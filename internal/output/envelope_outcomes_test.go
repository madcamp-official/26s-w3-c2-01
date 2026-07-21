package output

import (
	"testing"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

// TestCleanViewEnvelopeReflectsDriftedItems is a regression test for issue
// #59: a dry-run where every item still matches the plan snapshot is
// SUCCESS; a CHANGED/MISSING item (the plan is stale) must flip Outcome to
// PARTIAL and surface as an Issue, since that's the whole reason clean
// distinguishes those statuses in the first place.
func TestCleanViewEnvelopeReflectsDriftedItems(t *testing.T) {
	allGood := CleanView{Items: []CleanItemLine{{Path: "/a", Status: CleanItemWouldMove}}}
	if got := allGood.Envelope(); got.Outcome != OutcomeSuccess || len(got.Issues) != 0 {
		t.Errorf("Envelope() for an all-WOULD_MOVE plan = %+v, want OutcomeSuccess and no issues", got)
	}

	drifted := CleanView{Items: []CleanItemLine{
		{Path: "/a", Status: CleanItemWouldMove},
		{Path: "/b", Status: CleanItemMissing, Detail: "no longer known"},
	}}
	got := drifted.Envelope()
	if got.Outcome != OutcomePartial {
		t.Errorf("Outcome = %q, want %q when a plan item drifted", got.Outcome, OutcomePartial)
	}
	if len(got.Issues) != 1 || got.Issues[0].Code != "MISSING" || got.Issues[0].Path != "/b" {
		t.Errorf("Issues = %+v, want exactly one MISSING issue for /b", got.Issues)
	}
}

// TestCleanupTransactionViewEnvelopeMapsStatusAndFailedItems is a
// regression test for issue #59: transactionOutcome must distinguish full
// success, partial (some items failed/skipped but not all), and full
// failure, and only FAILED/SKIPPED items should surface as Issues --
// MOVED/RESTORED/PURGED items succeeded, not problems to report.
func TestCleanupTransactionViewEnvelopeMapsStatusAndFailedItems(t *testing.T) {
	tests := []struct {
		name        string
		status      domain.CleanupTransactionStatus
		wantOutcome Outcome
	}{
		{"quarantined", domain.TransactionQuarantined, OutcomeSuccess},
		{"restored", domain.TransactionRestored, OutcomeSuccess},
		{"purged", domain.TransactionPurged, OutcomeSuccess},
		{"partially quarantined", domain.TransactionPartiallyQuarantined, OutcomePartial},
		{"failed", domain.TransactionFailed, OutcomeFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			view := CleanupTransactionView{Status: tt.status}
			if got := view.Envelope().Outcome; got != tt.wantOutcome {
				t.Errorf("Envelope().Outcome for status %q = %q, want %q", tt.status, got, tt.wantOutcome)
			}
		})
	}

	view := CleanupTransactionView{
		Status: domain.TransactionPartiallyQuarantined,
		Items: []CleanupTransactionItemView{
			{OriginalPath: "/moved", Status: domain.TransactionItemMoved},
			{OriginalPath: "/failed", Status: domain.TransactionItemFailed, Reason: "locked"},
			{OriginalPath: "/skipped", Status: domain.TransactionItemSkipped, Reason: "already gone"},
		},
	}
	got := view.Envelope()
	if len(got.Issues) != 2 {
		t.Fatalf("Issues = %+v, want exactly 2 (FAILED and SKIPPED, not MOVED)", got.Issues)
	}
	if got.Issues[0].Path != "/failed" || got.Issues[0].Severity != "ERROR" {
		t.Errorf("Issues[0] = %+v, want /failed with ERROR severity", got.Issues[0])
	}
	if got.Issues[1].Path != "/skipped" || got.Issues[1].Severity != "WARNING" {
		t.Errorf("Issues[1] = %+v, want /skipped with WARNING severity", got.Issues[1])
	}
}

// TestPurgeViewEnvelopeReusesTransactionOutcome confirms PurgeView shares
// the same status mapping as CleanupTransactionView (issue #59), since both
// carry domain.CleanupTransactionStatus.
func TestPurgeViewEnvelopeReusesTransactionOutcome(t *testing.T) {
	if got := (PurgeView{Status: domain.TransactionPurged}).Envelope().Outcome; got != OutcomeSuccess {
		t.Errorf("Outcome for TransactionPurged = %q, want %q", got, OutcomeSuccess)
	}
	if got := (PurgeView{Status: domain.TransactionFailed}).Envelope().Outcome; got != OutcomeFailed {
		t.Errorf("Outcome for TransactionFailed = %q, want %q", got, OutcomeFailed)
	}
}

// TestPlanViewEnvelopeReflectsInsufficientCandidates is a regression test
// for issue #59: a plan that couldn't reach --target completed, but fell
// short of what was asked -- exactly the case Outcome exists to flag.
func TestPlanViewEnvelopeReflectsInsufficientCandidates(t *testing.T) {
	if got := (PlanView{Status: domain.CleanupPlanReady}).Envelope().Outcome; got != OutcomeSuccess {
		t.Errorf("Outcome for READY = %q, want %q", got, OutcomeSuccess)
	}
	if got := (PlanView{Status: domain.CleanupPlanInsufficientCandidates}).Envelope().Outcome; got != OutcomePartial {
		t.Errorf("Outcome for INSUFFICIENT_CANDIDATES = %q, want %q", got, OutcomePartial)
	}
}

// TestIssuesViewEnvelopeAlwaysSucceeds confirms listing a past scan's
// issues never itself degrades `issues`' own Outcome (issue #59) -- having
// issues to show is not the same as this command's own operation being
// degraded, unlike ScanView's Envelope.
func TestIssuesViewEnvelopeAlwaysSucceeds(t *testing.T) {
	view := IssuesView{ScanID: "scan-1", Issues: []IssueLine{{Code: "ACCESS_DENIED", Message: "denied"}}}
	got := view.Envelope()
	if got.Outcome != OutcomeSuccess {
		t.Errorf("Outcome = %q, want %q (issues listing itself always succeeds)", got.Outcome, OutcomeSuccess)
	}
	if len(got.Issues) != 1 || got.Issues[0].Code != "ACCESS_DENIED" {
		t.Errorf("Issues = %+v, want the one ACCESS_DENIED issue carried through", got.Issues)
	}
}
