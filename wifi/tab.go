package wifi

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/olivierpoupier/patch/tui"
	"github.com/olivierpoupier/patch/tui/components"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/textinput"
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
	theme          *tui.Theme
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
	scroll         components.ScrollableView
	connectState   connectPhase
	connectTarget  NetworkInfo
	passwordInput  textinput.Model
	connectErr     error
}

// New creates a new WiFi tab model.
func New(theme *tui.Theme) *Model {
	return &Model{
		theme:  theme,
		scroll: components.NewScrollableView(theme),
	}
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
	m.width = width
	m.height = height

	if !m.initialized {
		return m.theme.Dim.Render("  Initializing WiFi...")
	}

	if m.err != nil && m.client == nil {
		return m.theme.ErrorText.Render(fmt.Sprintf("  WiFi error: %v", m.err))
	}

	// Header (interface info).
	var header strings.Builder
	m.renderInterfaceHeader(&header, width)
	headerStr := header.String()

	// Footer (error + key bindings).
	var footerParts []string
	switch {
	case m.connectErr != nil:
		footerParts = append(footerParts, components.RenderError(m.theme, m.connectErr, width))
	case m.err != nil:
		footerParts = append(footerParts, components.RenderError(m.theme, m.err, width))
	}
	footerParts = append(footerParts, components.RenderFooter(m.theme, m.footerBindings(), width))
	footerStr := strings.Join(footerParts, "\n")

	// Body height: subtract header, footer, and the blank separator line.
	headerHeight := lipgloss.Height(headerStr)
	footerHeight := lipgloss.Height(footerStr)
	bodyHeight := height - headerHeight - footerHeight - 1
	if bodyHeight < 3 {
		bodyHeight = 3
	}

	// Body content.
	tableWidth := width - 1 // reserve 1 column for the scrollbar
	var body strings.Builder
	if !m.iface.PowerOn {
		body.WriteString("\n")
		body.WriteString(m.theme.Dim.Render("  WiFi is turned off"))
		body.WriteString("\n")
	} else {
		if !m.locationAuthed {
			warn := m.theme.Value.Foreground(m.theme.Warning)
			body.WriteString(warn.Render("  ⚠ Location Services required for SSID visibility."))
			body.WriteString("\n")
			body.WriteString(warn.Render("  Grant access when prompted, or enable patch in:"))
			body.WriteString("\n")
			body.WriteString(warn.Render("  System Settings > Privacy & Security > Location Services"))
			body.WriteString("\n")
			body.WriteString(warn.Render("  Then restart patch."))
			body.WriteString("\n\n")
		}
		m.renderConnectionBlock(&body, tableWidth)
		body.WriteString("\n")
		m.renderNetworkTable(&body, tableWidth)
	}
	bodyStr := body.String()

	// Pad below so the last row can scroll up and reveal the bottom border.
	for i := 0; i < bodyHeight/2; i++ {
		bodyStr += "\n"
	}

	// Configure scrollable body.
	m.scroll = m.scroll.SetSize(tableWidth, bodyHeight)
	m.scroll = m.scroll.SetContent(bodyStr)

	// Auto-scroll to keep the cursor visible.
	if m.iface.PowerOn && len(m.networks) > 0 {
		connBlockLines := 3 // "Not connected" + blank lines
		if m.connection.Connected {
			connBlockLines = 8 // bordered connection box + blank line
		}
		rowLine := connBlockLines + m.cursor + 2 // +2 for table top border + header row
		m.scroll = m.scroll.ScrollToLine(rowLine)
	}

	vpView := m.scroll.View()

	// Render connect modal overlay if active.
	if m.connectState != phaseIdle {
		vpView = m.renderConnectOverlay(vpView, width, bodyHeight)
	}

	return headerStr + "\n" + vpView + "\n" + footerStr
}

func (m *Model) footerBindings() []components.KeyBinding {
	switch {
	case m.connectState == phaseConfirm || m.connectState == phasePassword:
		return []components.KeyBinding{
			{Keys: "enter", Description: "confirm"},
			{Keys: "esc", Description: "cancel"},
		}
	case !m.iface.PowerOn:
		return []components.KeyBinding{
			{Keys: "p", Description: "power on"},
			{Keys: "esc", Description: "back"},
		}
	default:
		return []components.KeyBinding{
			{Keys: "↑↓/jk", Description: "navigate"},
			{Keys: "enter", Description: "connect/disconnect"},
			{Keys: "p", Description: "toggle power"},
			{Keys: "esc", Description: "back"},
		}
	}
}

