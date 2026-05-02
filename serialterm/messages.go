package serialterm

import tea "charm.land/bubbletea/v2"

// OpenedMsg is posted when a Session's transport has been successfully opened.
// Device views should call Session.Install(msg) then start listening via
// Session.WaitRx.
type OpenedMsg struct {
	Transport Transport
	RxCh      chan tea.Msg
	DoneCh    chan struct{}
}

// OpenErrMsg is posted when opening the transport failed.
type OpenErrMsg struct {
	Err error
}

// RxMsg carries a chunk of bytes received from the device. Device views
// typically feed the bytes into a Scrollback and call Session.WaitRx again.
type RxMsg struct {
	Data []byte
}

// RxClosedMsg is posted when the device disconnects or the read goroutine
// exits with a non-recoverable error.
type RxClosedMsg struct {
	Err error
}

// TxDoneMsg reports the result of a Session.Send call.
type TxDoneMsg struct {
	Err error
}
