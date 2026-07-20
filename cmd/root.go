// [한국어 설명] root.go는 Cobra 명령 트리의 진입점(entrypoint)이다.
// main.go는 이 패키지의 Execute() 함수 하나만 호출하며, 나머지
// cmd/*.go 파일(explain.go, impact.go, init.go, projects.go,
// scan.go, summary.go 등)은 각자의 init()에서 rootCmd.AddCommand로
// 자기 자신을 등록한다. 또한 --config, --json, --verbose 등 모든
// 하위 명령이 공유하는 전역 persistent flag 값도 이 파일에서 선언되며,
// 각 명령 파일은 이 변수들을 그대로 읽어 쓸 뿐 다시 정의하지 않는다.
package cmd

import "github.com/spf13/cobra"

// root.go wires the Cobra command tree's entrypoint; every other cmd/*.go
// file registers itself onto rootCmd from its own init(). Execute() is the
// only symbol main.go calls into this package.
//
// Global flag values shared by every subcommand. Populated by rootCmd's
// persistent flags; subcommands read these instead of redefining them.
var (
	cfgPath    string
	jsonOutput bool
	verbose    bool
	noColor    bool
	dryRun     bool
	assumeYes  bool
)

var rootCmd = &cobra.Command{
	Use:   "libra",
	Short: "Analyze and manage local developer storage",
	Long: `libra analyzes the dependency relationships between local development
projects and the SDKs, tools, caches, and build artifacts on disk. It explains
what is taking up space and what breaks if you delete it.

libra is read-only by default: scan, summary, explain, and impact never
modify the filesystem. Cleanup commands require an explicit action and
default to --dry-run.`,
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	flags := rootCmd.PersistentFlags()
	flags.StringVar(&cfgPath, "config", "", "path to libra config file (default: .libra.yaml)")
	flags.BoolVar(&jsonOutput, "json", false, "output machine-readable JSON instead of text")
	flags.BoolVar(&verbose, "verbose", false, "print additional diagnostic detail")
	flags.BoolVar(&noColor, "no-color", false, "disable ANSI color in text output")
	flags.BoolVar(&dryRun, "dry-run", false, "show what would happen without changing anything")
	flags.BoolVar(&assumeYes, "yes", false, "skip interactive confirmation prompts")
}
