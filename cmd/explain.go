// [한국어 설명] `libra explain` 명령을 등록하는 파일이다. 이 브랜치
// 기준으로는 아직 실제 로직이 구현되지 않은 스텁(stub)으로,
// RunE는 "not yet implemented" 메시지만 출력하고 종료한다. 실제 구현
// (리소스/프로젝트 상세 설명, 근거, 위험도, 신뢰도 출력 등)은 아직
// 병합되지 않은 별도 브랜치(PR #24)에서 진행 중이다. 같은 디렉터리의
// impact.go도 동일하게 스텁 상태이며 구조가 거의 같다 -- 두 파일 모두
// 명령 이름/설명 문자열만 다를 뿐, "인자 하나 받아서 미구현 메시지
// 출력" 패턴을 공유한다.
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// explainCmd represents the explain command.
var explainCmd = &cobra.Command{
	Use:   "explain <resource-id-or-path>",
	Short: "Explain what a project or resource is and why it exists",
	Long: `explain describes a single project or resource: its kind, path,
size, when it was created or last modified, which projects reference it,
the evidence behind that dependency, whether it can be regenerated, the
expected impact of deleting it, how to recover it, its risk level, and the
confidence of the analysis.`,
	Example: `  libra explain windows-sdk:10.0.22621.0
  libra explain "D:\Projects\OldWeb\node_modules"
  libra explain project:"D:\Projects\GameClient"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintf(cmd.OutOrStdout(), "explain %s: not yet implemented\n", args[0])
		return nil
	},
}

func init() {
	rootCmd.AddCommand(explainCmd)
}
