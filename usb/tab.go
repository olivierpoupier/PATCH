package usb

import (
	"github.com/olivierpoupier/patch/tui"

	tea "charm.land/bubbletea/v2"
)

// Model is the USB tab's Bubbletea model.
type Model struct {
	focused bool
}

// New creates a new USB tab model.
func New() *Model {
	return &Model{}
}

// Name returns the tab display name.
func (m *Model) Name() string { return "USB" }

// Init returns nil; no initialization needed yet.
func (m *Model) Init() tea.Cmd { return nil }

// Update is a no-op for the placeholder tab.
func (m *Model) Update(msg tea.Msg) (tui.Tab, tea.Cmd) {
	return m, nil
}

// View renders the placeholder content.
func (m *Model) View(width int) string {
	return "  USB device management coming soon..."
}

// SetFocused is a no-op for the placeholder tab.
func (m *Model) SetFocused(focused bool) (tui.Tab, tea.Cmd) {
	m.focused = focused
	return m, nil
}
