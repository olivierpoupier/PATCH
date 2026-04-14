package wifi

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/olivierpoupier/patch/tui"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
)

type connectPhase int

const (
	phaseIdle         connectPhase = iota
	phaseConfirm                   // open network confirmation
	phasePassword                  // password entry modal
	phaseConnecting                // async connect in progress
	phaseDisconnecting             // async disconnect in progress
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
	connectState   connectPhase
	connectTarget  NetworkInfo
	passwordInput  textinput.Model
	connectErr     error
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

	case powerToggleMsg:
		m.iface = InterfaceInfo(msg)
		m.err = nil
		if m.iface.PowerOn {
			return m, tea.Batch(
				fetchConnection(m.client),
				scanNetworksCmd(m.client),
				scheduleConnectionTick(),
				scheduleScanTick(),
			)
		}
		// WiFi turned off — clear connection and networks.
		m.connection = ConnectionInfo{}
		m.networks = nil
		m.cursor = 0
		return m, nil

	case connectResultMsg:
		m.connectState = phaseIdle
		if msg.err != nil {
			m.connectErr = msg.err
			slog.Warn("wifi connect failed", "error", msg.err)
			return m, nil
		}
		m.connectErr = nil
		return m, tea.Batch(
			fetchInterfaceInfo(m.client),
			fetchConnection(m.client),
			scanNetworksCmd(m.client),
		)

	case disconnectResultMsg:
		m.connectState = phaseIdle
		if msg.err != nil {
			m.connectErr = msg.err
			slog.Warn("wifi disconnect failed", "error", msg.err)
			return m, nil
		}
		m.connectErr = nil
		return m, tea.Batch(
			fetchInterfaceInfo(m.client),
			fetchConnection(m.client),
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
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *Model) handleKey(msg tea.KeyPressMsg) (tui.Tab, tea.Cmd) {
	key := msg.String()

	// When in a modal, handle modal-specific keys.
	if m.connectState != phaseIdle {
		return m.handleModalKey(msg, key)
	}

	// Normal mode keys.
	switch key {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.networks)-1 {
			m.cursor++
		}
	case "p":
		if m.client != nil {
			return m, togglePower(m.client)
		}
	case "enter":
		if m.client == nil || len(m.networks) == 0 || m.cursor >= len(m.networks) {
			return m, nil
		}
		net := m.networks[m.cursor]
		// If this is the connected network, disconnect.
		if net.SSID == m.connection.SSID && m.connection.Connected && net.SSID != "" {
			m.connectState = phaseDisconnecting
			m.connectTarget = net
			return m, disconnectCmd(m.client)
		}
		// Open network: show confirmation.
		if net.Security == SecurityNone {
			m.connectState = phaseConfirm
			m.connectTarget = net
			m.connectErr = nil
			return m, nil
		}
		// Secured network: show password modal.
		m.connectState = phasePassword
		m.connectTarget = net
		m.connectErr = nil
		ti := textinput.New()
		ti.Placeholder = "Enter password"
		ti.EchoMode = textinput.EchoPassword
		ti.Focus()
		m.passwordInput = ti
		return m, nil
	}
	return m, nil
}

