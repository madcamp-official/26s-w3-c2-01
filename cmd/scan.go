package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/madcamp-official/26s-w3-c2-01/internal/adapter/dotnet"
	gitadapter "github.com/madcamp-official/26s-w3-c2-01/internal/adapter/git"
	"github.com/madcamp-official/26s-w3-c2-01/internal/adapter/msbuild"
	nodeadapter "github.com/madcamp-official/26s-w3-c2-01/internal/adapter/node"
	"github.com/madcamp-official/26s-w3-c2-01/internal/adapter/windowsdk"
	"github.com/madcamp-official/26s-w3-c2-01/internal/app"
	"github.com/madcamp-official/26s-w3-c2-01/internal/config"
	"github.com/madcamp-official/26s-w3-c2-01/internal/safety"
	"github.com/madcamp-official/26s-w3-c2-01/internal/scanner"
	"github.com/madcamp-official/26s-w3-c2-01/internal/store/sqlite"
	"github.com/spf13/cobra"
)

// scan.go is the only command that writes to the database -- every other
// command (projects/resources/summary/explain/impact) only reads what a
// prior `libra scan` already persisted. resourceDetectors is a var (not a
// literal call) specifically so tests can stub it out; see
// cmd/summary_golden_test.go and cmd/resources_test.go, which both replace
// it to avoid depending on whatever SDKs happen to be installed on the
// machine running the test.
var (
	scanRoot          string
	scanFull          bool
	resourceDetectors = defaultResourceDetectors
)

func defaultResourceDetectors() []app.ResourceDetector {
	return []app.ResourceDetector{
		app.WindowsSDKResourceDetector{Detector: windowsdk.FilesystemDetector{}},
		app.DotNetSDKResourceDetector{Lister: dotnet.CLISDKLister{}},
		app.VisualStudioResourceDetector{Locator: msbuild.VSWhereToolLocator{}},
	}
}

// scanCmd represents the scan command.
var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Discover projects, resources, and build artifacts",
	Long: `scan walks the configured project roots, detects projects
(.sln, .vcxproj, .csproj, package.json, .git), detects known development
resources and build artifacts, computes their logical size, runs dependency
analysis, and stores the results in the local SQLite database.

Permission errors on individual paths are recorded but do not abort the
scan. Every scan is currently a full scan -- incremental scanning does
not exist yet, so --full has no effect (see --help).`,
	Example: `  libra scan
  libra scan --root D:\Projects`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		scanOpts, err := resolveScanOptions()
		if err != nil {
			return err
		}

		db, err := openDatabase()
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer db.Close()

		classifier, err := safety.NewSystemPathClassifier()
		if err != nil {
			return fmt.Errorf("build path classifier: %w", err)
		}

		filesystem := scanner.New(4)
		resources := app.NewResourceService(filesystem, sqlite.NewResourceRepository(db), classifier, app.DefaultRiskPolicy{})
		orchestrator := app.NewAnalysisOrchestrator(
			filesystem,
			sqlite.NewScanRepository(db),
			sqlite.NewProjectRepository(db),
			sqlite.NewWorkspaceRepository(db),
			resources,
			sqlite.NewDependencyRepository(db),
		).WithDetectors([]app.ProjectDetector{
			app.GitProjectDetector{Detector: gitadapter.FilesystemDetector{}},
			app.NodeProjectDetector{Detector: nodeadapter.FilesystemDetector{}},
			app.MSBuildProjectDetector{Parser: msbuild.XMLBuildProjectParser{}},
		}, resourceDetectors(), nil)

		result, err := orchestrator.Run(cmd.Context(), app.AnalysisOptions{
			ScanID: fmt.Sprintf("scan-%s", time.Now().UTC().Format("20060102-150405")),
			Scan:   scanOpts,
		})
		if err != nil {
			return fmt.Errorf("run scan: %w", err)
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Scan completed")
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintf(cmd.OutOrStdout(), "Roots scanned:   %d\n", result.Filesystem.RootsScanned)
		fmt.Fprintf(cmd.OutOrStdout(), "Projects found:  %d\n", len(result.Projects))
		fmt.Fprintf(cmd.OutOrStdout(), "Resources found: %d\n", len(result.Resources))
		fmt.Fprintf(cmd.OutOrStdout(), "Files inspected: %d\n", result.Filesystem.FilesInspected)
		fmt.Fprintf(cmd.OutOrStdout(), "Warnings:        %d\n", len(result.Issues))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(scanCmd)

	scanCmd.Flags().StringVar(&scanRoot, "root", "", "scan only this project root instead of all configured roots")
	scanCmd.Flags().BoolVar(&scanFull, "full", false, "no-op: every scan is currently a full scan")
	_ = scanCmd.Flags().MarkDeprecated("full", "every scan is currently a full scan; incremental scanning does not exist yet")
}

// resolveScanOptions builds scanner options from the config file (if one
// exists) and the --root override.
func resolveScanOptions() (scanner.Options, error) {
	cfg := config.Default()
	if _, err := os.Stat(configFilePath()); err == nil {
		loaded, err := config.Load(configFilePath())
		if err != nil {
			return scanner.Options{}, fmt.Errorf("load config: %w", err)
		}
		cfg = loaded
	}

	roots := cfg.ProjectRoots
	if scanRoot != "" {
		roots = []string{scanRoot}
	}
	if len(roots) == 0 {
		return scanner.Options{}, fmt.Errorf("no project roots configured; run %q or pass --root", "libra init")
	}

	return scanner.Options{
		Roots:               roots,
		Exclude:             cfg.Exclude,
		MaxDepth:            cfg.Scan.MaxDepth,
		FollowReparsePoints: cfg.Scan.FollowReparsePoints,
	}, nil
}
