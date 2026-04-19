package serialterm

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/olivierpoupier/patch/tui"
	"github.com/olivierpoupier/patch/tui/components"

	"charm.land/lipgloss/v2"
)

// View renders the full-screen serial terminal modal.
func (m *Model) View(width, height int) string {
	m.width = width
	m.height = height
	if width <= 0 || height <= 0 {
		return ""
	}

	t := m.theme.Terminal

	header := m.renderHeader(width)
	footer := m.renderFooter(width)

	headerH := lipgloss.Height(header)
	footerH := lipgloss.Height(footer)
	bodyHeight := height - headerH - footerH - 1
	if bodyHeight < 3 {
		bodyHeight = 3
	}
	bodyWidth := width - 1 // reserve scrollbar column

	body := m.renderBody(bodyWidth, bodyHeight)

	m.scroll = m.scroll.SetSize(bodyWidth, bodyHeight)
	m.scroll = m.scroll.SetContent(body)

	// Pin scroll to the bottom so latest output is always visible.
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

	banner := t.Banner.Render(bannerFor(m.profile.Key))

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
			field("Device", name),
			field("Port", m.device.PortPath),
			field("Baud", fmt.Sprintf("%d", m.device.Baud)),
		),
		fieldLine(t,
			field("VID:PID", vidPid),
			field("Serial", orDash(m.device.SerialNumber)),
		),
	}

	if len(m.profile.HeaderFields) > 0 && len(m.headerFields) > 0 {
		var fs []kv
		for _, key := range m.profile.HeaderFields {
			val, ok := m.headerFields[key]
			if !ok || val == "" {
				continue
			}
			if key == "charge.level" {
				val += "%"
			}
			fs = append(fs, field(prettyLabel(key), val))
		}
		if len(fs) > 0 {
			rows = append(rows, fieldLine(t, fs...))
		}
	}

	if m.disconnected {
		rows = append(rows, t.DisconnectBanner.Render(" DEVICE DISCONNECTED "))
	}

	parts := []string{
		sep,
		indent(banner, 2),
		sep,
		indent(strings.Join(rows, "\n"), 1),
		sep,
	}
	return strings.Join(parts, "\n")
}

func (m *Model) renderBody(width, _ int) string {
	t := m.theme.Terminal

	if m.err != nil && m.port == nil {
		return m.renderOpenError(width)
	}

	if m.port == nil {
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

func (m *Model) renderOpenError(width int) string {
	t := m.theme.Terminal

	var b strings.Builder
	b.WriteString(t.DisconnectBanner.Render(" FAILED TO OPEN PORT "))
	b.WriteString("\n\n")
	b.WriteString(t.Body.Render("  " + m.err.Error()))
	b.WriteString("\n\n")

	if isPermissionDenied(m.err.Error()) && runtime.GOOS == "linux" {
		b.WriteString(t.HeaderKey.Render("  This user cannot access the serial port."))
		b.WriteString("\n")
		b.WriteString(t.HeaderVal.Render("  Add yourself to the dialout group:"))
		b.WriteString("\n")
		b.WriteString(t.Body.Render("    sudo usermod -aG dialout $USER && newgrp dialout"))
		b.WriteString("\n")
		b.WriteString(t.Dim.Render("  (on Arch, the group is 'uucp')"))
		b.WriteString("\n")
	}
	_ = width
	return b.String()
}

func (m *Model) renderFooter(width int) string {
	bindings := []components.KeyBinding{
		{Keys: "esc", Description: "close"},
		{Keys: "ctrl+l", Description: "clear"},
	}
	if len(m.profile.HeaderQueries) > 0 && m.profile.HeaderParser != nil {
		bindings = append(bindings, components.KeyBinding{Keys: "ctrl+r", Description: "refresh info"})
	}
	bindings = append(bindings, components.KeyBinding{Keys: "keys", Description: "→ device"})

	var parts []string
	for _, b := range bindings {
		parts = append(parts, b.Keys+" "+b.Description)
	}
	_ = width
	return m.theme.Terminal.Help.Render("  " + strings.Join(parts, "  "))
}

// --- small helpers ---

type kv struct {
	label, value string
}

func field(label, value string) kv { return kv{label, value} }

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

func prettyLabel(key string) string {
	// Convert "firmware_version" → "Firmware", "charge_level" → "Charge"
	switch key {
	case "firmware_version":
		return "Firmware"
	case "hardware_ver":
		return "Hardware"
	case "radio_stack":
		return "Radio"
	case "charge_level", "charge.level":
		return "Battery"
	}
	parts := strings.Split(key, "_")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}

func isPermissionDenied(msg string) bool {
	m := strings.ToLower(msg)
	return strings.Contains(m, "permission denied") || strings.Contains(m, "access denied") || strings.Contains(m, "eacces")
}
