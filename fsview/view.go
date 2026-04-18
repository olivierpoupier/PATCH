package fsview

import (
	"fmt"
	"strings"

	"github.com/olivierpoupier/patch/tui/components"

	"charm.land/lipgloss/v2"
)

// View renders the file system view as a full-screen string.
func (m *Model) View(width, height int) string {
	m.width = width
	m.height = height

	headerStr := m.renderHeader(width)
	footerStr := m.renderFooter(width)

	headerHeight := lipgloss.Height(headerStr)
	footerHeight := lipgloss.Height(footerStr)
	bodyHeight := height - headerHeight - footerHeight - 1
	if bodyHeight < 3 {
		bodyHeight = 3
	}

	bodyWidth := width - 1

	var body strings.Builder
	switch {
	case m.atSyntheticRoot():
		m.renderSyntheticRoot(&body, bodyWidth)
	case m.err != nil && len(m.entries) == 0:
		body.WriteString(m.theme.ErrorText.Render(fmt.Sprintf("  %v", m.err)))
		body.WriteString("\n")
	case m.loading:
		body.WriteString(m.theme.Dim.Render("  Loading…"))
		body.WriteString("\n")
	default:
		m.renderListing(&body, bodyWidth)
	}
	bodyStr := body.String()

	for i := 0; i < bodyHeight/2; i++ {
		bodyStr += "\n"
	}

	m.scroll = m.scroll.SetSize(bodyWidth, bodyHeight)
	m.scroll = m.scroll.SetContent(bodyStr)

	activeCursor := m.cursor
	activeLen := len(m.entries)
	if m.atSyntheticRoot() {
		activeCursor = m.rootCursor
		activeLen = len(m.rootEntries)
	}
	if activeLen > 0 {
		m.scroll = m.scroll.ScrollToLine(activeCursor)
	}

	return headerStr + "\n" + m.scroll.View() + "\n" + footerStr
}

func (m *Model) renderHeader(width int) string {
	pathLabel := m.currentDir()
	if m.atSyntheticRoot() {
		pathLabel = "Volumes"
	}
	hidden := "off"
	if m.showHidden {
		hidden = "on"
	}
	count := len(m.entries)
	if m.atSyntheticRoot() {
		count = len(m.rootEntries)
	}

	fields := [][]components.HeaderField{
		{
			{Label: "Path", Value: pathLabel},
			{Label: "Entries", Value: fmt.Sprintf("%d", count)},
			{Label: "Hidden", Value: hidden},
		},
	}
	return components.RenderHeader(m.theme, fields, width)
}

func (m *Model) renderFooter(width int) string {
	var parts []string
	if m.err != nil && len(m.entries) > 0 {
		parts = append(parts, components.RenderError(m.theme, m.err, width))
	}
	if m.status != "" {
		parts = append(parts, m.theme.Dim.Render("  "+m.status))
	}
	bindings := []components.KeyBinding{
		{Keys: "↑↓/jk", Description: "navigate"},
		{Keys: "enter", Description: "open"},
		{Keys: "bksp", Description: "up"},
		{Keys: "c", Description: "copy path"},
		{Keys: "o", Description: "open with"},
		{Keys: ".", Description: "toggle hidden"},
		{Keys: "esc", Description: "close"},
	}
	parts = append(parts, components.RenderFooter(m.theme, bindings, width))
	return strings.Join(parts, "\n")
}

func (m *Model) renderSyntheticRoot(b *strings.Builder, width int) {
	cols := []components.Column{
		{Title: "Volume", Percent: 40},
		{Title: "Mount", Percent: 40},
		{Title: "Used / Total", Percent: 20},
	}
	t := components.NewDeviceTable(m.theme, cols).SetWidth(width).SetFocused(true)

	rows := make([][]string, 0, len(m.rootEntries))
	for _, r := range m.rootEntries {
		usage := "—"
		if r.totalBytes > 0 {
			usage = fmt.Sprintf("%s / %s", formatSize(r.usedBytes), formatSize(r.totalBytes))
		}
		rows = append(rows, []string{r.label, r.mountPoint, usage})
	}
	t = t.SetRows(rows).SetCursor(m.rootCursor)
	b.WriteString(t.View(nil))
}

func (m *Model) renderListing(b *strings.Builder, width int) {
	cols := []components.Column{
		{Title: "Name", Percent: 50},
		{Title: "Size", Percent: 15},
		{Title: "Modified", Percent: 20},
		{Title: "Perms", Percent: 15},
	}
	t := components.NewDeviceTable(m.theme, cols).SetWidth(width).SetFocused(true)

	if len(m.entries) == 0 {
		t = t.SetRows([][]string{{"(empty directory)", "", "", ""}}).SetCursor(0)
		b.WriteString(t.View(nil))
		return
	}

	nameWidth := t.ContentWidth(0)
	if nameWidth < 8 {
		nameWidth = 8
	}

	rows := make([][]string, 0, len(m.entries))
	for _, e := range m.entries {
		name := e.name
		switch {
		case e.isSymlink:
			if e.target != "" {
				name = "↪ " + e.name + " → " + e.target
			} else {
				name = "↪ " + e.name
			}
		case e.isDir:
			name = "▸ " + e.name
		default:
			name = "  " + e.name
		}
		if len([]rune(name)) > nameWidth {
			name = components.TruncateText(name, nameWidth)
		}
		size := "—"
		if !e.isDir {
			size = formatSize(e.size)
		}
		rows = append(rows, []string{
			name,
			size,
			formatModTime(e.modTime),
			formatMode(e.mode),
		})
	}
	t = t.SetRows(rows).SetCursor(m.cursor)
	b.WriteString(t.View(nil))
}
