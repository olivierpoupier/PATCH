package audio

import (
	"fmt"
	"image/color"
	"log/slog"
	"strings"

	"github.com/olivierpoupier/patch/tui"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
)

type pane int

const (
	sourcePane pane = iota
	outputPane
)

// Model is the Audio tab's Bubbletea model.
type Model struct {
	backend      Backend
	sources      []Source
	outputs      []Output
	activePane   pane
	sourceCursor int
	outputCursor int
	err          error
	width        int
	height       int
	active       bool
	focused      bool
	initialized  bool
	done         chan struct{}
	sourceVP     viewport.Model
	outputVP     viewport.Model
}

// New creates a new Audio tab model.
func New() *Model {
	return &Model{}
}

// Name returns the tab display name.
func (m *Model) Name() string { return "Audio" }

// Init returns the initial command to set up the audio backend.
func (m *Model) Init() tea.Cmd {
	return initBackendCmd()
}

// Update handles messages and returns the updated tab.
func (m *Model) Update(msg tea.Msg) (tui.Tab, tea.Cmd) {
	switch msg := msg.(type) {
	case initBackendMsg:
		m.backend = msg.backend
		m.initialized = true
		if m.done == nil {
			m.done = make(chan struct{})
		}
		cmds := []tea.Cmd{
			fetchSources(m.backend),
			fetchOutputs(m.backend),
		}
		if m.active {
			cmds = append(cmds, scheduleRefreshTick())
		}
		return m, tea.Batch(cmds...)

	case sourcesMsg:
		m.sources = msg.sources
		m.err = nil
		apps := m.appSources()
		if m.sourceCursor >= len(apps) && len(apps) > 0 {
			m.sourceCursor = len(apps) - 1
		}
		return m, nil

	case outputsMsg:
		m.outputs = msg.outputs
		m.err = nil
		if m.outputCursor >= len(m.outputs) && len(m.outputs) > 0 {
			m.outputCursor = len(m.outputs) - 1
		}
		return m, nil

	case refreshTickMsg:
		if !m.active || m.backend == nil {
			return m, nil
		}
		return m, tea.Batch(
			fetchSources(m.backend),
			fetchOutputs(m.backend),
			scheduleRefreshTick(),
		)

	case errorMsg:
		m.err = msg.err
		slog.Warn("audio error displayed", "error", msg.err)
		return m, nil

	case tea.KeyPressMsg:
		if !m.focused || !m.initialized {
			return m, nil
		}
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *Model) handleKey(msg tea.KeyPressMsg) (tui.Tab, tea.Cmd) {
	apps := m.appSources()

	switch msg.String() {
	case "h":
		m.activePane = sourcePane
	case "l":
		m.activePane = outputPane
	case "j", "down":
		m.moveCursorDown()
	case "k", "up":
		m.moveCursorUp()
	case "+", "=":
		// Volume always controls system audio.
		if sys := m.systemSource(); sys != nil {
			newVol := sys.Volume + 0.05
			if newVol > 1.0 {
				newVol = 1.0
			}
			return m, setVolumeCmd(m.backend, "system", newVol)
		}
	case "-":
		if sys := m.systemSource(); sys != nil {
			newVol := sys.Volume - 0.05
			if newVol < 0.0 {
				newVol = 0.0
			}
			return m, setVolumeCmd(m.backend, "system", newVol)
		}
	case "m":
		// Mute always controls system audio.
		if sys := m.systemSource(); sys != nil {
			return m, toggleMuteCmd(m.backend, "system")
		}
	case "space":
		if m.activePane == sourcePane {
			if len(apps) > 0 && m.sourceCursor < len(apps) {
				return m, togglePauseSourceCmd(m.backend, apps[m.sourceCursor].ID)
			}
			// No app sources: toggle active media player.
			if m.backend != nil {
				return m, togglePauseSourceCmd(m.backend, "active")
			}
		}
		if m.activePane == outputPane && len(m.outputs) > 0 {
			out := m.outputs[m.outputCursor]
			return m, toggleOutputCmd(m.backend, out.ID, !out.Active)
		}
	case "shift+space":
		if m.backend != nil {
			return m, pauseAllCmd(m.backend)
		}
	case "enter":
		if m.activePane == outputPane && len(m.outputs) > 0 {
			out := m.outputs[m.outputCursor]
			return m, toggleOutputCmd(m.backend, out.ID, !out.Active)
		}
	}
	return m, nil
}

func (m *Model) moveCursorDown() {
	switch m.activePane {
	case sourcePane:
		apps := m.appSources()
		if m.sourceCursor < len(apps)-1 {
			m.sourceCursor++
		}
	case outputPane:
		if m.outputCursor < len(m.outputs)-1 {
			m.outputCursor++
		}
	}
}

func (m *Model) moveCursorUp() {
	switch m.activePane {
	case sourcePane:
		if m.sourceCursor > 0 {
			m.sourceCursor--
		}
	case outputPane:
		if m.outputCursor > 0 {
			m.outputCursor--
		}
	}
}

// SetFocused is called when the tab content gains or loses keyboard focus.
func (m *Model) SetFocused(focused bool) (tui.Tab, tea.Cmd) {
	m.focused = focused
	return m, nil
}

// SetActive is called when the tab becomes visible or is switched away from.
func (m *Model) SetActive(active bool) (tui.Tab, tea.Cmd) {
	m.active = active
	if !m.initialized || m.backend == nil {
		return m, nil
	}

	if active {
		slog.Info("audio tab activated")
		if m.done == nil {
			m.done = make(chan struct{})
		}
		return m, tea.Batch(
			fetchSources(m.backend),
			fetchOutputs(m.backend),
			scheduleRefreshTick(),
		)
	}

	slog.Info("audio tab deactivated")
	if m.done != nil {
		close(m.done)
		m.done = nil
	}
	return m, nil
}

// systemSource returns the system volume source, or nil if not found.
func (m *Model) systemSource() *Source {
	for i := range m.sources {
		if m.sources[i].IsSystem {
			return &m.sources[i]
		}
	}
	return nil
}

// appSources returns all non-system sources.
func (m *Model) appSources() []Source {
	var apps []Source
	for _, s := range m.sources {
		if !s.IsSystem {
			apps = append(apps, s)
		}
	}
	return apps
}

// renderHeaderBar renders the system volume + play status line above the panes.
func (m *Model) renderHeaderBar(width int) string {
	sys := m.systemSource()

	// Left side: volume icon + bar + percentage.
	var left string
	if sys != nil {
		icon := "🔊"
		if sys.Muted {
			icon = "🔇"
		}
		barWidth := 10
		volBar := renderVolumeBar(sys.Volume, barWidth)
		pct := fmt.Sprintf("%3d%%", int(sys.Volume*100))
		left = fmt.Sprintf("  %s %s %s", icon, volBar, pct)
	} else {
		left = "  🔊 --"
	}

	// Right side: play/pause status.
	anyPlaying := false
	for _, s := range m.sources {
		if !s.IsSystem && s.Playing {
			anyPlaying = true
			break
		}
	}
	var right string
	if anyPlaying {
		playStyle := lipgloss.NewStyle().Foreground(tui.ActiveColor)
		right = playStyle.Render("▶ Playing")
	} else {
		pauseStyle := lipgloss.NewStyle().Foreground(tui.InactiveColor)
		right = pauseStyle.Render("⏸ Paused")
	}

	// Join left and right with spacing.
	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	gap := width - leftWidth - rightWidth
	if gap < 1 {
		gap = 1
	}

	return left + strings.Repeat(" ", gap) + right
}

// View renders the header bar + two-pane layout: Sources (left) and Outputs (right).
func (m *Model) View(width, height int) string {
	m.width = width
	m.height = height

	if !m.initialized {
		return "  Initializing Audio..."
	}

	if m.err != nil && m.backend == nil {
		return fmt.Sprintf("  Audio error: %v", m.err)
	}

	// Footer.
	var footer strings.Builder
	if m.err != nil {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4444"))
		footer.WriteString(errStyle.Render(fmt.Sprintf("  Error: %v", m.err)))
		footer.WriteString("\n")
	}
	helpStyle := lipgloss.NewStyle().Foreground(tui.InactiveColor)
	footer.WriteString(helpStyle.Render("  h/l pane  j/k scroll  +/- volume  space play/pause  S-space pause all  m mute  esc back"))
	footerStr := footer.String()
	footerHeight := lipgloss.Height(footerStr)

	// Header bar with system volume.
	headerStr := m.renderHeaderBar(width)
	headerHeight := lipgloss.Height(headerStr)

	// Available height for panes.
	paneHeight := height - footerHeight - headerHeight - 1
	if paneHeight < 5 {
		paneHeight = 5
	}

	// Pane widths.
	const gap = 1
	leftWidth := (width - gap) * 55 / 100
	rightWidth := width - gap - leftWidth

	leftView := m.renderSourcePane(leftWidth, paneHeight)
	rightView := m.renderOutputPane(rightWidth, paneHeight)

	panes := lipgloss.JoinHorizontal(lipgloss.Top, leftView, " ", rightView)

	return headerStr + "\n" + panes + "\n" + footerStr
}

func (m *Model) renderSourcePane(width, height int) string {
	focused := m.focused && m.activePane == sourcePane

	// Border style.
	borderColor := tui.InactiveColor
	if focused {
		borderColor = tui.ActiveColor
	}

	// Title.
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(borderColor)
	title := titleStyle.Render(" Sources ")

	// Only show app sources (system volume is in the header bar).
	apps := m.appSources()

	// Content.
	contentWidth := width - 4 // border(2) + padding(2)
	var content strings.Builder

	if len(apps) == 0 {
		dimStyle := lipgloss.NewStyle().Foreground(tui.InactiveColor)
		content.WriteString(dimStyle.Render("No active audio sources"))
	} else {
		for i, src := range apps {
			line := m.renderSourceRow(i, src, contentWidth, focused)
			content.WriteString(line)
			if i < len(apps)-1 {
				content.WriteString("\n")
			}
		}
	}

	// Configure viewport for scrollable content.
	bodyHeight := height - 4 // border(2) + title overhead(2)
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	m.sourceVP.SetWidth(contentWidth)
	m.sourceVP.SetHeight(bodyHeight)
	m.sourceVP.SetContent(content.String())

	// Auto-scroll to keep cursor visible.
	if len(apps) > 0 {
		m.autoScroll(&m.sourceVP, m.sourceCursor, bodyHeight)
	}

	vpView := m.sourceVP.View()
	contentLines := strings.Count(content.String(), "\n") + 1
	vpView = m.overlayScrollbar(vpView, &m.sourceVP, bodyHeight, contentLines)

	// Build the pane with a custom top border that includes the title.
	return m.renderPaneWithTitle(title, vpView, width, height, borderColor)
}

func (m *Model) renderOutputPane(width, height int) string {
	focused := m.focused && m.activePane == outputPane

	borderColor := tui.InactiveColor
	if focused {
		borderColor = tui.ActiveColor
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(borderColor)
	title := titleStyle.Render(" Outputs ")

	contentWidth := width - 4
	var content strings.Builder

	if len(m.outputs) == 0 {
		dimStyle := lipgloss.NewStyle().Foreground(tui.InactiveColor)
		content.WriteString(dimStyle.Render("No audio outputs"))
	} else {
		for i, out := range m.outputs {
			line := m.renderOutputRow(i, out, contentWidth, focused)
			content.WriteString(line)
			if i < len(m.outputs)-1 {
				content.WriteString("\n")
			}
		}
	}

	bodyHeight := height - 4
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	m.outputVP.SetWidth(contentWidth)
	m.outputVP.SetHeight(bodyHeight)
	m.outputVP.SetContent(content.String())

	if len(m.outputs) > 0 {
		m.autoScroll(&m.outputVP, m.outputCursor, bodyHeight)
	}

	vpView := m.outputVP.View()
	contentLines := strings.Count(content.String(), "\n") + 1
	vpView = m.overlayScrollbar(vpView, &m.outputVP, bodyHeight, contentLines)

	return m.renderPaneWithTitle(title, vpView, width, height, borderColor)
}

func (m *Model) renderSourceRow(idx int, src Source, width int, paneFocused bool) string {
	selected := paneFocused && idx == m.sourceCursor

	// Cursor indicator.
	cursor := "  "
	if selected {
		cursor = "▸ "
	}

	// Play/pause icon.
	playIcon := "⏸"
	if src.Playing {
		playIcon = "▶"
	}
	if src.Muted {
		playIcon = "🔇"
	}

	var line string
	if src.VolumeControllable {
		// Full layout: cursor + name + icon + volume bar + percentage.
		barWidth := 8
		volStr := renderVolumeBar(src.Volume, barWidth)
		// Layout: cursor(2) + name + " " + icon(1) + " " + bar(barWidth) + " " + pct(4)
		fixedWidth := 2 + 1 + 1 + barWidth + 1 + 4
		nameWidth := width - fixedWidth
		if nameWidth < 4 {
			nameWidth = 4
		}
		name := padOrTruncate(src.Name, nameWidth)
		pct := fmt.Sprintf("%3d%%", int(src.Volume*100))
		line = fmt.Sprintf("%s%s %s %s %s", cursor, name, playIcon, volStr, pct)
	} else {
		// Compact layout: cursor + name + icon (no volume bar).
		// Layout: cursor(2) + name + " " + icon(1)
		fixedWidth := 2 + 1 + 1
		nameWidth := width - fixedWidth
		if nameWidth < 4 {
			nameWidth = 4
		}
		name := padOrTruncate(src.Name, nameWidth)
		line = fmt.Sprintf("%s%s %s", cursor, name, playIcon)
	}

	if selected {
		selectedStyle := lipgloss.NewStyle().
			Background(tui.ActiveColor).
			Foreground(lipgloss.Color("#000000"))
		return selectedStyle.Render(line)
	}

	normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	if !src.Playing {
		normalStyle = normalStyle.Foreground(tui.InactiveColor)
	}
	return normalStyle.Render(line)
}

// padOrTruncate pads or truncates a string to exactly the given width.
func padOrTruncate(s string, width int) string {
	runes := []rune(s)
	if len(runes) > width {
		return truncateText(s, width)
	}
	return s + strings.Repeat(" ", width-len(runes))
}

func (m *Model) renderOutputRow(idx int, out Output, width int, paneFocused bool) string {
	selected := paneFocused && idx == m.outputCursor

	cursor := "  "
	if selected {
		cursor = "▸ "
	}

	// Checkbox.
	checkbox := "[ ]"
	if out.Active {
		checkbox = "[x]"
	}

	// Active indicator.
	indicator := "  "
	if out.Active {
		indicatorStyle := lipgloss.NewStyle().Foreground(tui.ActiveColor)
		indicator = indicatorStyle.Render("●") + " "
	}

	// AirPlay label.
	airplayLabel := ""
	if out.IsAirPlay {
		airplayLabel = " (AirPlay)"
	}

	// Name: fill remaining space.
	// Layout: cursor(2) + checkbox(3) + " " + name + airplay + " " + indicator(2)
	fixedWidth := 2 + 3 + 1 + len(airplayLabel) + 1 + 2
	nameWidth := width - fixedWidth
	if nameWidth < 4 {
		nameWidth = 4
	}

	name := out.Name
	if len([]rune(name)) > nameWidth {
		name = truncateText(name, nameWidth)
	} else {
		name = name + strings.Repeat(" ", nameWidth-len([]rune(name)))
	}

	line := fmt.Sprintf("%s%s %s%s %s", cursor, checkbox, name, airplayLabel, indicator)

	if selected {
		selectedStyle := lipgloss.NewStyle().
			Background(tui.ActiveColor).
			Foreground(lipgloss.Color("#000000"))
		return selectedStyle.Render(line)
	}

	normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	if !out.Active {
		normalStyle = normalStyle.Foreground(tui.InactiveColor)
	}
	return normalStyle.Render(line)
}

func renderVolumeBar(volume float32, barWidth int) string {
	filled := int(volume * float32(barWidth))
	if filled > barWidth {
		filled = barWidth
	}
	empty := barWidth - filled
	return strings.Repeat("█", filled) + strings.Repeat("░", empty)
}

// renderPaneWithTitle draws a bordered pane with a title embedded in the top border.
func (m *Model) renderPaneWithTitle(title, content string, width, height int, borderColor color.Color) string {
	bs := lipgloss.RoundedBorder()
	bStyle := lipgloss.NewStyle().Foreground(borderColor)

	// Top border with title.
	titleLen := lipgloss.Width(title)
	topBarLen := width - 2 - titleLen - 1 // -2 for corners, -1 for left bar segment
	if topBarLen < 0 {
		topBarLen = 0
	}
	topBorder := bStyle.Render(bs.TopLeft+bs.Top) + title + bStyle.Render(strings.Repeat(bs.Top, topBarLen)+bs.TopRight)

	// Content lines with side borders.
	contentLines := strings.Split(content, "\n")
	innerWidth := width - 2 // minus left+right border
	var body strings.Builder
	for _, line := range contentLines {
		lineWidth := lipgloss.Width(line)
		pad := innerWidth - lineWidth
		if pad < 0 {
			pad = 0
		}
		body.WriteString(bStyle.Render(bs.Left))
		body.WriteString(line)
		body.WriteString(strings.Repeat(" ", pad))
		body.WriteString(bStyle.Render(bs.Right))
		body.WriteString("\n")
	}

	// Bottom border.
	bottomBorder := bStyle.Render(bs.BottomLeft + strings.Repeat(bs.Bottom, innerWidth) + bs.BottomRight)

	return topBorder + "\n" + body.String() + bottomBorder
}

func (m *Model) autoScroll(vp *viewport.Model, cursor int, vpHeight int) {
	margin := vpHeight / 5
	if margin < 1 {
		margin = 1
	}
	yOffset := vp.YOffset()
	if cursor < yOffset+margin {
		vp.SetYOffset(cursor - margin)
	} else if cursor >= yOffset+vpHeight-margin {
		vp.SetYOffset(cursor - vpHeight + margin + 1)
	}
	if vp.YOffset() < 0 {
		vp.SetYOffset(0)
	}
}

// overlayScrollbar renders a scrollbar track on the right edge of the viewport content.
func (m *Model) overlayScrollbar(vpView string, vp *viewport.Model, vpHeight int, contentLines int) string {
	if contentLines <= vpHeight {
		return vpView
	}

	trackStyle := lipgloss.NewStyle().Foreground(tui.InactiveColor)
	thumbStyle := lipgloss.NewStyle().Foreground(tui.ActiveColor)

	thumbSize := vpHeight * vpHeight / contentLines
	if thumbSize < 1 {
		thumbSize = 1
	}
	scrollable := contentLines - vpHeight
	if scrollable < 0 {
		scrollable = 0
	}
	yOffset := vp.YOffset()
	if yOffset > scrollable {
		yOffset = scrollable
	}
	thumbPos := 0
	if scrollable > 0 {
		if yOffset >= scrollable {
			thumbPos = vpHeight - thumbSize
		} else {
			thumbPos = yOffset * (vpHeight - thumbSize) / scrollable
		}
	}

	lines := strings.Split(vpView, "\n")
	if len(lines) > vpHeight {
		lines = lines[:vpHeight]
	}

	var b strings.Builder
	for i := 0; i < len(lines); i++ {
		b.WriteString(lines[i])
		if i >= thumbPos && i < thumbPos+thumbSize {
			b.WriteString(thumbStyle.Render("┃"))
		} else {
			b.WriteString(trackStyle.Render("│"))
		}
		if i < len(lines)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// truncateText truncates s to maxWidth characters, appending "..." if truncated.
func truncateText(s string, maxWidth int) string {
	if maxWidth <= 1 {
		return "…"
	}
	runes := []rune(s)
	if len(runes) <= maxWidth {
		return s
	}
	return string(runes[:maxWidth-1]) + "…"
}