func (m *Model) renderConnectOverlay(base string, width, height int) string {
	var content string
	ssid := m.connectTarget.SSID
	if ssid == "" {
		ssid = "(Hidden)"
	}

	title := m.theme.Label
	value := m.theme.Value
	dim := m.theme.Dim

	switch m.connectState {
	case phaseConfirm:
		content = title.Render("Connect to "+ssid+"?") + "\n\n" +
			dim.Render("enter confirm  esc cancel")
	case phasePassword:
		content = title.Render("Connect to "+ssid) + "\n\n" +
			value.Render("Password: ") + m.passwordInput.View() + "\n\n" +
			dim.Render("enter connect  esc cancel")
	case phaseConnecting:
		content = title.Render("Connecting to " + ssid + "...")
	case phaseDisconnecting:
		content = title.Render("Disconnecting...")
	default:
		return base
	}

	boxStyle := m.theme.Border.
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Active).
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
	theme := m.theme

	name := m.iface.Name
	if name == "" {
		name = "Unknown"
	}

	powerColor := theme.Error
	powerText := "● Off"
	if m.iface.PowerOn {
		powerColor = theme.Active
		powerText = "● On"
	}
	powerIndicator := theme.Value.Foreground(powerColor).Render(powerText)

	b.WriteString(fmt.Sprintf("  %s %s    %s %s    %s %s",
		theme.Label.Render("Interface:"),
		theme.Value.Render(name),
		theme.Label.Render("Power:"),
		powerIndicator,
		theme.Label.Render("MAC:"),
		theme.Value.Render(m.iface.HardwareAddr),
	))

	if m.scanning {
		b.WriteString("    ")
		b.WriteString(theme.Dim.Render("Scanning..."))
	}
	b.WriteString("\n")
	_ = width
}

func (m *Model) renderConnectionBlock(b *strings.Builder, width int) {
	theme := m.theme

	if !m.connection.Connected {
		b.WriteString(theme.Dim.Render("  Not connected"))
		b.WriteString("\n")
		return
	}

	// Build connection info lines.
	var lines []string
	lines = append(lines, fmt.Sprintf("  %s %s              %s %s",
		theme.Label.Render("SSID:"),
		theme.Value.Render(m.connection.SSID),
		theme.Label.Render("Signal:"),
		theme.Value.Render(signalString(m.connection.RSSI)+" dBm"),
	))
	lines = append(lines, fmt.Sprintf("  %s %d (%s)       %s %d dBm",
		theme.Label.Render("Channel:"),
		m.connection.Channel,
		m.connection.Band,
		theme.Label.Render("Noise:"),
		m.connection.Noise,
	))
	lines = append(lines, fmt.Sprintf("  %s %s      %s %.0f Mbps",
		theme.Label.Render("Security:"),
		theme.Value.Render(m.connection.Security.String()),
		theme.Label.Render("TX Rate:"),
		m.connection.TXRate,
	))
	lines = append(lines, fmt.Sprintf("  %s %s     %s %s",
		theme.Label.Render("BSSID:"),
		theme.Value.Render(m.connection.BSSID),
		theme.Label.Render("PHY:"),
		theme.Value.Render(m.connection.PHYMode.String()),
	))

	content := strings.Join(lines, "\n")

	borderStyle := theme.Border.
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Active).
		Padding(0, 1)

	b.WriteString("  " + theme.Label.Render("Current Connection"))
	b.WriteString("\n")
	boxed := borderStyle.Render(content)
	for _, line := range strings.Split(boxed, "\n") {
		b.WriteString("  ")
		b.WriteString(line)
		b.WriteString("\n")
	}
	_ = width
}

func (m *Model) renderNetworkTable(b *strings.Builder, width int) {
	theme := m.theme

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

	headers := [4]string{"SSID", "Signal", "Security", "Channel"}

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
		if len([]rune(ssid)) > ssidContentW {
			ssid = components.TruncateText(ssid, ssidContentW)
		}
		rows = append(rows, []string{ssid, r.signal, r.security, r.channel})
	}

	cursor := m.cursor
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
				w = ssidW
			case 1:
				w = signalW
			case 2:
				w = securityW
			default:
				w = channelW
			}

			if row == table.HeaderRow {
				return theme.TableHeader.Width(w)
			}

			if m.focused && row == cursor {
				return theme.Selected.Width(w)
			}

			base := theme.TableCell.Width(w)
			if len(m.networks) == 0 {
				return base.Foreground(theme.Inactive)
			}

			// Highlight connected network SSID.
			if col == 0 && row < len(m.networks) {
				if m.networks[row].SSID == m.connection.SSID && m.connection.Connected {
					return base.Foreground(theme.Active)
				}
				if m.networks[row].SSID == "" {
					return base.Foreground(theme.Inactive)
				}
			}

			return base
		})

	for _, line := range strings.Split(t.String(), "\n") {
		b.WriteString("  ")
		b.WriteString(line)
		b.WriteString("\n")
	}
}
