package serialterm

import (
	"github.com/olivierpoupier/patch/tui"

	tea "charm.land/bubbletea/v2"
)

// openedMsg fires once the serial port is open and the RX goroutine is live.
type openedMsg struct{}

// openErrMsg is emitted when serial.Open fails.
type openErrMsg struct {
	err error
}

// rxMsg carries a chunk of bytes received from the device.
type rxMsg struct {
	data []byte
}

// rxClosedMsg is sent from the RX goroutine when the device disconnects or
// read returns a non-recoverable error.
type rxClosedMsg struct {
	err error
}

// txDoneMsg reports the result of a write.
type txDoneMsg struct {
	err error
}

// deviceInfoMsg carries parsed header fields from a profile's on-open query.
type deviceInfoMsg struct {
	fields map[string]string
}

func requestCloseCmd() tea.Cmd {
	return func() tea.Msg { return tui.CloseSerialTermMsg{} }
}
