package tui

import tea "charm.land/bubbletea/v2"

// Tab represents a peripheral tab in the application.
// Each peripheral (Bluetooth, USB, etc.) implements this interface.
type Tab interface {
	Name() string
	Init() tea.Cmd
	Update(msg tea.Msg) (Tab, tea.Cmd)
	View(width, height int) string
	SetFocused(focused bool) (Tab, tea.Cmd)
	SetActive(active bool) (Tab, tea.Cmd)
}
