package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/madcamp-official/26s-w3-c2-01/internal/app"
	"github.com/madcamp-official/26s-w3-c2-01/internal/output"
	"github.com/madcamp-official/26s-w3-c2-01/internal/store/sqlite"
	"github.com/spf13/cobra"
)

var exportFormat, exportOutputPath string

var exportCmd = &cobra.Command{
	Use: "export", Short: "Export the latest analysis report", Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		format := strings.ToLower(strings.TrimSpace(exportFormat))
		if format != "json" && format != "markdown" {
			return errors.New("--format must be json or markdown")
		}
		db, err := openDatabase()
		if err != nil {
			return err
		}
		defer db.Close()
		report, err := app.NewExportService(sqlite.NewScanRepository(db), sqlite.NewProjectRepository(db), sqlite.NewResourceRepository(db), sqlite.NewScanIssueRepository(db), sqlite.NewCleanupTransactionRepository(db)).Build(cmd.Context())
		if err != nil {
			return err
		}

		var writer io.Writer = cmd.OutOrStdout()
		var file *os.File
		if exportOutputPath != "" {
			file, err = os.OpenFile(exportOutputPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
			if err != nil {
				return fmt.Errorf("open export output: %w", err)
			}
			defer file.Close()
			writer = file
		}
		if jsonOutput {
			if format != "json" {
				return errors.New("--json cannot be combined with --format markdown")
			}
			err = output.New(writer, true, "export").Print(output.ExportView(report))
		} else if format == "json" {
			err = output.WriteExportJSON(writer, report)
		} else {
			err = output.WriteExportMarkdown(writer, report)
		}
		if err != nil {
			return fmt.Errorf("write %s export: %w", format, err)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(exportCmd)
	exportCmd.Flags().StringVar(&exportFormat, "format", "json", "export format: json or markdown")
	exportCmd.Flags().StringVar(&exportOutputPath, "output", "", "write report to a file instead of stdout")
}
