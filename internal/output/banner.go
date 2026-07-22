package output

import (
	_ "embed"
	"io"
)

// bannerArt is the project mascot rendered as half-block Unicode + 24-bit
// ANSI color (via `chafa --format symbols --symbols block -c full`), shown
// once on a genuine first-time `libra init`. Regenerate by re-running chafa
// against the source sprite and overwriting banner.ans.
//
//go:embed banner.ans
var bannerArt string

// printBanner writes the mascot banner to w, unless w won't render color
// (colorEnabled covers both NO_COLOR and non-terminal writers) -- piping
// `libra init` to a file or another program must stay plain text.
func printBanner(w io.Writer) {
	if !colorEnabled(w) {
		return
	}
	io.WriteString(w, bannerArt)
}
