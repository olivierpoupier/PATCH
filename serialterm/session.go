package serialterm

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

// Session owns the I/O lifecycle for a single open transport: the transport
// handle, a receive channel, and a done channel that signals the read
// goroutine to exit. Device views compose a Session as their I/O engine.
//
// Lifecycle:
//
//	cmd := serialterm.OpenSerialSession(path, baud)
//	// dispatch cmd; on OpenedMsg call sess.Install(msg) + sess.WaitRx()
//	// on each RxMsg, handle bytes then call sess.WaitRx() again
//	// on Close/disconnect, call sess.Close() / sess.MarkDisconnected()
type Session struct {
	transport Transport
	rxCh      chan tea.Msg
	doneCh    chan struct{}
}

// OpenSerialSession returns a command that opens a serial port off the event
// loop. Success posts an OpenedMsg; failure posts an OpenErrMsg.
func OpenSerialSession(path string, baud int) tea.Cmd {
	return func() tea.Msg {
		t, err := OpenSerial(path, baud)
		if err != nil {
			return OpenErrMsg{Err: fmt.Errorf("open %s: %w", path, err)}
		}
		rxCh := make(chan tea.Msg, 64)
		doneCh := make(chan struct{})
		go readLoop(t, rxCh, doneCh)
		return OpenedMsg{Transport: t, RxCh: rxCh, DoneCh: doneCh}
	}
}

// Install stores the transport and channels from an OpenedMsg onto the
// session so subsequent Send/WaitRx/Close calls operate on them.
func (s *Session) Install(msg OpenedMsg) {
	s.transport = msg.Transport
	s.rxCh = msg.RxCh
	s.doneCh = msg.DoneCh
}

// Send writes data to the underlying transport off the event loop. Returns
// nil when the session has no live transport or data is empty.
func (s *Session) Send(data []byte) tea.Cmd {
	if s == nil || s.transport == nil || len(data) == 0 {
		return nil
	}
	t := s.transport
	return func() tea.Msg {
		_, err := t.Write(data)
		return TxDoneMsg{Err: err}
	}
}

// WaitRx returns a command that blocks on the receive channel until the next
// chunk of bytes or a close event arrives. Call after each RxMsg to keep the
// listen loop alive.
func (s *Session) WaitRx() tea.Cmd {
	if s == nil || s.rxCh == nil {
		return nil
	}
	ch := s.rxCh
	return func() tea.Msg {
		m, ok := <-ch
		if !ok {
			return RxClosedMsg{}
		}
		return m
	}
}

// Active reports whether the session has a live transport.
func (s *Session) Active() bool {
	return s != nil && s.transport != nil
}

// Close stops the read goroutine and closes the transport. Safe to call on
// an already-closed session.
func (s *Session) Close() {
	if s == nil {
		return
	}
	if s.doneCh != nil {
		close(s.doneCh)
		s.doneCh = nil
	}
	if s.transport != nil {
		_ = s.transport.Close()
		s.transport = nil
	}
	s.rxCh = nil
}

// MarkDisconnected is called when an RxClosedMsg is observed. It releases the
// transport handle without closing doneCh, which the read goroutine has
// already returned from.
func (s *Session) MarkDisconnected() {
	if s == nil {
		return
	}
	if s.transport != nil {
		_ = s.transport.Close()
		s.transport = nil
	}
	s.rxCh = nil
	s.doneCh = nil
}

// readLoop pumps bytes from the transport into rxCh until the transport errors
// or doneCh is closed. It owns no Session state; all updates flow through
// rxCh. On exit the channel is closed so any blocked WaitRx unblocks with an
// RxClosedMsg rather than leaking a goroutine.
func readLoop(t Transport, out chan<- tea.Msg, done <-chan struct{}) {
	defer close(out)
	buf := make([]byte, 4096)
	for {
		select {
		case <-done:
			return
		default:
		}
		n, err := t.Read(buf)
		if err != nil {
			select {
			case <-done:
			case out <- RxClosedMsg{Err: err}:
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
		case out <- RxMsg{Data: chunk}:
		}
	}
}
