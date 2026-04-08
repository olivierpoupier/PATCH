package bluetooth

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/olivierpoupier/patch/tui"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
)

// Model is the Bluetooth tab's Bubbletea model.
type Model struct {
	adapter      *CompositeAdapter
	adapterInfo  AdapterInfo
	devices      []DeviceInfo
	cursor       int
	scanning     bool
	err          error
	width        int
	height       int
	active       bool
	focused      bool
	initialized  bool
	done         chan struct{}
	viewport     viewport.Model
	scrollOffset int // marquee scroll position for selected row
	lastCursor   int // track cursor changes to reset scroll
}

// New creates a new Bluetooth tab model.
func New() *Model {
	return &Model{}
}

// Name returns the tab display name.
func (m *Model) Name() string { return "Bluetooth" }

// Init returns the initial command to set up the adapter.
func (m *Model) Init() tea.Cmd {
	return initCompositeAdapter()
}

// Update handles messages and returns the updated tab.
func (m *Model) Update(msg tea.Msg) (tui.Tab, tea.Cmd) {
	switch msg := msg.(type) {
	case initMsg:
		m.adapter = msg.adapter
		m.initialized = true
		if m.done == nil {
			m.done = make(chan struct{})
		}
		cmds := []tea.Cmd{
			startDiscovery(m.adapter),
			fetchAdapterInfo(m.adapter),
			fetchDevices(m.adapter),
		}
		if m.adapter.HasClassic() {
			cmds = append(cmds, listenDeviceEvents(m.done), listenAdapterEvents(m.done))
		}
		m.adapter.StartBLEScan(m.done)
		if m.active {
			cmds = append(cmds, scheduleScanTick(), scheduleBLEScanTick(), scheduleSysProfTick(), scheduleScrollAnimTick())
		}
		return m, tea.Batch(cmds...)

	case adapterInfoMsg:
		m.adapterInfo = AdapterInfo(msg)
		m.err = nil
		return m, nil

	case devicesMsg:
		m.devices = []DeviceInfo(msg)
		m.err = nil
		if m.cursor >= len(m.devices) && len(m.devices) > 0 {
			m.cursor = len(m.devices) - 1
		}
		return m, nil

	case deviceAddedMsg:
		return m, tea.Batch(fetchDevices(m.adapter), listenDeviceEvents(m.done))

	case deviceUpdatedMsg:
		return m, tea.Batch(fetchDevices(m.adapter), listenDeviceEvents(m.done))

	case deviceRemovedMsg:
		return m, tea.Batch(fetchDevices(m.adapter), listenDeviceEvents(m.done))

	case adapterUpdatedMsg:
		return m, tea.Batch(fetchAdapterInfo(m.adapter), listenAdapterEvents(m.done))

	case scanTickMsg:
		if !m.active || m.adapter == nil {
			return m, nil
		}
		return m, tea.Batch(
			fetchDevices(m.adapter),
			scheduleScanTick(),
		)

	case bleScanTickMsg:
		if !m.active || m.adapter == nil {
			return m, nil
		}
		return m, tea.Batch(
			fetchDevices(m.adapter),
			scheduleBLEScanTick(),
		)

	case sysProfTickMsg:
		if !m.active || m.adapter == nil {
			return m, nil
		}
		return m, tea.Batch(
			refreshSystemProfiler(m.adapter),
			scheduleSysProfTick(),
		)

	case scrollAnimTickMsg:
		if !m.active || !m.focused {
			return m, nil
		}
		m.scrollOffset++
		return m, scheduleScrollAnimTick()

	case errorMsg:
		m.err = msg.err
		slog.Warn("bluetooth error displayed", "error", msg.err)
		return m, nil

	case tea.KeyPressMsg:
		if !m.focused || !m.initialized {
			return m, nil
		}
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				m.scrollOffset = 0
			}
		case "down", "j":
			if m.cursor < len(m.devices)-1 {
				m.cursor++
				m.scrollOffset = 0
			}
		case "enter":
			if len(m.devices) > 0 && m.cursor < len(m.devices) {
				dev := m.devices[m.cursor]
				if dev.Connected {
					return m, disconnectDevice(m.adapter, dev)
				}
				return m, connectDevice(m.adapter, dev)
			}
		case "p":
			if m.adapter != nil {
				return m, togglePower(m.adapter)
			}
		}
	}
	return m, nil
}

// SetFocused is called when the tab content gains or loses keyboard focus.
func (m *Model) SetFocused(focused bool) (tui.Tab, tea.Cmd) {
	m.focused = focused
	return m, nil
}

