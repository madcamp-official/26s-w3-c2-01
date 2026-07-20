package output

import (
	"fmt"
	"io"
	"text/tabwriter"

	humanize "github.com/dustin/go-humanize"
)

// [한국어 설명] summary.go는 `libra summary` 명령의 출력 형식
// (SummaryView/SummaryLine)과 텍스트 렌더링(RenderText)을 정의한다.
// cmd/summary.go가 app.SummaryService의 집계 결과를 이 구조체로
// 변환해 넘겨주면, 이 파일은 그것을 tabwriter로 정렬된 표 형태의
// 텍스트로 출력하는 역할만 담당한다. JSON 출력은 여기서 별도로
// 구현하지 않고 format.go의 Printer가 구조체의 json 태그를 이용해
// 공통 처리한다 -- projects.go의 ProjectsView와 동일한 패턴이다.

// SummaryView is the rendered result of `libra summary`: developer storage
// usage broken down by resource type, plus totals by risk level. See F-06 in
// docs/libra_cli_commands_and_schedule.md.
type SummaryView struct {
	Drive           string        `json:"drive,omitempty"`
	ProjectCount    int           `json:"project_count"`
	ResourceCount   int           `json:"resource_count"`
	ResourcesByType []SummaryLine `json:"resources_by_type"`
	SafeReclaimable int64         `json:"safe_reclaimable_bytes"`
	NeedsReview     int64         `json:"needs_review_bytes"`
	Blocked         int64         `json:"blocked_bytes"`
}

// SummaryLine is a single labeled byte total in a SummaryView.
type SummaryLine struct {
	Label string `json:"label"`
	Bytes int64  `json:"bytes"`
}

// RenderText implements Renderable.
func (s SummaryView) RenderText(w io.Writer) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)

	title := "Developer storage"
	if s.Drive != "" {
		title = fmt.Sprintf("%s drive developer storage", s.Drive)
	}
	fmt.Fprintf(tw, "%s\n\n", title)
	fmt.Fprintf(tw, "Projects\t%d\n", s.ProjectCount)
	fmt.Fprintf(tw, "Resources\t%d\n", s.ResourceCount)
	fmt.Fprintf(tw, "\t\n")

	for _, line := range s.ResourcesByType {
		fmt.Fprintf(tw, "%s\t%s\n", line.Label, humanize.Bytes(uint64(line.Bytes)))
	}

	fmt.Fprintf(tw, "\t\n")
	fmt.Fprintf(tw, "Safely reclaimable\t%s\n", humanize.Bytes(uint64(s.SafeReclaimable)))
	fmt.Fprintf(tw, "Needs review\t%s\n", humanize.Bytes(uint64(s.NeedsReview)))
	fmt.Fprintf(tw, "Blocked\t%s\n", humanize.Bytes(uint64(s.Blocked)))

	return tw.Flush()
}
