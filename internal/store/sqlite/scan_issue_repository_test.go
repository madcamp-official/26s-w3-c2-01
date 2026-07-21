package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/app"
)

func TestScanIssueRepositoryReplacesListsFiltersAndCascades(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := NewScanRepository(db).Save(ctx, app.ScanRecord{
		ID: "scan-issues", StartedAt: time.Now(), Roots: []string{"/projects"}, Status: app.ScanStatusCompletedWithErrors,
	}); err != nil {
		t.Fatal(err)
	}

	repository := NewScanIssueRepository(db)
	issues := []app.Issue{
		{Code: app.IssueAccessDenied, Phase: app.PhaseDiscoverFiles, Path: "/denied", Operation: "read", Severity: app.IssueWarning, Message: "permission denied"},
		{Code: app.IssueAdapterFailed, Phase: app.PhaseResolveDependencies, Adapter: "msbuild", Severity: app.IssueError, Message: "tool failed"},
	}
	if err := repository.Replace(ctx, "scan-issues", issues); err != nil {
		t.Fatalf("Replace() error = %v", err)
	}

	got, err := repository.List(ctx, app.IssueFilter{ScanID: "scan-issues", Severity: app.IssueWarning})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Code != app.IssueAccessDenied || got[0].Cause != nil {
		t.Fatalf("List() = %#v", got)
	}

	if err := repository.Replace(ctx, "scan-issues", issues[1:]); err != nil {
		t.Fatal(err)
	}
	got, err = repository.List(ctx, app.IssueFilter{ScanID: "scan-issues"})
	if err != nil || len(got) != 1 || got[0].Adapter != "msbuild" {
		t.Fatalf("List() after replace = %#v, %v", got, err)
	}

	if _, err := db.Exec("DELETE FROM scans WHERE id = 'scan-issues'"); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM scan_issues").Scan(&count); err != nil || count != 0 {
		t.Fatalf("issues after scan delete = %d, %v", count, err)
	}
}
