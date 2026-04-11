package usb

import (
	"log/slog"
	"time"

	tea "charm.land/bubbletea/v2"
)

const refreshInterval = 5 * time.Second

type (
	initMsg struct {
		scanner *USBScanner
	}
	dataMsg struct {
		groups []PortGroup
	}
	errorMsg struct {
		err error
	}
	refreshTickMsg struct{}
)

func initScanner() tea.Cmd {
	return func() tea.Msg {
		scanner := NewUSBScanner()
		if err := scanner.Refresh(); err != nil {
			slog.Error("usb scanner init failed", "error", err)
			return errorMsg{err: err}
		}
		return initMsg{scanner: scanner}
	}
}

func fetchData(scanner *USBScanner) tea.Cmd {
	return func() tea.Msg {
		if err := scanner.Refresh(); err != nil {
			slog.Warn("usb refresh failed", "error", err)
			return errorMsg{err: err}
		}
		buses, ports := scanner.Data()
		groups := buildPortGroups(buses, ports)
		return dataMsg{groups: groups}
	}
}

func scheduleRefreshTick() tea.Cmd {
	return tea.Tick(refreshInterval, func(time.Time) tea.Msg {
		return refreshTickMsg{}
	})
}
