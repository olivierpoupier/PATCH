package serialterm

import (
	"fmt"
	"time"

	"github.com/olivierpoupier/patch/tui"

	tea "charm.land/bubbletea/v2"
	"go.bug.st/serial"
)

// portOpenedMsg carries the port + channels from the open goroutine back to
// Update, which stores them on the model before starting the RX loop.
type portOpenedMsg struct {
	port   serial.Port
	rxCh   chan tea.Msg
	doneCh chan struct{}
}

// captureEndMsg fires after the header-query response window expires.
type captureEndMsg struct{}

// openSerialCmd opens the serial port off the event loop. Success posts a
// portOpenedMsg so Update can install the port+channels and kick off reading;
// failure posts openErrMsg.
func openSerialCmd(info tui.SerialDeviceInfo) tea.Cmd {
	return func() tea.Msg {
		mode := &serial.Mode{
			BaudRate: info.Baud,
			DataBits: 8,
			Parity:   serial.NoParity,
			StopBits: serial.OneStopBit,
		}
		port, err := serial.Open(info.PortPath, mode)
		if err != nil {
			return openErrMsg{err: fmt.Errorf("open %s: %w", info.PortPath, err)}
		}
		rxCh := make(chan tea.Msg, 64)
		doneCh := make(chan struct{})
		go readLoop(port, rxCh, doneCh)
		return portOpenedMsg{port: port, rxCh: rxCh, doneCh: doneCh}
	}
}

// readLoop pumps bytes from the port into rxCh until the port errors or the
// doneCh is closed. It owns no model state; all updates flow through rxCh.
// On exit the channel is closed so any blocked waitRxCmd unblocks with an
// rxClosedMsg rather than leaking a goroutine.
func readLoop(port serial.Port, out chan<- tea.Msg, done <-chan struct{}) {
	defer close(out)
	buf := make([]byte, 4096)
	for {
		select {
		case <-done:
			return
		default:
		}
		n, err := port.Read(buf)
		if err != nil {
			select {
			case <-done:
			case out <- rxClosedMsg{err: err}:
			}
			return
		}
		if n <= 0 {
			continue
		}
		chunk := make([]byte, n)
		copy(chunk, buf[:n])
		select {
		case <-done:
			return
		case out <- rxMsg{data: chunk}:
		}
	}
}

// waitRxCmd blocks on rxCh until a message arrives, then returns it. Update
// must keep calling this after each rxMsg to maintain the listen loop.
func waitRxCmd(rxCh <-chan tea.Msg) tea.Cmd {
	if rxCh == nil {
		return nil
	}
	return func() tea.Msg {
		m, ok := <-rxCh
		if !ok {
			return rxClosedMsg{}
		}
		return m
	}
}

// txCmd writes bytes to the port off the event loop and reports completion.
func txCmd(port serial.Port, data []byte) tea.Cmd {
	if port == nil || len(data) == 0 {
		return nil
	}
	return func() tea.Msg {
		_, err := port.Write(data)
		return txDoneMsg{err: err}
	}
}

// captureWindowCmd fires captureEndMsg after d elapses.
func captureWindowCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg {
		return captureEndMsg{}
	})
}
