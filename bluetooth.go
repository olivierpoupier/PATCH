package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/bluetuith-org/bluetooth-classic/api/bluetooth"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
)

// Messages for the bluetooth tab.
type (
	btAdapterInfoMsg adapterInfo
	btDevicesMsg     []deviceInfo
	btErrorMsg       struct{ err error }
	btDeviceAddedMsg struct{ data bluetooth.DeviceData }
	btDeviceUpdatedMsg struct{ data bluetooth.DeviceEventData }
	btDeviceRemovedMsg struct{ data bluetooth.DeviceEventData }
	btAdapterUpdatedMsg struct{ data bluetooth.AdapterEventData }
	btScanTickMsg      struct{}
)

type bluetoothModel struct {
	adapter     *btAdapter
	adapterInfo adapterInfo
	devices     []deviceInfo
	cursor      int
	scanning    bool
	err         error
	width       int
	height      int
	focused     bool
	initialized bool

	deviceSub  *bluetooth.Subscriber[bluetooth.DeviceData, bluetooth.DeviceEventData]
	adapterSub *bluetooth.Subscriber[bluetooth.AdapterData, bluetooth.AdapterEventData]
}

func newBluetoothModel() bluetoothModel {
	return bluetoothModel{}
}

func (m bluetoothModel) Init() tea.Cmd {
	return initBTAdapter()
}

// initBTAdapter creates the adapter and fetches initial state.
func initBTAdapter() tea.Cmd {
	return func() tea.Msg {
		adapter, err := newBTAdapter()
		if err != nil {
			return btErrorMsg{err}
		}
		return btInitMsg{adapter: adapter}
	}
}

type btInitMsg struct {
	adapter *btAdapter
}

func (m bluetoothModel) Update(msg tea.Msg) (bluetoothModel, tea.Cmd) {
	switch msg := msg.(type) {
	case btInitMsg:
		m.adapter = msg.adapter
		m.initialized = true
		return m, tea.Batch(
			fetchAdapterInfo(m.adapter),
			fetchDevices(m.adapter),
			listenDeviceEvents(),
			listenAdapterEvents(),
		)

	case btAdapterInfoMsg:
		m.adapterInfo = adapterInfo(msg)
		m.err = nil
		return m, nil

	case btDevicesMsg:
		m.devices = []deviceInfo(msg)
		m.err = nil
		if m.cursor >= len(m.devices) && len(m.devices) > 0 {
			m.cursor = len(m.devices) - 1
		}
		return m, nil

	case btDeviceAddedMsg:
		return m, tea.Batch(fetchDevices(m.adapter), listenDeviceEvents())

	case btDeviceUpdatedMsg:
		return m, tea.Batch(fetchDevices(m.adapter), listenDeviceEvents())

	case btDeviceRemovedMsg:
		return m, tea.Batch(fetchDevices(m.adapter), listenDeviceEvents())

	case btAdapterUpdatedMsg:
		return m, tea.Batch(fetchAdapterInfo(m.adapter), listenAdapterEvents())

	case btScanTickMsg:
		if !m.focused || m.adapter == nil {
			return m, nil
		}
		return m, tea.Batch(
			fetchDevices(m.adapter),
			scheduleScanTick(),
		)

	case btErrorMsg:
		m.err = msg.err
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

func (m bluetoothModel) SetFocused(focused bool) (bluetoothModel, tea.Cmd) {
	m.focused = focused
	if !m.initialized || m.adapter == nil {
		return m, nil
	}

	if focused {
		return m, tea.Batch(
			startDiscovery(m.adapter),
			fetchDevices(m.adapter),
			fetchAdapterInfo(m.adapter),
			scheduleScanTick(),
		)
	}
	return m, stopDiscovery(m.adapter)
}

func (m bluetoothModel) View(width int) string {
	if !m.initialized {
		return "  Initializing Bluetooth..."
	}

	if m.err != nil && m.adapter == nil {
		return fmt.Sprintf("  Bluetooth error: %v", m.err)
	}

	var b strings.Builder

	// Adapter info section
	m.renderAdapterInfo(&b, width)
	b.WriteString("\n")

	// Device table
	m.renderDeviceTable(&b, width)

	// Help line
	b.WriteString("\n")
	helpStyle := lipgloss.NewStyle().Foreground(inactiveColor)
	b.WriteString(helpStyle.Render("  ↑↓/jk navigate  enter connect/disconnect  p toggle power  esc back"))

	return b.String()
}

func (m bluetoothModel) renderAdapterInfo(b *strings.Builder, width int) {
	labelStyle := lipgloss.NewStyle().Bold(true).Foreground(activeColor)
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	dimStyle := lipgloss.NewStyle().Foreground(inactiveColor)

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

func (m bluetoothModel) renderDeviceTable(b *strings.Builder, width int) {
	tableWidth := width - 6
	if tableWidth < 40 {
		tableWidth = 40
	}
	nameW := tableWidth * 50 / 100
	typeW := tableWidth * 25 / 100
	statusW := tableWidth - nameW - typeW

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(activeColor).Padding(0, 1)
	selectedStyle := lipgloss.NewStyle().Background(lipgloss.Color("#04B575")).Foreground(lipgloss.Color("#000000")).Padding(0, 1)
	normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Padding(0, 1)

	cursor := m.cursor

	var rows [][]string
	if len(m.devices) == 0 {
		rows = [][]string{{"No devices found", "", ""}}
	} else {
		for _, dev := range m.devices {
			name := dev.Name
			typeName := dev.Type

			var status string
			if dev.Connected {
				status = "Connected"
			} else if dev.Paired {
				status = "Paired"
			} else {
				status = "Available"
			}

			rows = append(rows, []string{name, typeName, status})
		}
	}

	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(activeColor)).
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
				return base.Foreground(inactiveColor)
			}

			// Color the status column based on connection state
			if col == 2 && row < len(m.devices) {
				dev := m.devices[row]
				if dev.Connected {
					return base.Foreground(lipgloss.Color("#04B575"))
				} else if dev.Paired {
					return base.Foreground(lipgloss.Color("#FFAA00"))
				}
				return base.Foreground(inactiveColor)
			}

			return base
		})

	b.WriteString(t.String())
}

