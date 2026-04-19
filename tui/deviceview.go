package tui

import tea "charm.land/bubbletea/v2"

// DeviceView is the interface the app uses to delegate work to a per-device
// modal view. Each device package implements it and registers a factory with
// the devices registry via init().
//
// Shape mirrors Tab: the Update method returns DeviceView so the app's
// dispatch remains type-safe; individual device Models may still return
// themselves via a type assertion inside their own Update.
type DeviceView interface {
	// Open is called once per modal opening, after construction, with the
	// resolved device info. Returns the initial command (typically opens the
	// underlying serial session).
	Open(info SerialDeviceInfo) tea.Cmd

	// Update handles all messages while the modal is visible.
	Update(msg tea.Msg) (DeviceView, tea.Cmd)

	// View renders the full-screen modal.
	View(width, height int) string

	// SetSize stores the current full-screen dimensions.
	SetSize(width, height int)

	// Close tears down any live I/O. Must be safe to call on a freshly
	// constructed view.
	Close()

	// Name returns a short identifier for logs and error messages.
	Name() string
}
