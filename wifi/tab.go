package wifi

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/olivierpoupier/patch/tui"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
)

// Model is the WiFi tab's Bubbletea model.
type Model struct {
	client         *WLANClient
	iface          InterfaceInfo
	connection     ConnectionInfo
	networks       []NetworkInfo
	cursor         int
	scanning       bool
	err            error
	width          int
	height         int
	active         bool
	focused        bool
	initialized    bool
	locationAuthed bool
	viewport       viewport.Model
}

// New creates a new WiFi tab model.
func New() *Model {
	return &Model{}
}

// Name returns the tab display name.
func (m *Model) Name() string { return "WiFi" }

// Init returns the initial command to set up the CoreWLAN client.
func (m *Model) Init() tea.Cmd {
	return initClient()
}

// Update handles messages and returns the updated tab.
func (m *Model) Update(msg tea.Msg) (tui.Tab, tea.Cmd) {
	switch msg := msg.(type) {
	case initMsg:
		m.client = msg.client
		m.initialized = true
		m.locationAuthed = msg.client.LocationAuthorized()
		cmds := []tea.Cmd{
			fetchInterfaceInfo(m.client),
			fetchConnection(m.client),
		}
		if m.active {
			cmds = append(cmds, scanNetworksCmd(m.client), scheduleConnectionTick(), scheduleScanTick())
		}
		return m, tea.Batch(cmds...)

	case interfaceInfoMsg:
		m.iface = InterfaceInfo(msg)
		m.err = nil
		return m, nil

	case connectionMsg:
		m.connection = ConnectionInfo(msg)
		m.err = nil
		return m, nil

	case networksMsg:
		m.scanning = false
		networks := []NetworkInfo(msg)
		// Sort: connected SSID first, then by RSSI descending.
		connSSID := m.connection.SSID
		sort.Slice(networks, func(i, j int) bool {
			iConn := networks[i].SSID == connSSID && connSSID != ""
			jConn := networks[j].SSID == connSSID && connSSID != ""
			if iConn != jConn {
				return iConn
			}
			return networks[i].RSSI > networks[j].RSSI
		})
		m.networks = networks
		m.err = nil
		if m.cursor >= len(m.networks) && len(m.networks) > 0 {
			m.cursor = len(m.networks) - 1
		}
		return m, nil

	case connectionTickMsg:
		if !m.active || m.client == nil {
			return m, nil
		}
		return m, tea.Batch(
			fetchInterfaceInfo(m.client),
			fetchConnection(m.client),
			scheduleConnectionTick(),
		)

	case scanTickMsg:
		if !m.active || m.client == nil {
			return m, nil
		}
		m.scanning = true
		return m, tea.Batch(
			scanNetworksCmd(m.client),
			scheduleScanTick(),
		)

	case errorMsg:
		m.err = msg.err
		m.scanning = false
		slog.Warn("wifi error displayed", "error", msg.err)
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
			if m.cursor < len(m.networks)-1 {
				m.cursor++
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
	if !m.initialized || m.client == nil {
		return m, nil
	}

	if active {
		slog.Info("wifi tab activated")
		return m, tea.Batch(
			fetchInterfaceInfo(m.client),
			fetchConnection(m.client),
			scanNetworksCmd(m.client),
			scheduleConnectionTick(),
			scheduleScanTick(),
		)
	}

	slog.Info("wifi tab deactivated")
	return m, nil
}

// View renders the tab content with a fixed header, scrollable body, and fixed footer.
func (m *Model) View(width, height int) string {
	if !m.initialized {
		return "  Initializing WiFi..."
	}

	if m.err != nil && m.client == nil {
		return fmt.Sprintf("  WiFi error: %v", m.err)
	}

	// Render header (interface info).
	var header strings.Builder
	m.renderInterfaceHeader(&header, width)
	headerStr := header.String()

	// Render footer (error + help).
	var footer strings.Builder
	if m.err != nil {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4444"))
		footer.WriteString(errStyle.Render(fmt.Sprintf("  Error: %v", m.err)))
		footer.WriteString("\n")
	}
	helpStyle := lipgloss.NewStyle().Foreground(tui.InactiveColor)
	footer.WriteString(helpStyle.Render("  ↑↓/jk navigate  esc back"))
	footerStr := footer.String()

	// Calculate body height for the viewport.
	headerHeight := lipgloss.Height(headerStr)
	footerHeight := lipgloss.Height(footerStr)
	bodyHeight := height - headerHeight - footerHeight - 1
	if bodyHeight < 3 {
		bodyHeight = 3
	}

	// Render body content.
	tableWidth := width - 1 // reserve 1 char for scrollbar track
	var body strings.Builder

	if !m.iface.PowerOn {
		dimStyle := lipgloss.NewStyle().Foreground(tui.InactiveColor)
		body.WriteString("\n")
		body.WriteString(dimStyle.Render("  WiFi is turned off"))
		body.WriteString("\n")
	} else {
		if !m.locationAuthed {
			warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFAA00"))
			body.WriteString(warnStyle.Render("  ⚠ Location Services permission required for SSID visibility."))
			body.WriteString("\n")
			body.WriteString(warnStyle.Render("  Grant access in System Settings → Privacy & Security → Location Services."))
			body.WriteString("\n\n")
		}
		m.renderConnectionBlock(&body, tableWidth)
		body.WriteString("\n")
		m.renderNetworkTable(&body, tableWidth)
	}

	bodyStr := body.String()
	contentLines := lipgloss.Height(bodyStr)
	// Pad below so the last row can scroll up.
	for i := 0; i < bodyHeight/2; i++ {
		bodyStr += "\n"
	}

	// Configure viewport for scrollable body.
	m.viewport.SetWidth(tableWidth)
	m.viewport.SetHeight(bodyHeight)
	m.viewport.SetContent(bodyStr)

	// Auto-scroll to keep the cursor visible.
	if len(m.networks) > 0 {
		// Account for connection block lines + table header + border.
		connBlockLines := 0
		if m.connection.Connected {
			connBlockLines = 8 // connection box + blank line
		} else {
			connBlockLines = 3 // "Not connected" + blank lines
		}
		rowLine := connBlockLines + m.cursor + 2 // +2 for table top border + header row
		vpHeight := m.viewport.Height()
		margin := vpHeight / 5
		if margin < 1 {
			margin = 1
		}
		yOffset := m.viewport.YOffset()
		if rowLine < yOffset+margin {
			m.viewport.SetYOffset(rowLine - margin)
		} else if rowLine >= yOffset+vpHeight-margin {
			m.viewport.SetYOffset(rowLine - vpHeight + margin + 1)
		}
		if m.viewport.YOffset() < 0 {
			m.viewport.SetYOffset(0)
		}
	}

	vpView := m.viewport.View()
	vpView = m.overlayScrollbar(vpView, bodyHeight, contentLines)

	return headerStr + "\n" + vpView + "\n" + footerStr
}

func (m *Model) renderInterfaceHeader(b *strings.Builder, width int) {
	labelStyle := lipgloss.NewStyle().Bold(true).Foreground(tui.ActiveColor)
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	dimStyle := lipgloss.NewStyle().Foreground(tui.InactiveColor)

	name := m.iface.Name
	if name == "" {
		name = "Unknown"
	}

	powerIndicator := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4444")).Render("● Off")
	if m.iface.PowerOn {
		powerIndicator = lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Render("● On")
	}

	b.WriteString(fmt.Sprintf("  %s %s    %s %s    %s %s",
		labelStyle.Render("Interface:"),
		valueStyle.Render(name),
		labelStyle.Render("Power:"),
		powerIndicator,
		labelStyle.Render("MAC:"),
		valueStyle.Render(m.iface.HardwareAddr),
	))

	if m.scanning {
		b.WriteString("    ")
		b.WriteString(dimStyle.Render("Scanning..."))
	}
	b.WriteString("\n")
}

func (m *Model) renderConnectionBlock(b *strings.Builder, width int) {
	if !m.connection.Connected {
		dimStyle := lipgloss.NewStyle().Foreground(tui.InactiveColor)
		b.WriteString(dimStyle.Render("  Not connected"))
		b.WriteString("\n")
		return
	}

	labelStyle := lipgloss.NewStyle().Bold(true).Foreground(tui.ActiveColor)
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))

	// Build connection info lines.
	var lines []string
	lines = append(lines, fmt.Sprintf("  %s %s              %s %s",
		labelStyle.Render("SSID:"),
		valueStyle.Render(m.connection.SSID),
		labelStyle.Render("Signal:"),
		valueStyle.Render(signalString(m.connection.RSSI)+" dBm"),
	))
	lines = append(lines, fmt.Sprintf("  %s %d (%s)       %s %d dBm",
		labelStyle.Render("Channel:"),
		m.connection.Channel,
		m.connection.Band,
		labelStyle.Render("Noise:"),
		m.connection.Noise,
	))
	lines = append(lines, fmt.Sprintf("  %s %s      %s %.0f Mbps",
		labelStyle.Render("Security:"),
		valueStyle.Render(m.connection.Security.String()),
		labelStyle.Render("TX Rate:"),
		m.connection.TXRate,
	))
	lines = append(lines, fmt.Sprintf("  %s %s     %s %s",
		labelStyle.Render("BSSID:"),
		valueStyle.Render(m.connection.BSSID),
		labelStyle.Render("PHY:"),
		valueStyle.Render(m.connection.PHYMode.String()),
	))

	content := strings.Join(lines, "\n")

	// Wrap in a rounded border box.
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(tui.ActiveColor).
		Padding(0, 1)

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(tui.ActiveColor)
	b.WriteString("  " + titleStyle.Render("Current Connection"))
	b.WriteString("\n")
	boxed := borderStyle.Render(content)
	for _, line := range strings.Split(boxed, "\n") {
		b.WriteString("  ")
		b.WriteString(line)
		b.WriteString("\n")
	}
}

