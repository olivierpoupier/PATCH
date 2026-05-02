package components

import (
	"strings"

	"github.com/olivierpoupier/patch/tui"
)

// KV is a label/value pair rendered by FieldLine.
type KV struct {
	Label, Value string
}

// FieldLine renders a sequence of "label: value" pairs joined by two spaces,
// using the terminal theme's HeaderKey/HeaderVal styles.
func FieldLine(t *tui.TerminalTheme, parts ...KV) string {
	var segs []string
	for _, p := range parts {
		segs = append(segs, t.HeaderKey.Render(p.Label+":")+" "+t.HeaderVal.Render(p.Value))
	}
	return strings.Join(segs, "  ")
}

// Indent prefixes every line of s with n spaces.
func Indent(s string, n int) string {
	if n <= 0 {
		return s
	}
	pad := strings.Repeat(" ", n)
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		lines[i] = pad + ln
	}
	return strings.Join(lines, "\n")
}

// OrDash returns s, or "—" if s is empty. Used for header values that may be
// missing.
func OrDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
