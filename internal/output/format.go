// Package output renders libra's analysis results as human-readable text or
// machine-readable JSON, so every command can share one --json contract
// instead of formatting output ad hoc.
package output

import (
	"encoding/json"
	"io"
)

// [한국어 설명] format.go는 output 패키지의 공통 렌더링 기반
// (Format/Renderable/Printer)을 정의한다. 각 명령의 결과를 나타내는
// 구체적인 뷰 타입(예: internal/output/projects.go의 ProjectsView,
// internal/output/summary.go의 SummaryView)은 Renderable 인터페이스만
// 구현하면 되고, 텍스트로 보여줄지 JSON으로 보여줄지는 이 파일의
// Printer가 --json 플래그 값에 따라 일괄 처리한다. 즉 이 파일은
// "어떻게 출력하는가"라는 공통 메커니즘을, 나머지 파일들은 "무엇을
// 출력하는가"라는 각 명령별 데이터 구조를 담당한다.

// Format selects how a Printer renders a Renderable.
type Format int

const (
	Text Format = iota
	JSON
)

// Renderable is a result type a Printer can display. Implementations must
// also be safe to encode with encoding/json (exported fields, json tags)
// since the same value is used for both Text and JSON output.
type Renderable interface {
	RenderText(w io.Writer) error
}

// Printer writes a Renderable to Out in its configured Format.
type Printer struct {
	Out    io.Writer
	Format Format
}

// New returns a Printer that writes JSON when jsonOutput is true, text
// otherwise. This mirrors the --json persistent flag every command shares.
func New(w io.Writer, jsonOutput bool) *Printer {
	f := Text
	if jsonOutput {
		f = JSON
	}
	return &Printer{Out: w, Format: f}
}

// Print renders v to the printer's writer in its configured format.
func (p *Printer) Print(v Renderable) error {
	if p.Format == JSON {
		enc := json.NewEncoder(p.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	}
	return v.RenderText(p.Out)
}