func (m *Model) handleModalKey(msg tea.KeyPressMsg, key string) (tui.Tab, tea.Cmd) {
	switch m.connectState {
	case phaseConfirm:
		switch key {
		case "enter":
			m.connectState = phaseConnecting
			ssid := m.connectTarget.SSID
			return m, connectToNetworkCmd(m.client, ssid, "")
		case "esc":
			m.connectState = phaseIdle
			m.connectErr = nil
		}
		return m, nil

	case phasePassword:
		switch key {
		case "enter":
			m.connectState = phaseConnecting
			ssid := m.connectTarget.SSID
			pass := m.passwordInput.Value()
			return m, connectToNetworkCmd(m.client, ssid, pass)
		case "esc":
			m.connectState = phaseIdle
			m.connectErr = nil
			return m, nil
		}
		// Forward to textinput for typing.
		var cmd tea.Cmd
		m.passwordInput, cmd = m.passwordInput.Update(msg)
		return m, cmd

	case phaseConnecting, phaseDisconnecting:
		// No key handling while async operation is in progress.
		return m, nil
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
	if m.connectErr != nil {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4444"))
		footer.WriteString(errStyle.Render(fmt.Sprintf("  Error: %v", m.connectErr)))
		footer.WriteString("\n")
	} else if m.err != nil {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4444"))
		footer.WriteString(errStyle.Render(fmt.Sprintf("  Error: %v", m.err)))
		footer.WriteString("\n")
	}
	helpStyle := lipgloss.NewStyle().Foreground(tui.InactiveColor)
	switch {
	case m.connectState == phaseConfirm || m.connectState == phasePassword:
		footer.WriteString(helpStyle.Render("  enter confirm  esc cancel"))
	case !m.iface.PowerOn:
		footer.WriteString(helpStyle.Render("  p power on  esc back"))
	default:
		footer.WriteString(helpStyle.Render("  p power  enter connect  ↑↓/jk navigate  esc back"))
	}
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
			body.WriteString(warnStyle.Render("  ⚠ Location Services required for SSID visibility."))
			body.WriteString("\n")
			body.WriteString(warnStyle.Render("  Grant access when prompted, or enable patch in:"))
			body.WriteString("\n")
			body.WriteString(warnStyle.Render("  System Settings > Privacy & Security > Location Services"))
			body.WriteString("\n")
			body.WriteString(warnStyle.Render("  Then restart patch."))
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

	// Render connect modal overlay if active.
	if m.connectState != phaseIdle {
		vpView = m.renderConnectOverlay(vpView, width, bodyHeight)
	}

	return headerStr + "\n" + vpView + "\n" + footerStr
}

func (m *Model) renderConnectOverlay(base string, width, height int) string {
	var content string
	ssid := m.connectTarget.SSID
	if ssid == "" {
		ssid = "(Hidden)"
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(tui.ActiveColor)
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	dimStyle := lipgloss.NewStyle().Foreground(tui.InactiveColor)

	switch m.connectState {
	case phaseConfirm:
		content = titleStyle.Render("Connect to "+ssid+"?") + "\n\n" +
			dimStyle.Render("enter confirm  esc cancel")
	case phasePassword:
		content = titleStyle.Render("Connect to "+ssid) + "\n\n" +
			valueStyle.Render("Password: ") + m.passwordInput.View() + "\n\n" +
			dimStyle.Render("enter connect  esc cancel")
	case phaseConnecting:
		content = titleStyle.Render("Connecting to "+ssid+"...")
	case phaseDisconnecting:
		content = titleStyle.Render("Disconnecting...")
	default:
		return base
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(tui.ActiveColor).
		Padding(1, 3)
	box := boxStyle.Render(content)

	boxWidth := lipgloss.Width(box)
	boxHeight := lipgloss.Height(box)

	// Center the box on the base content.
	baseLines := strings.Split(base, "\n")
	for len(baseLines) < height {
		baseLines = append(baseLines, "")
	}
	if len(baseLines) > height {
		baseLines = baseLines[:height]
	}

	startRow := (height - boxHeight) / 2
	if startRow < 0 {
		startRow = 0
	}
	startCol := (width - boxWidth) / 2
	if startCol < 0 {
		startCol = 0
	}

	boxLines := strings.Split(box, "\n")
	for i, boxLine := range boxLines {
		row := startRow + i
		if row >= len(baseLines) {
			break
		}
		line := baseLines[row]
		// Pad the base line to startCol with spaces if needed.
		lineRunes := []rune(line)
		if len(lineRunes) < startCol {
			line = line + strings.Repeat(" ", startCol-len(lineRunes))
		}
		// Replace the portion of the line with the box line.
		pre := string([]rune(line)[:startCol])
		boxLineRunes := []rune(boxLine)
		afterCol := startCol + len(boxLineRunes)
		var post string
		if afterCol < len(lineRunes) {
			post = string(lineRunes[afterCol:])
		}
		baseLines[row] = pre + boxLine + post
	}

	return strings.Join(baseLines, "\n")
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
