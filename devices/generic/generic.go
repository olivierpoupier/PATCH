package generic

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/olivierpoupier/patch/serialterm"
	"github.com/olivierpoupier/patch/tui"
	"github.com/olivierpoupier/patch/tui/components"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Model is the fallback device view used for any USB serial device that does
// not have a dedicated package. It is a plain scrolling terminal: no
// on-connect queries, no header parsing.
type Model struct {
	theme  *tui.Theme
	device tui.SerialDeviceInfo

	session    *serialterm.Session
	scrollback *serialterm.Scrollback
	scroll     components.ScrollableView

	err          error
	disconnected bool

	width, height int
}

// New constructs an empty generic view.
func New(theme *tui.Theme) *Model {
	return &Model{
		theme:      theme,
		session:    &serialterm.Session{},
		scrollback: serialterm.NewScrollback(0),
		scroll:     components.NewScrollableView(theme),
	}
}

// Name returns the display name for this view.
func (m *Model) Name() string { return "Serial Terminal" }

// Open activates the view for the given device and opens the serial port.
func (m *Model) Open(info tui.SerialDeviceInfo) tea.Cmd {
	m.device = info
	if m.device.Baud == 0 {
		m.device.Baud = 115200
	}
	if m.device.Name == "" {
		m.device.Name = "USB Serial Device"
	}
	m.scrollback.Clear()
	m.err = nil
	m.disconnected = false
	return serialterm.OpenSerialSession(m.device.PortPath, m.device.Baud)
}

// Close stops the I/O session.
func (m *Model) Close() {
	m.session.Close()
	m.err = nil
	m.disconnected = false
}

// SetSize stores the full-screen dimensions.
func (m *Model) SetSize(w, h int) { m.width = w; m.height = h }

// Update handles all messages while the modal is visible.
func (m *Model) Update(msg tea.Msg) (tui.DeviceView, tea.Cmd) {
	switch msg := msg.(type) {
	case serialterm.OpenedMsg:
		m.session.Install(msg)
		m.err = nil
		return m, m.session.WaitRx()
	case serialterm.OpenErrMsg:
		m.err = msg.Err
		return m, nil
	case serialterm.RxMsg:
		m.scrollback.Write(msg.Data)
		return m, m.session.WaitRx()
	case serialterm.RxClosedMsg:
		m.disconnected = true
		if msg.Err != nil {
			m.err = msg.Err
		}
		m.session.MarkDisconnected()
		return m, nil
	case serialterm.TxDoneMsg:
		if msg.Err != nil {
			m.err = msg.Err
		}
		return m, nil
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return tui.CloseDeviceMsg{} }
		case "ctrl+l":
			m.scrollback.Clear()
			return m, nil
		}
		if !m.session.Active() || m.disconnected {
			return m, nil
		}
		data := serialterm.KeyToBytes(msg)
		if len(data) == 0 {
			return m, nil
		}
		return m, m.session.Send(data)
	}
	return m, nil
}

// View renders the full-screen modal.
func (m *Model) View(width, height int) string {
	m.width = width
	m.height = height
	if width <= 0 || height <= 0 {
		return ""
	}

	t := m.theme.Terminal

	header := m.renderHeader(width)
	footer := m.renderFooter()

	headerH := lipgloss.Height(header)
	footerH := lipgloss.Height(footer)
	bodyHeight := height - headerH - footerH - 1
	if bodyHeight < 3 {
		bodyHeight = 3
	}
	bodyWidth := width - 1

	body := m.renderBody()

	m.scroll = m.scroll.SetSize(bodyWidth, bodyHeight)
	m.scroll = m.scroll.SetContent(body)

	lines := strings.Count(body, "\n")
	if lines > bodyHeight {
		m.scroll = m.scroll.ScrollToLine(lines)
	}

	screen := t.Body.Render(m.scroll.View())
	return header + "\n" + screen + "\n" + footer
}

func (m *Model) renderHeader(width int) string {
	t := m.theme.Terminal
	sep := t.Separator.Render(strings.Repeat("═", width))

	bannerLine := t.Banner.Render(banner)

	name := m.device.Name
	if name == "" {
		name = "USB Serial Device"
	}
	vid := strings.TrimPrefix(strings.ToLower(m.device.VID), "0x")
	pid := strings.TrimPrefix(strings.ToLower(m.device.PID), "0x")
	vidPid := vid
	if pid != "" {
		vidPid = vid + ":" + pid
	}

	rows := []string{
		fieldLine(t,
			kv{"Device", name},
			kv{"Port", m.device.PortPath},
			kv{"Baud", fmt.Sprintf("%d", m.device.Baud)},
		),
		fieldLine(t,
			kv{"VID:PID", vidPid},
			kv{"Serial", orDash(m.device.SerialNumber)},
		),
	}

	if m.disconnected {
		rows = append(rows, t.DisconnectBanner.Render(" DEVICE DISCONNECTED "))
	}

	parts := []string{
		sep,
		indent(bannerLine, 2),
		sep,
		indent(strings.Join(rows, "\n"), 1),
		sep,
	}
	return strings.Join(parts, "\n")
}

func (m *Model) renderBody() string {
	t := m.theme.Terminal

	if m.err != nil && !m.session.Active() {
		return m.renderOpenError()
	}
	if !m.session.Active() {
		return t.Dim.Render("  Opening serial port…")
	}
	lines := m.scrollback.Lines()
	if len(lines) == 0 {
		return t.Dim.Render("  (waiting for output — type to send bytes)")
	}
	var b strings.Builder
	for _, line := range lines {
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

func (m *Model) renderOpenError() string {
	t := m.theme.Terminal

	var b strings.Builder
	b.WriteString(t.DisconnectBanner.Render(" FAILED TO OPEN PORT "))
	b.WriteString("\n\n")
	b.WriteString(t.Body.Render("  " + m.err.Error()))
	b.WriteString("\n\n")

	if serialterm.IsPermissionDenied(m.err.Error()) && runtime.GOOS == "linux" {
		b.WriteString(t.HeaderKey.Render("  This user cannot access the serial port."))
		b.WriteString("\n")
		b.WriteString(t.HeaderVal.Render("  Add yourself to the dialout group:"))
		b.WriteString("\n")
		b.WriteString(t.Body.Render("    sudo usermod -aG dialout $USER && newgrp dialout"))
		b.WriteString("\n")
		b.WriteString(t.Dim.Render("  (on Arch, the group is 'uucp')"))
		b.WriteString("\n")
	}
	return b.String()
}

func (m *Model) renderFooter() string {
	bindings := []components.KeyBinding{
		{Keys: "esc", Description: "close"},
		{Keys: "ctrl+l", Description: "clear"},
		{Keys: "keys", Description: "→ device"},
	}
	var parts []string
	for _, b := range bindings {
		parts = append(parts, b.Keys+" "+b.Description)
	}
	return m.theme.Terminal.Help.Render("  " + strings.Join(parts, "  "))
}

// --- small helpers (duplicated from flipper; extract to tui/components if a
// third device adds the same helpers) ---

type kv struct {
	label, value string
}

func fieldLine(t *tui.TerminalTheme, parts ...kv) string {
	var segs []string
	for _, p := range parts {
		segs = append(segs, t.HeaderKey.Render(p.label+":")+" "+t.HeaderVal.Render(p.value))
	}
	return strings.Join(segs, "  ")
}

func indent(s string, n int) string {
	if n <= 0 {
		return s
	}
	pad := strings.Repeat(" ", n)
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		lines[i] = pad + ln
	}
	return strings.Join(lines, "\n")
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
