package wifi

import (
	"log/slog"
	"time"

	tea "charm.land/bubbletea/v2"
)

// Message types for the wifi tab.
type (
	initMsg             struct{ client *WLANClient }
	interfaceInfoMsg    InterfaceInfo
	connectionMsg       ConnectionInfo
	networksMsg         []NetworkInfo
	errorMsg            struct{ err error }
	connectionTickMsg   struct{}
	scanTickMsg         struct{}
	powerToggleMsg      InterfaceInfo
	connectResultMsg    struct{ err error }
	disconnectResultMsg struct{ err error }
)

// Polling intervals.
const (
	ConnectionInterval = 5 * time.Second
	ScanInterval       = 10 * time.Second
)

func initClient() tea.Cmd {
	return func() tea.Msg {
		client := NewWLANClient()
		slog.Info("wifi client initialized")
		return initMsg{client: client}
	}
}

func fetchInterfaceInfo(client *WLANClient) tea.Cmd {
	return func() tea.Msg {
		info, err := client.InterfaceInfo()
		if err != nil {
			slog.Error("failed to fetch wifi interface info", "error", err)
			return errorMsg{err}
		}
		return interfaceInfoMsg(info)
	}
}

func fetchConnection(client *WLANClient) tea.Cmd {
	return func() tea.Msg {
		conn, err := client.ConnectionInfo()
		if err != nil {
			slog.Error("failed to fetch wifi connection", "error", err)
			return errorMsg{err}
		}
		return connectionMsg(conn)
	}
}

func scanNetworksCmd(client *WLANClient) tea.Cmd {
	return func() tea.Msg {
		networks, err := client.ScanNetworks()
		if err != nil {
			slog.Warn("wifi scan failed", "error", err)
			return errorMsg{err}
		}
		slog.Debug("wifi scan complete", "count", len(networks))
		return networksMsg(networks)
	}
}

func scheduleConnectionTick() tea.Cmd {
	return tea.Tick(ConnectionInterval, func(time.Time) tea.Msg {
		return connectionTickMsg{}
	})
}

func scheduleScanTick() tea.Cmd {
	return tea.Tick(ScanInterval, func(time.Time) tea.Msg {
		return scanTickMsg{}
	})
}

func togglePower(client *WLANClient) tea.Cmd {
	return func() tea.Msg {
		info, err := client.InterfaceInfo()
		if err != nil {
			slog.Error("failed to read interface for power toggle", "error", err)
			return errorMsg{err}
		}
		if err := client.SetPower(!info.PowerOn); err != nil {
			slog.Error("failed to toggle wifi power", "error", err)
			return errorMsg{err}
		}
		slog.Info("wifi power toggled", "newState", !info.PowerOn)
		info, err = client.InterfaceInfo()
		if err != nil {
			slog.Error("failed to read interface after power toggle", "error", err)
			return errorMsg{err}
		}
		return powerToggleMsg(info)
	}
}

func connectToNetworkCmd(client *WLANClient, ssid, password string) tea.Cmd {
	return func() tea.Msg {
		slog.Info("connecting to wifi network", "ssid", ssid)
		if err := client.Connect(ssid, password); err != nil {
			slog.Error("failed to connect to wifi", "ssid", ssid, "error", err)
			return connectResultMsg{err: err}
		}
		slog.Info("wifi connected", "ssid", ssid)
		return connectResultMsg{}
	}
}

func disconnectCmd(client *WLANClient) tea.Cmd {
	return func() tea.Msg {
		slog.Info("disconnecting from wifi")
		client.Disconnect()
		slog.Info("wifi disconnected")
		return disconnectResultMsg{}
	}
}