// SetActive is called when the tab becomes the visible tab or is switched away from.
func (m *Model) SetActive(active bool) (tui.Tab, tea.Cmd) {
	m.active = active
	if !m.initialized || m.adapter == nil {
		return m, nil
	}

	if active {
		slog.Info("bluetooth tab activated")
		if m.done == nil {
			m.done = make(chan struct{})
		}
		cmds := []tea.Cmd{
			startDiscovery(m.adapter),
			fetchDevices(m.adapter),
			fetchAdapterInfo(m.adapter),
			scheduleScanTick(),
			scheduleBLEScanTick(),
			scheduleSysProfTick(),
			scheduleScrollAnimTick(),
		}
		if m.adapter.HasClassic() {
			cmds = append(cmds, listenDeviceEvents(m.done), listenAdapterEvents(m.done))
		}
		m.adapter.StartBLEScan(m.done)
		return m, tea.Batch(cmds...)
	}

	slog.Info("bluetooth tab deactivated")
	// Cancel event listeners and stop discovery.
	if m.done != nil {
		close(m.done)
		m.done = nil
	}
	m.adapter.StopBLEScan()
	return m, stopDiscovery(m.adapter)
}

// View renders the tab content with a fixed header, scrollable body, and fixed footer.
func (m *Model) View(width, height int) string {
	if !m.initialized {
		return "  Initializing Bluetooth..."
	}

	if m.err != nil && m.adapter == nil {
		return fmt.Sprintf("  Bluetooth error: %v", m.err)
	}

	// Render header (adapter info).
	var header strings.Builder
	m.renderAdapterInfo(&header, width)
	headerStr := header.String()

	// Render footer (error + help).
	var footer strings.Builder
	if m.err != nil {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4444"))
		footer.WriteString(errStyle.Render(fmt.Sprintf("  Error: %v", m.err)))
		footer.WriteString("\n")
	}
	helpStyle := lipgloss.NewStyle().Foreground(tui.InactiveColor)
	footer.WriteString(helpStyle.Render("  ↑↓/jk navigate  enter connect/disconnect  p toggle power  esc back"))
	footerStr := footer.String()

	// Calculate body height for the viewport.
	headerHeight := lipgloss.Height(headerStr)
	footerHeight := lipgloss.Height(footerStr)
	bodyHeight := height - headerHeight - footerHeight - 1 // -1 for the blank line between header and body
	if bodyHeight < 3 {
		bodyHeight = 3
	}

	// Render the device table as the body content.
	// Reserve 1 char on the right for the scrollbar track.
	tableWidth := width - 1
	var body strings.Builder
	m.renderDeviceTable(&body, tableWidth)
	bodyStr := body.String()
	contentLines := lipgloss.Height(bodyStr)
	// Pad below the table so the last row can scroll up and reveal the bottom border.
	for i := 0; i < bodyHeight/2; i++ {
		bodyStr += "\n"
	}

	// Configure viewport for scrollable body.
	m.viewport.SetWidth(tableWidth)
	m.viewport.SetHeight(bodyHeight)
	m.viewport.SetContent(bodyStr)

	// Auto-scroll to keep the cursor in the middle ~60% of the viewport.
	// This acts like vim's scrolloff: 20% margin on top and bottom.
	if len(m.devices) > 0 {
		rowLine := m.cursor + 2 // +2 for top border + header row
		vpHeight := m.viewport.Height()
		margin := vpHeight / 5 // 20% margin on each side
		if margin < 1 {
			margin = 1
		}
		yOffset := m.viewport.YOffset()
		if rowLine < yOffset+margin {
			m.viewport.SetYOffset(rowLine - margin)
		} else if rowLine >= yOffset+vpHeight-margin {
			m.viewport.SetYOffset(rowLine - vpHeight + margin + 1)
		}
		// Clamp to top.
		if m.viewport.YOffset() < 0 {
			m.viewport.SetYOffset(0)
		}
	}

	// Render viewport with scrollbar overlay.
	vpView := m.viewport.View()
	vpView = m.overlayScrollbar(vpView, bodyHeight, contentLines)

	return headerStr + "\n" + vpView + "\n" + footerStr
}

func (m *Model) renderAdapterInfo(b *strings.Builder, width int) {
	labelStyle := lipgloss.NewStyle().Bold(true).Foreground(tui.ActiveColor)
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	dimStyle := lipgloss.NewStyle().Foreground(tui.InactiveColor)

	name := m.adapterInfo.Name
	if name == "" {
		name = "Unknown"
	}

	powerIndicator := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4444")).Render("● Off")
	if m.adapterInfo.Powered {
		powerIndicator = lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Render("● On")
	}

	b.WriteString(fmt.Sprintf("  %s %s    %s %s\n",
		labelStyle.Render("Adapter:"),
		valueStyle.Render(name),
		labelStyle.Render("Power:"),
		powerIndicator,
	))
	b.WriteString(fmt.Sprintf("  %s %s    %s\n",
		labelStyle.Render("Address:"),
		valueStyle.Render(m.adapterInfo.Address),
		func() string {
			if m.adapterInfo.Discovering {
				return dimStyle.Render("Scanning...")
			}
			return ""
		}(),
	))
}

