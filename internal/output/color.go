package output

import (
	"io"
	"os"

	isatty "github.com/mattn/go-isatty"
)

// ANSI codes for the three risk tiers: SAFE green, REVIEW orange, BLOCKED
// red. Orange has no standard 8-color slot, so this uses the 256-color
// escape (widely supported on modern terminals, including Windows Terminal).
const (
	ansiGreen  = "\033[32m"
	ansiOrange = "\033[38;5;208m"
	ansiRed    = "\033[31m"
	ansiReset  = "\033[0m"
)

// colorEnabled reports whether w should receive ANSI escapes: never when
// NO_COLOR is set (https://no-color.org), and only when w is a real
// terminal -- piping `libra plan` to a file or another program must stay
// plain text, since escape codes would otherwise pollute redirected output.
func colorEnabled(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
}

// colorText wraps text in code, unless colorEnabled(w) is false, in which
// case it's returned unchanged.
func colorText(w io.Writer, text, code string) string {
	if !colorEnabled(w) {
		return text
	}
	return code + text + ansiReset
}

// riskLevelColor maps a risk tier label to its ANSI code.
func riskLevelColor(label string) string {
	switch label {
	case "SAFE":
		return ansiGreen
	case "REVIEW":
		return ansiOrange
	case "BLOCKED":
		return ansiRed
	default:
		return ""
	}
}
