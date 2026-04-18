package bluetooth

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/olivierpoupier/patch/tui"
	"github.com/olivierpoupier/patch/tui/components"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
)

// Model is the Bluetooth tab's Bubbletea model.
type Model struct {
	theme        *tui.Theme
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
	scroll       components.ScrollableView
	scrollOffset int // marquee scroll position for selected row
	lastCursor   int // track cursor changes to reset scroll
}

// New creates a new Bluetooth tab model.
func New(theme *tui.Theme) *Model {
	return &Model{
		theme:  theme,
		scroll: components.NewScrollableView(theme),
	}
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
		return m.theme.Dim.Render("  Initializing Bluetooth...")
	}

	if m.err != nil && m.adapter == nil {
		return m.theme.ErrorText.Render(fmt.Sprintf("  Bluetooth error: %v", m.err))
	}

	// Render header (adapter info).
	var header strings.Builder
	m.renderAdapterInfo(&header, width)
	headerStr := header.String()

	// Render footer (error + help).
	var footerParts []string
	if m.err != nil {
		footerParts = append(footerParts, components.RenderError(m.theme, m.err, width))
	}
	bindings := []components.KeyBinding{
		{Keys: "↑↓/jk", Description: "navigate"},
		{Keys: "enter", Description: "connect/disconnect"},
		{Keys: "p", Description: "toggle power"},
		{Keys: "esc", Description: "back"},
	}
	footerParts = append(footerParts, components.RenderFooter(m.theme, bindings, width))
	footerStr := strings.Join(footerParts, "\n")

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
	// Pad below the table so the last row can scroll up and reveal the bottom border.
	for i := 0; i < bodyHeight/2; i++ {
		bodyStr += "\n"
	}

	// Configure scrollable view.
	m.scroll = m.scroll.SetSize(tableWidth, bodyHeight)
	m.scroll = m.scroll.SetContent(bodyStr)

	// Auto-scroll to keep the cursor in view.
	if len(m.devices) > 0 {
		rowLine := m.cursor + 2 // +2 for top border + header row
		m.scroll = m.scroll.ScrollToLine(rowLine)
	}

	// Render viewport with scrollbar overlay.
	vpView := m.scroll.View()

	return headerStr + "\n" + vpView + "\n" + footerStr
}

func (m *Model) renderAdapterInfo(b *strings.Builder, width int) {
	theme := m.theme

	name := m.adapterInfo.Name
	if name == "" {
		name = "Unknown"
	}

	powerColor := theme.Error
	powerText := "● Off"
	if m.adapterInfo.Powered {
		powerColor = theme.Active
		powerText = "● On"
	}

	b.WriteString(fmt.Sprintf("  %s %s    %s %s\n",
		theme.Label.Render("Adapter:"),
		theme.Value.Render(name),
		theme.Label.Render("Power:"),
		lipgloss.NewStyle().Foreground(powerColor).Render(powerText),
	))
	b.WriteString(fmt.Sprintf("  %s %s    %s\n",
		theme.Label.Render("Address:"),
		theme.Value.Render(m.adapterInfo.Address),
		func() string {
			if m.adapterInfo.Discovering {
				return theme.Dim.Render("Scanning...")
			}
			return ""
		}(),
	))
}

func (m *Model) renderDeviceTable(b *strings.Builder, width int) {
	theme := m.theme

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

	headers := [4]string{"Name", "Type", "Link", "Status"}

	// Add horizontal margin: 2 chars on each side.
	const tableMargin = 2
	tableWidth := width - tableMargin*2

	const borderOverhead = 5
	const cellPadding = 2

	available := tableWidth - borderOverhead
	if available < 40 {
		available = 40
	}

	// Proportional column widths: ~40/20/20/20.
	nameW := available * 40 / 100
	typeW := available * 20 / 100
	transportW := available * 20 / 100
	statusW := available - nameW - typeW - transportW

	if nameW < 10+cellPadding {
		nameW = 10 + cellPadding
	}

	nameContentW := nameW - cellPadding

	// Build rows with truncation / marquee for the Name column.
	cursor := m.cursor
	var rows [][]string
	for i, r := range rowInfos {
		name := r.name
		if len(name) > nameContentW {
			if m.focused && i == cursor {
				name = components.ScrollText(name, nameContentW, m.scrollOffset)
			} else {
				name = components.TruncateText(name, nameContentW)
			}
		}
		rows = append(rows, []string{name, r.typ, r.transport, r.status})
	}

	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(theme.Border).
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
				return theme.TableHeader.Width(w)
			}

			if m.focused && row == cursor {
				return theme.Selected.Width(w)
			}

			base := theme.TableCell.Width(w)
			if len(m.devices) == 0 {
				return base.Foreground(theme.Inactive)
			}

			if col == 3 && row < len(m.devices) {
				dev := m.devices[row]
				if dev.Connected {
					return base.Foreground(theme.Active)
				} else if dev.Paired {
					return base.Foreground(theme.Warning)
				}
				return base.Foreground(theme.Inactive)
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
