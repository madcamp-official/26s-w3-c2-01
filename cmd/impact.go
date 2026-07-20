// [한국어 설명] `libra impact` 명령을 등록하는 파일이다. explain.go와
// 마찬가지로 이 브랜치 기준으로는 아직 실제 로직이 구현되지 않은
// 스텁(stub)이며, RunE는 "not yet implemented" 메시지만 출력한다.
// 실제 구현(리소스 제거 시 영향 분석: 실행 파일 동작 여부, 재빌드
// 필요 여부, IDE 디버깅 가능 여부, 복구 방법, CI 참조 여부 등)은
// 아직 병합되지 않은 별도 브랜치(PR #24)에 있다. explain.go와 거의
// 동일한 스텁 구조를 공유하는 자매 파일이다.
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// impactCmd represents the impact command.
var impactCmd = &cobra.Command{
	Use:   "impact <resource-id-or-path>",
	Short: "Show what breaks if a resource is removed",
	Long: `impact analyzes what happens to affected projects if a resource is
removed: whether already-built executables can still run, whether the
project rebuilds, whether IDE debugging still works, how to restore the
dependency, and any CI configuration that references it.`,
	Example: `  libra impact windows-sdk:10.0.22621.0
  libra impact "C:\Program Files (x86)\Windows Kits\10\Lib\10.0.22621.0"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintf(cmd.OutOrStdout(), "impact %s: not yet implemented\n", args[0])
		return nil
	},
}

func init() {
	rootCmd.AddCommand(impactCmd)
}
