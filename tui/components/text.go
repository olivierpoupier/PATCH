package components

import "strings"

// TruncateText truncates s to maxWidth runes, appending "..." if truncated.
func TruncateText(s string, maxWidth int) string {
	if maxWidth <= 1 {
		return "…"
	}
	runes := []rune(s)
	if len(runes) <= maxWidth {
		return s
	}
	return string(runes[:maxWidth-1]) + "…"
}

// PadRight pads s with spaces to width. If s is longer, it is returned as-is.
func PadRight(s string, width int) string {
	r := []rune(s)
	if len(r) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(r))
}

// ScrollText produces a sliding window of visibleWidth over s, creating a marquee effect.
func ScrollText(s string, visibleWidth int, offset int) string {
	runes := []rune(s)
	if len(runes) <= visibleWidth {
		return s
	}
	// Add a gap and repeat to create smooth wraparound.
	gap := []rune("   ")
	looped := append(runes, append(gap, runes...)...)
	total := len(runes) + len(gap)
	start := offset % total
	end := start + visibleWidth
	if end > len(looped) {
		end = len(looped)
	}
	return string(looped[start:end])
}
