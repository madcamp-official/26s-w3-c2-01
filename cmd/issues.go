package cmd

import (
	"fmt"
	"strings"

	"github.com/madcamp-official/26s-w3-c2-01/internal/app"
	"github.com/madcamp-official/26s-w3-c2-01/internal/output"
	"github.com/madcamp-official/26s-w3-c2-01/internal/store/sqlite"
	"github.com/spf13/cobra"
)

var (
	issuesScanID   string
	issuesCode     string
	issuesSeverity string
)

var issuesCmd = &cobra.Command{
	Use:   "issues",
	Short: "List warnings and errors recorded by a scan",
	Long: `issues lists structured warnings and errors persisted by libra scan.
By default it reads the latest scan; use --scan to inspect an earlier scan.`,
	Example: `  libra issues
  libra issues --scan scan-20260721-120000
  libra issues --code ACCESS_DENIED --severity warning`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		db, err := openDatabase()
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer db.Close()

		service := app.NewIssueService(sqlite.NewScanIssueRepository(db), sqlite.NewScanRepository(db))
		scanID, issues, err := service.List(cmd.Context(), app.IssueFilter{
			ScanID: issuesScanID, Code: app.IssueCode(strings.ToUpper(strings.TrimSpace(issuesCode))),
			Severity: app.IssueSeverity(strings.ToUpper(strings.TrimSpace(issuesSeverity))),
		})
		if err != nil {
			return err
		}

		view := output.IssuesView{ScanID: scanID}
		for _, issue := range issues {
			view.Issues = append(view.Issues, output.IssueLine{
				Code: issue.Code, Phase: issue.Phase, Adapter: issue.Adapter, Path: issue.Path,
				Operation: issue.Operation, Severity: issue.Severity, Message: issue.Message,
			})
		}
		return output.New(cmd.OutOrStdout(), jsonOutput, "issues").PrintEnvelope(view, view.Envelope())
	},
}

func init() {
	rootCmd.AddCommand(issuesCmd)
	issuesCmd.Flags().StringVar(&issuesScanID, "scan", "", "scan ID to inspect (default: latest scan)")
	issuesCmd.Flags().StringVar(&issuesCode, "code", "", "filter by issue code (e.g. ACCESS_DENIED)")
	issuesCmd.Flags().StringVar(&issuesSeverity, "severity", "", "filter by severity (warning or error)")
}
