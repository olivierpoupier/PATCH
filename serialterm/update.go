package serialterm

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

// captureDuration is the window during which RX bytes are mirrored into
// captureBuf after sending the profile's header-query commands.
const captureDuration = 900 * time.Millisecond

// Update handles messages for the serial terminal view.
func (m *Model) Update(msg tea.Msg) (*Model, tea.Cmd) {
	switch msg := msg.(type) {
	case portOpenedMsg:
		return m.handlePortOpened(msg)
	case openErrMsg:
		m.err = msg.err
		return m, nil
	case rxMsg:
		return m.handleRx(msg)
	case rxClosedMsg:
		return m.handleRxClosed(msg)
	case txDoneMsg:
		if msg.err != nil {
			m.err = msg.err
		}
		return m, nil
	case captureEndMsg:
		return m.handleCaptureEnd()
	case tea.KeyPressMsg:
		return m.handleKeyPress(msg)
	}
	return m, nil
}

func (m *Model) handlePortOpened(msg portOpenedMsg) (*Model, tea.Cmd) {
	m.port = msg.port
	m.rxCh = msg.rxCh
	m.doneCh = msg.doneCh
	m.err = nil

	cmds := []tea.Cmd{waitRxCmd(m.rxCh)}
	if startCmd := m.startHeaderCapture(); startCmd != nil {
		cmds = append(cmds, startCmd)
	}
	return m, tea.Batch(cmds...)
}

func (m *Model) handleRx(msg rxMsg) (*Model, tea.Cmd) {
	// While capturing, RX belongs to our diagnostic queries — route it to the
	// parse buffer only so the user's scrollback stays clean (and so a device
	// that rejects a query doesn't leak "unknown command" errors on open).
	if m.capturing {
		m.captureBuf = append(m.captureBuf, msg.data...)
	} else {
		m.scrollback.Write(msg.data)
	}
	return m, waitRxCmd(m.rxCh)
}

func (m *Model) handleRxClosed(msg rxClosedMsg) (*Model, tea.Cmd) {
	m.disconnected = true
	if msg.err != nil {
		m.err = msg.err
	}
	// Goroutine already exited; still close the port so the handle is freed.
	if m.port != nil {
		_ = m.port.Close()
		m.port = nil
	}
	m.rxCh = nil
	m.doneCh = nil
	return m, nil
}

func (m *Model) handleCaptureEnd() (*Model, tea.Cmd) {
	m.capturing = false
	if m.profile.HeaderParser != nil && len(m.captureBuf) > 0 {
		parsed := m.profile.HeaderParser(m.captureBuf)
		for k, v := range parsed {
			m.headerFields[k] = v
		}
	}
	m.captureBuf = m.captureBuf[:0]
	return m, nil
}

func (m *Model) handleKeyPress(k tea.KeyPressMsg) (*Model, tea.Cmd) {
	switch k.String() {
	case "esc":
		return m, requestCloseCmd()
	case "ctrl+l":
		m.scrollback.Clear()
		return m, nil
	case "ctrl+r":
		return m, m.startHeaderCapture()
	}

	if m.port == nil || m.disconnected {
		return m, nil
	}

	data := keyToBytes(k)
	if len(data) == 0 {
		return m, nil
	}
	return m, txCmd(m.port, data)
}

// startHeaderCapture writes the profile's header queries and begins a capture
// window. Returns nil if the profile has no queries.
func (m *Model) startHeaderCapture() tea.Cmd {
	if m.port == nil {
		return nil
	}
	if len(m.profile.HeaderQueries) == 0 || m.profile.HeaderParser == nil {
		return nil
	}
	var cmds []tea.Cmd
	for _, q := range m.profile.HeaderQueries {
		cmds = append(cmds, txCmd(m.port, []byte(q)))
	}
	m.capturing = true
	m.captureBuf = m.captureBuf[:0]
	cmds = append(cmds, captureWindowCmd(captureDuration))
	return tea.Batch(cmds...)
}

// keyToBytes maps a bubbletea keypress to the byte sequence to send to the
// device. Returns nil for keys that should be swallowed.
func keyToBytes(k tea.KeyPressMsg) []byte {
	switch k.String() {
	case "enter":
		return []byte{'\r'}
	case "tab":
		return []byte{'\t'}
	case "backspace":
		return []byte{0x7f}
	case "up":
		return []byte("\x1b[A")
	case "down":
		return []byte("\x1b[B")
	case "right":
		return []byte("\x1b[C")
	case "left":
		return []byte("\x1b[D")
	case "home":
		return []byte("\x1b[H")
	case "end":
		return []byte("\x1b[F")
	case "pgup":
		return []byte("\x1b[5~")
	case "pgdown":
		return []byte("\x1b[6~")
	case "delete":
		return []byte("\x1b[3~")
	case "ctrl+@", "ctrl+space":
		return []byte{0x00}
	case "ctrl+a":
		return []byte{0x01}
	case "ctrl+b":
		return []byte{0x02}
	case "ctrl+c":
		return []byte{0x03}
	case "ctrl+d":
		return []byte{0x04}
	case "ctrl+e":
		return []byte{0x05}
	case "ctrl+f":
		return []byte{0x06}
	case "ctrl+g":
		return []byte{0x07}
	case "ctrl+h":
		return []byte{0x08}
	case "ctrl+i":
		return []byte{0x09}
	case "ctrl+j":
		return []byte{0x0a}
	case "ctrl+k":
		return []byte{0x0b}
	case "ctrl+n":
		return []byte{0x0e}
	case "ctrl+o":
		return []byte{0x0f}
	case "ctrl+p":
		return []byte{0x10}
	case "ctrl+u":
		return []byte{0x15}
	case "ctrl+w":
		return []byte{0x17}
	case "ctrl+x":
		return []byte{0x18}
	case "ctrl+y":
		return []byte{0x19}
	case "ctrl+z":
		return []byte{0x1a}
	case "space":
		return []byte{' '}
	}
	if len(k.Text) > 0 {
		return []byte(k.Text)
	}
	return nil
}