// Commands

func fetchAdapterInfo(adapter *btAdapter) tea.Cmd {
	return func() tea.Msg {
		info, err := adapter.info()
		if err != nil {
			return btErrorMsg{err}
		}
		return btAdapterInfoMsg(info)
	}
}

func fetchDevices(adapter *btAdapter) tea.Cmd {
	return func() tea.Msg {
		devs, err := adapter.devices()
		if err != nil {
			return btErrorMsg{err}
		}
		return btDevicesMsg(devs)
	}
}

func startDiscovery(adapter *btAdapter) tea.Cmd {
	return func() tea.Msg {
		if err := adapter.startDiscovery(); err != nil {
			return btErrorMsg{err}
		}
		info, err := adapter.info()
		if err != nil {
			return btErrorMsg{err}
		}
		return btAdapterInfoMsg(info)
	}
}

func stopDiscovery(adapter *btAdapter) tea.Cmd {
	return func() tea.Msg {
		adapter.stopDiscovery()
		return nil
	}
}

func connectDevice(adapter *btAdapter, addr bluetooth.MacAddress) tea.Cmd {
	return func() tea.Msg {
		if err := adapter.connectDevice(addr); err != nil {
			return btErrorMsg{err}
		}
		devs, err := adapter.devices()
		if err != nil {
			return btErrorMsg{err}
		}
		return btDevicesMsg(devs)
	}
}

func disconnectDevice(adapter *btAdapter, addr bluetooth.MacAddress) tea.Cmd {
	return func() tea.Msg {
		if err := adapter.disconnectDevice(addr); err != nil {
			return btErrorMsg{err}
		}
		devs, err := adapter.devices()
		if err != nil {
			return btErrorMsg{err}
		}
		return btDevicesMsg(devs)
	}
}

func togglePower(adapter *btAdapter) tea.Cmd {
	return func() tea.Msg {
		if err := adapter.togglePower(); err != nil {
			return btErrorMsg{err}
		}
		info, err := adapter.info()
		if err != nil {
			return btErrorMsg{err}
		}
		return btAdapterInfoMsg(info)
	}
}

func listenDeviceEvents() tea.Cmd {
	return func() tea.Msg {
		sub, ok := bluetooth.DeviceEvents().Subscribe()
		if !ok {
			return nil
		}

		// Return first event, then re-subscribe for the next one.
		// This is the bubbletea pattern: one Cmd yields one Msg.
		select {
		case data := <-sub.AddedEvents:
			sub.Unsubscribe()
			return btDeviceAddedMsg{data}
		case data := <-sub.UpdatedEvents:
			sub.Unsubscribe()
			return btDeviceUpdatedMsg{data}
		case data := <-sub.RemovedEvents:
			sub.Unsubscribe()
			return btDeviceRemovedMsg{data}
		case <-sub.Done:
			return nil
		}
	}
}

func listenAdapterEvents() tea.Cmd {
	return func() tea.Msg {
		sub, ok := bluetooth.AdapterEvents().Subscribe()
		if !ok {
			return nil
		}

		select {
		case data := <-sub.UpdatedEvents:
			sub.Unsubscribe()
			return btAdapterUpdatedMsg{data}
		case <-sub.Done:
			return nil
		}
	}
}

func scheduleScanTick() tea.Cmd {
	return tea.Tick(3*time.Second, func(time.Time) tea.Msg {
		return btScanTickMsg{}
	})
}

