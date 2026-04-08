package bluetooth

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/olivierpoupier/patch/tui"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
)

// Model is the Bluetooth tab's Bubbletea model.
type Model struct {
	adapter     *Adapter
	adapterInfo AdapterInfo
	devices     []DeviceInfo
	cursor      int
	scanning    bool
	err         error
	width       int
	height      int
	active      bool
	focused     bool
	initialized bool
	done        chan struct{}
}

// New creates a new Bluetooth tab model.
func New() *Model {
	return &Model{}
}

// Name returns the tab display name.
func (m *Model) Name() string { return "Bluetooth" }

// Init returns the initial command to set up the adapter.
func (m *Model) Init() tea.Cmd {
	return initAdapter()
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
			listenDeviceEvents(m.done),
			listenAdapterEvents(m.done),
		}
		if m.active {
			cmds = append(cmds, scheduleScanTick())
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
			}
		case "down", "j":
			if m.cursor < len(m.devices)-1 {
				m.cursor++
			}
		case "enter":
			if len(m.devices) > 0 && m.cursor < len(m.devices) {
				dev := m.devices[m.cursor]
				if dev.Connected {
					return m, disconnectDevice(m.adapter, dev.Address)
				}
				return m, connectDevice(m.adapter, dev.Address)
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
		return m, tea.Batch(
			startDiscovery(m.adapter),
			fetchDevices(m.adapter),
			fetchAdapterInfo(m.adapter),
			listenDeviceEvents(m.done),
			listenAdapterEvents(m.done),
			scheduleScanTick(),
		)
	}

	slog.Info("bluetooth tab deactivated")
	// Cancel event listeners and stop discovery.
	if m.done != nil {
		close(m.done)
		m.done = nil
	}
	return m, stopDiscovery(m.adapter)
}

// View renders the tab content.
func (m *Model) View(width int) string {
	if !m.initialized {
		return "  Initializing Bluetooth..."
	}

	if m.err != nil && m.adapter == nil {
		return fmt.Sprintf("  Bluetooth error: %v", m.err)
	}

	var errMsg string
	if m.err != nil {
		errMsg = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4444")).Render(fmt.Sprintf("  Error: %v", m.err))
	}

	var b strings.Builder

	m.renderAdapterInfo(&b, width)
	b.WriteString("\n")

	m.renderDeviceTable(&b, width)

	if errMsg != "" {
		b.WriteString("\n")
		b.WriteString(errMsg)
	}

	b.WriteString("\n")
	helpStyle := lipgloss.NewStyle().Foreground(tui.InactiveColor)
	b.WriteString(helpStyle.Render("  ↑↓/jk navigate  enter connect/disconnect  p toggle power  esc back"))

	return b.String()
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
	tableWidth := width - 6
	if tableWidth < 40 {
		tableWidth = 40
	}
	nameW := tableWidth * 50 / 100
	typeW := tableWidth * 25 / 100
	statusW := tableWidth - nameW - typeW

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(tui.ActiveColor).Padding(0, 1)
	selectedStyle := lipgloss.NewStyle().Background(lipgloss.Color("#04B575")).Foreground(lipgloss.Color("#000000")).Padding(0, 1)
	normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Padding(0, 1)

	cursor := m.cursor

	var rows [][]string
	if len(m.devices) == 0 {
		rows = [][]string{{"No devices found", "", ""}}
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
			rows = append(rows, []string{dev.Name, dev.Type, status})
		}
	}

	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(tui.ActiveColor)).
		Headers("Name", "Type", "Status").
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			var w int
			switch col {
			case 0:
				w = nameW
			case 1:
				w = typeW
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

			if col == 2 && row < len(m.devices) {
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

	b.WriteString(t.String())
}