func (m *Model) renderDeviceTable(b *strings.Builder, width int) {
	// Build row data first so we can measure content widths.
	type rowData struct {
		name, typ, transport, status string
	}
	var rowInfos []rowData

	if len(m.devices) == 0 {
		rowInfos = []rowData{{"No devices found", "", "", ""}}
	} else {
		for _, dev := range m.devices {
			var status string
			if dev.Connected {
				status = "Connected"
			} else if dev.Paired {
				status = "Paired"
			} else {
				status = "Available"
			}

			var transport string
			switch dev.Transport {
			case TransportClassic:
				transport = "Classic"
			case TransportBLE:
				transport = "BLE"
			case TransportDual:
				transport = "Dual"
			}

			rowInfos = append(rowInfos, rowData{dev.Name, dev.Type, transport, status})
		}
	}

	// Measure max content width per column (headers included).
	headers := [4]string{"Name", "Type", "Link", "Status"}
	maxContent := [4]int{}
	for i, h := range headers {
		maxContent[i] = len(h)
	}
	for _, r := range rowInfos {
		cols := [4]string{r.name, r.typ, r.transport, r.status}
		for i, c := range cols {
			if w := len(c); w > maxContent[i] {
				maxContent[i] = w
			}
		}
	}

	// Add horizontal margin: 2 chars on each side.
	const tableMargin = 2
	tableWidth := width - tableMargin*2

	// Width budget: total table width = sum(colWidths) + borderOverhead.
	// Borders: left(1) + right(1) + 3 column separators = 5.
	const borderOverhead = 5
	const cellPadding = 2 // Padding(0, 1) = 1 left + 1 right

	available := tableWidth - borderOverhead
	if available < 40 {
		available = 40
	}

	// Proportional column widths: ~40/20/20/20.
	nameW := available * 40 / 100
	typeW := available * 20 / 100
	transportW := available * 20 / 100
	statusW := available - nameW - typeW - transportW // remainder to avoid rounding loss

	// Enforce minimums.
	if nameW < 10+cellPadding {
		nameW = 10 + cellPadding
	}

	// The usable content width inside the Name column (for truncation).
	nameContentW := nameW - cellPadding

	// Build rows with truncation / marquee for the Name column.
	cursor := m.cursor
	var rows [][]string
	for i, r := range rowInfos {
		name := r.name
		if len(name) > nameContentW {
			if m.focused && i == cursor {
				// Marquee scroll for the selected row.
				name = scrollText(name, nameContentW, m.scrollOffset)
			} else {
				// Truncate with ellipsis.
				name = truncateText(name, nameContentW)
			}
		}
		rows = append(rows, []string{name, r.typ, r.transport, r.status})
	}

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(tui.ActiveColor).Padding(0, 1)
	selectedStyle := lipgloss.NewStyle().Background(lipgloss.Color("#04B575")).Foreground(lipgloss.Color("#000000")).Padding(0, 1)
	normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Padding(0, 1)

	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(tui.ActiveColor)).
		Width(tableWidth).
		Headers(headers[0], headers[1], headers[2], headers[3]).
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			var w int
			switch col {
			case 0:
				w = nameW
			case 1:
				w = typeW
			case 2:
				w = transportW
			default:
				w = statusW
			}

			if row == table.HeaderRow {
				return headerStyle.Width(w)
			}

			if m.focused && row == cursor {
				return selectedStyle.Width(w)
			}

			base := normalStyle.Width(w)
			if len(m.devices) == 0 {
				return base.Foreground(tui.InactiveColor)
			}

			if col == 3 && row < len(m.devices) {
				dev := m.devices[row]
				if dev.Connected {
					return base.Foreground(lipgloss.Color("#04B575"))
				} else if dev.Paired {
					return base.Foreground(lipgloss.Color("#FFAA00"))
				}
				return base.Foreground(tui.InactiveColor)
			}

			return base
		})

	// Indent table with left margin.
	for _, line := range strings.Split(t.String(), "\n") {
		b.WriteString("  ")
		b.WriteString(line)
		b.WriteString("\n")
	}
}

// truncateText truncates s to maxWidth characters, appending "…" if truncated.
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

// scrollText produces a sliding window of visibleWidth over s, creating a marquee effect.
func scrollText(s string, visibleWidth int, offset int) string {
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

// overlayScrollbar renders a scrollbar track on the right edge of the viewport
// content. Only shown when the content is taller than the viewport.
func (m *Model) overlayScrollbar(vpView string, vpHeight int, contentLines int) string {
	totalLines := contentLines
	if totalLines <= vpHeight {
		return vpView // no scrollbar needed
	}

	trackStyle := lipgloss.NewStyle().Foreground(tui.InactiveColor)
	thumbStyle := lipgloss.NewStyle().Foreground(tui.ActiveColor)

	// Calculate thumb size and position.
	thumbSize := vpHeight * vpHeight / totalLines
	if thumbSize < 1 {
		thumbSize = 1
	}
	scrollable := totalLines - vpHeight
	if scrollable < 0 {
		scrollable = 0
	}
	yOffset := m.viewport.YOffset()
	if yOffset > scrollable {
		yOffset = scrollable
	}
	thumbPos := 0
	if scrollable > 0 {
		if yOffset >= scrollable {
			// Pin thumb to the bottom when fully scrolled.
			thumbPos = vpHeight - thumbSize
		} else {
			thumbPos = yOffset * (vpHeight - thumbSize) / scrollable
		}
	}

	lines := strings.Split(vpView, "\n")
	// The viewport output may have a trailing newline producing an extra empty element.
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
