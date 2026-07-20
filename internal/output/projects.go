package output

import (
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	humanize "github.com/dustin/go-humanize"

	"github.com/madcamp-official/26s-w3-c2-01/internal/domain"
)

// [한국어 설명] projects.go는 `libra projects` 명령의 출력 형식
// (ProjectsView/ProjectLine)과 텍스트 렌더링(RenderText)을 정의한다.
// JSON 렌더링은 별도로 구현하지 않는데, format.go의 Printer가
// encoding/json으로 이 구조체를 그대로 인코딩해 주기 때문이다(구조체
// 필드에 붙은 json 태그가 그 계약). 이 파일의 formatTime 헬퍼는
// 이름과 달리 ProjectsView 전용이 아니라 output 패키지 전체에서
// 타임스탬프를 표시하는 모든 뷰(explain/impact 등)가 공유하도록
// 의도된 함수이며, 단지 ProjectsView가 첫 사용처였기 때문에 이
// 파일에 있을 뿐이다.

// ProjectsView is the rendered result of `libra projects`: every discovered
// project and its activity state. See F-03/3.4 in
// docs/libra_cli_commands_and_schedule.md.
type ProjectsView struct {
	Projects []ProjectLine `json:"projects"`
}

// ProjectLine is a single project row in a ProjectsView.
type ProjectLine struct {
	Name           string               `json:"name"`
	Path           string               `json:"path"`
	Type           domain.ProjectType   `json:"type"`
	Drive          string               `json:"drive,omitempty"`
	LogicalSize    int64                `json:"logical_size_bytes"`
	LastModifiedAt time.Time            `json:"last_modified_at,omitempty"`
	LastObservedAt time.Time            `json:"last_observed_at"`
	Status         domain.ProjectStatus `json:"status"`
	ResourceCount  int                  `json:"resource_count"`
}

// RenderText implements Renderable.
func (v ProjectsView) RenderText(w io.Writer) error {
	if len(v.Projects) == 0 {
		fmt.Fprintln(w, "No projects found. Run `libra scan` first.")
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tTYPE\tDRIVE\tSIZE\tSTATUS\tRESOURCES\tMODIFIED\tPATH")
	for _, p := range v.Projects {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%d\t%s\t%s\n",
			p.Name, p.Type, p.Drive, humanize.Bytes(uint64(p.LogicalSize)),
			p.Status, p.ResourceCount, formatTime(p.LastModifiedAt), p.Path)
	}
	return tw.Flush()
}

// formatTime is shared by every view in this package that renders a
// timestamp (ProjectLine here, plus explain/impact views), not just
// ProjectsView -- it lives in this file because ProjectsView was the first
// caller, not because it's projects-specific.
func formatTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format("2006-01-02")
}
