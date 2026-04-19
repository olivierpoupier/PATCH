package serialterm

import (
	"github.com/olivierpoupier/patch/tui"
	"github.com/olivierpoupier/patch/tui/components"

	tea "charm.land/bubbletea/v2"
	"go.bug.st/serial"
)

// Model holds the serial terminal modal state.
type Model struct {
	theme *tui.Theme

	device  tui.SerialDeviceInfo
	profile Profile

	port   serial.Port
	rxCh   chan tea.Msg
	doneCh chan struct{}

	scrollback   *scrollback
	scroll       components.ScrollableView
	headerFields map[string]string

	// Transient capture window used to populate headerFields from the
	// profile's on-open query responses.
	capturing     bool
	captureBuf    []byte
	pendingOpens  int // number of tick-based events we still expect

	err          error
	disconnected bool

	active bool
	width  int
	height int
}

// New creates an empty serial terminal model.
func New(theme *tui.Theme) *Model {
	return &Model{
		theme:        theme,
		scrollback:   newScrollback(),
		scroll:       components.NewScrollableView(theme),
		headerFields: make(map[string]string),
	}
}

// Open activates the modal for the given device and kicks off port opening.
func (m *Model) Open(info tui.SerialDeviceInfo) tea.Cmd {
	m.device = info
	m.profile = LookupProfile(info.VID, info.PID)
	if m.device.Baud == 0 {
		m.device.Baud = m.profile.Baud
	}
	if m.device.Baud == 0 {
		m.device.Baud = 115200
	}
	if m.device.Name == "" {
		m.device.Name = m.profile.Name
	}
	m.scrollback.Clear()
	m.headerFields = make(map[string]string)
	m.captureBuf = m.captureBuf[:0]
	m.capturing = false
	m.err = nil
	m.disconnected = false
	m.active = true
	return openSerialCmd(m.device)
}

// Close tears the modal state down. The port and RX goroutine are stopped
// here so reopening starts from a clean slate.
func (m *Model) Close() {
	m.stopIO()
	m.active = false
	m.err = nil
	m.disconnected = false
}

// Active reports whether the modal should be rendered.
func (m *Model) Active() bool { return m.active }

// SetSize stores the full-screen dimensions.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// stopIO closes the RX goroutine and the serial port.
func (m *Model) stopIO() {
	if m.doneCh != nil {
		close(m.doneCh)
		m.doneCh = nil
	}
	if m.port != nil {
		_ = m.port.Close()
		m.port = nil
	}
	m.rxCh = nil
}
