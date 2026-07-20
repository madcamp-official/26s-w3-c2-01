// [한국어 설명] `libra summary` 명령을 등록하는 파일이다. cmd
// 패키지에서 projects.go 등 다른 읽기 전용 명령들과 달리 유일하게
// application service(app.SummaryService)를 거쳐 집계를 수행한다
// -- 부수효과가 있어서가 아니라 순수 집계 로직이기 때문. 리소스
// 타입을 사람이 읽기 좋은 라벨로 바꾸는 resourceTypeLabels 매핑과
// resourceTypeLabel 헬퍼도 이 파일에 있으며, 실제 텍스트/JSON 렌더링
// 형식은 internal/output/summary.go의 SummaryView가 담당한다(이
// 파일은 도메인 데이터를 그 뷰 구조체로 변환만 한다).
package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/madcamp-official/26s-w3-c2-01/internal/app"
	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
	"github.com/madcamp-official/26s-w3-c2-01/internal/output"
	"github.com/madcamp-official/26s-w3-c2-01/internal/store/sqlite"
	"github.com/spf13/cobra"
)

// summary.go is the one command that goes through an application service
// (app.SummaryService) purely for aggregation, not because summary has
// side effects -- contrast with cmd/projects.go/cmd/resources.go, which
// skip that layer for the same kind of read-only work.
var (
	summaryDrive string
	summaryType  string
)

// resourceTypeLabels maps domain.ResourceType to the display label used in
// `libra summary` (F-06 in docs/libra_cli_commands_and_schedule.md).
var resourceTypeLabels = map[domain.ResourceType]string{
	domain.ResourceTypeWindowsSDK:   "Windows SDKs",
	domain.ResourceTypeNetFXSDK:     ".NET Framework SDKs",
	domain.ResourceTypeVisualStudio: "Visual Studio tools",
	domain.ResourceTypeMSBuild:      "MSBuild",
	domain.ResourceTypeDotNetSDK:    ".NET SDKs",
	domain.ResourceTypeNodeModules:  "Node project artifacts",
	domain.ResourceTypeBuildOutput:  "Build outputs",
	domain.ResourceTypeGlobalCache:  "Global package caches",
	domain.ResourceTypeDockerCache:  "Docker cache",
}

func resourceTypeLabel(t domain.ResourceType) string {
	if label, ok := resourceTypeLabels[t]; ok {
		return label
	}
	return string(t)
}

// summaryCmd represents the summary command.
var summaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Summarize developer storage usage and reclaimable space",
	Long: `summary reports project and resource counts, storage usage broken
down by resource type and drive, and how much space is safely reclaimable,
needs review, or is blocked from cleanup.`,
	Example: `  libra summary
  libra summary --drive C:
  libra summary --type node-modules`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := openDatabase()
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer db.Close()

		service := app.NewSummaryService(sqlite.NewProjectRepository(db), sqlite.NewResourceRepository(db))
		summary, err := service.Summarize(cmd.Context(), func(resource domain.Resource) bool {
			if summaryType != "" && string(resource.Type) != summaryType {
				return false
			}
			if summaryDrive != "" && !strings.EqualFold(filepath.VolumeName(resource.DisplayPath), summaryDrive) {
				return false
			}
			return true
		})
		if err != nil {
			return fmt.Errorf("summarize: %w", err)
		}

		view := output.SummaryView{
			Drive:           summaryDrive,
			ProjectCount:    summary.ProjectCount,
			ResourceCount:   summary.ResourceCount,
			SafeReclaimable: summary.SafeReclaimable,
			NeedsReview:     summary.NeedsReview,
			Blocked:         summary.Blocked,
		}
		for _, line := range summary.ResourcesByType {
			view.ResourcesByType = append(view.ResourcesByType, output.SummaryLine{
				Label: resourceTypeLabel(line.Type),
				Bytes: line.Bytes,
			})
		}
		return output.New(cmd.OutOrStdout(), jsonOutput).Print(view)
	},
}

func init() {
	rootCmd.AddCommand(summaryCmd)

	summaryCmd.Flags().StringVar(&summaryDrive, "drive", "", "limit the summary to this drive (e.g. C:)")
	summaryCmd.Flags().StringVar(&summaryType, "type", "", "limit the summary to this resource type (e.g. node-modules)")
}