func (m *Model) renderNetworkTable(b *strings.Builder, width int) {
	type rowData struct {
		ssid, signal, security, channel string
	}
	var rowInfos []rowData

	if len(m.networks) == 0 {
		rowInfos = []rowData{{"No networks found", "", "", ""}}
	} else {
		for _, net := range m.networks {
			ssid := net.SSID
			if ssid == "" {
				ssid = "(Hidden)"
			}

			signal := signalString(net.RSSI)

			ch := fmt.Sprintf("%d", net.Channel)
			if net.Band != "" {
				// Shorten band label for table.
				bandShort := net.Band
				switch net.Band {
				case "2.4 GHz":
					bandShort = "2G"
				case "5 GHz":
					bandShort = "5G"
				case "6 GHz":
					bandShort = "6G"
				}
				ch = fmt.Sprintf("%d %s", net.Channel, bandShort)
			}

			rowInfos = append(rowInfos, rowData{ssid, signal, net.Security.String(), ch})
		}
	}

	// Measure max content width per column.
	headers := [4]string{"SSID", "Signal", "Security", "Channel"}
	maxContent := [4]int{}
	for i, h := range headers {
		maxContent[i] = len(h)
	}
	for _, r := range rowInfos {
		cols := [4]string{r.ssid, r.signal, r.security, r.channel}
		for i, c := range cols {
			if w := len(c); w > maxContent[i] {
				maxContent[i] = w
			}
		}
	}

	const tableMargin = 2
	tableWidth := width - tableMargin*2

	const borderOverhead = 5
	const cellPadding = 2

	available := tableWidth - borderOverhead
	if available < 40 {
		available = 40
	}

	// Proportional widths: SSID ~40%, Signal ~20%, Security ~20%, Channel ~20%.
	ssidW := available * 40 / 100
	signalW := available * 20 / 100
	securityW := available * 20 / 100
	channelW := available - ssidW - signalW - securityW

	if ssidW < 10+cellPadding {
		ssidW = 10 + cellPadding
	}

	ssidContentW := ssidW - cellPadding

	var rows [][]string
	for _, r := range rowInfos {
		ssid := r.ssid
		if len(ssid) > ssidContentW {
			ssid = truncateText(ssid, ssidContentW)
		}
		rows = append(rows, []string{ssid, r.signal, r.security, r.channel})
	}

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(tui.ActiveColor).Padding(0, 1)
	selectedStyle := lipgloss.NewStyle().Background(lipgloss.Color("#04B575")).Foreground(lipgloss.Color("#000000")).Padding(0, 1)
	normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Padding(0, 1)

	cursor := m.cursor

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
				w = ssidW
			case 1:
				w = signalW
			case 2:
				w = securityW
			default:
				w = channelW
			}

			if row == table.HeaderRow {
				return headerStyle.Width(w)
			}

			if m.focused && row == cursor {
				return selectedStyle.Width(w)
			}

			base := normalStyle.Width(w)
			if len(m.networks) == 0 {
				return base.Foreground(tui.InactiveColor)
			}

			// Highlight connected network SSID in green.
			if col == 0 && row < len(m.networks) {
				if m.networks[row].SSID == m.connection.SSID && m.connection.Connected {
					return base.Foreground(lipgloss.Color("#04B575"))
				}
			}

			// Dim hidden networks.
			if col == 0 && row < len(m.networks) && m.networks[row].SSID == "" {
				return base.Foreground(tui.InactiveColor)
			}

			return base
		})

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

// overlayScrollbar renders a scrollbar track on the right edge of the viewport.
func (m *Model) overlayScrollbar(vpView string, vpHeight int, contentLines int) string {
	totalLines := contentLines
	if totalLines <= vpHeight {
		return vpView
	}

	trackStyle := lipgloss.NewStyle().Foreground(tui.InactiveColor)
	thumbStyle := lipgloss.NewStyle().Foreground(tui.ActiveColor)

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
