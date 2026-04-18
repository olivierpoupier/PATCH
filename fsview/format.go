package fsview

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// formatSize renders a byte count with a one-decimal unit.
func formatSize(b int64) string {
	const (
		KB = 1000
		MB = KB * 1000
		GB = MB * 1000
		TB = GB * 1000
	)
	switch {
	case b >= TB:
		return fmt.Sprintf("%.1f TB", float64(b)/float64(TB))
	case b >= GB:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(GB))
	case b >= MB:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(MB))
	case b >= KB:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(KB))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// formatMode renders a file mode as "drwxr-xr-x".
func formatMode(m os.FileMode) string {
	return m.String()
}

// formatModTime renders a time as relative (if within 7 days) or YYYY-MM-DD HH:MM.
func formatModTime(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d/time.Minute))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d/time.Hour))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d/(24*time.Hour)))
	default:
		return t.Format("2006-01-02 15:04")
	}
}

// sortEntries filters hidden files (if requested) and sorts dirs-first, then
// case-insensitive alpha by name.
func sortEntries(e []entry, showHidden bool) []entry {
	out := e
	if !showHidden {
		out = out[:0]
		for _, it := range e {
			if strings.HasPrefix(it.name, ".") {
				continue
			}
			out = append(out, it)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].isDir != out[j].isDir {
			return out[i].isDir
		}
		return strings.ToLower(out[i].name) < strings.ToLower(out[j].name)
	})
	return out
}
