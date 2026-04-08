package bluetooth

import (
	"log/slog"
	"time"

	"github.com/bluetuith-org/bluetooth-classic/api/bluetooth"

	tea "charm.land/bubbletea/v2"
)

// Message types for the bluetooth tab.
type (
	initMsg        struct{ adapter *Adapter }
	adapterInfoMsg AdapterInfo
	devicesMsg     []DeviceInfo
	errorMsg       struct{ err error }
	deviceAddedMsg struct{ data bluetooth.DeviceData }
	deviceUpdatedMsg struct {
		data bluetooth.DeviceEventData
	}
	deviceRemovedMsg struct {
		data bluetooth.DeviceEventData
	}
	adapterUpdatedMsg struct {
		data bluetooth.AdapterEventData
	}
	scanTickMsg struct{}
)

// Commands

func initAdapter() tea.Cmd {
	return func() tea.Msg {
		adapter, err := NewAdapter()
		if err != nil {
			slog.Error("failed to initialize adapter", "error", err)
			return errorMsg{err}
		}
		slog.Info("adapter initialized")
		return initMsg{adapter: adapter}
	}
}

func fetchAdapterInfo(adapter *Adapter) tea.Cmd {
	return func() tea.Msg {
		info, err := adapter.Info()
		if err != nil {
			slog.Error("failed to fetch adapter info", "error", err)
			return errorMsg{err}
		}
		return adapterInfoMsg(info)
	}
}

func fetchDevices(adapter *Adapter) tea.Cmd {
	return func() tea.Msg {
		devs, err := adapter.Devices()
		if err != nil {
			slog.Error("failed to fetch devices", "error", err)
			return errorMsg{err}
		}
		slog.Debug("fetched devices", "count", len(devs))
		return devicesMsg(devs)
	}
}

func startDiscovery(adapter *Adapter) tea.Cmd {
	return func() tea.Msg {
		if err := adapter.StartDiscovery(); err != nil {
			slog.Error("failed to start discovery", "error", err)
			return errorMsg{err}
		}
		slog.Info("discovery started")
		info, err := adapter.Info()
		if err != nil {
			slog.Error("failed to fetch adapter info after discovery start", "error", err)
			return errorMsg{err}
		}
		return adapterInfoMsg(info)
	}
}

func stopDiscovery(adapter *Adapter) tea.Cmd {
	return func() tea.Msg {
		adapter.StopDiscovery()
		return nil
	}
}

func connectDevice(adapter *Adapter, addr bluetooth.MacAddress) tea.Cmd {
	return func() tea.Msg {
		if err := adapter.ConnectDevice(addr); err != nil {
			slog.Error("failed to connect device", "address", addr, "error", err)
			return errorMsg{err}
		}
		slog.Info("device connected", "address", addr)
		devs, err := adapter.Devices()
		if err != nil {
			slog.Error("failed to fetch devices after connect", "error", err)
			return errorMsg{err}
		}
		return devicesMsg(devs)
	}
}

func disconnectDevice(adapter *Adapter, addr bluetooth.MacAddress) tea.Cmd {
	return func() tea.Msg {
		if err := adapter.DisconnectDevice(addr); err != nil {
			slog.Error("failed to disconnect device", "address", addr, "error", err)
			return errorMsg{err}
		}
		slog.Info("device disconnected", "address", addr)
		devs, err := adapter.Devices()
		if err != nil {
			slog.Error("failed to fetch devices after disconnect", "error", err)
			return errorMsg{err}
		}
		return devicesMsg(devs)
	}
}

func togglePower(adapter *Adapter) tea.Cmd {
	return func() tea.Msg {
		if err := adapter.TogglePower(); err != nil {
			slog.Error("failed to toggle power", "error", err)
			return errorMsg{err}
		}
		slog.Info("adapter power toggled")
		info, err := adapter.Info()
		if err != nil {
			slog.Error("failed to fetch adapter info after power toggle", "error", err)
			return errorMsg{err}
		}
		return adapterInfoMsg(info)
	}
}

func listenDeviceEvents(done <-chan struct{}) tea.Cmd {
	return func() tea.Msg {
		sub, ok := bluetooth.DeviceEvents().Subscribe()
		if !ok {
			return nil
		}

		select {
		case data := <-sub.AddedEvents:
			sub.Unsubscribe()
			return deviceAddedMsg{data}
		case data := <-sub.UpdatedEvents:
			sub.Unsubscribe()
			return deviceUpdatedMsg{data}
		case data := <-sub.RemovedEvents:
			sub.Unsubscribe()
			return deviceRemovedMsg{data}
		case <-sub.Done:
			return nil
		case <-done:
			sub.Unsubscribe()
			return nil
		}
	}
}

func listenAdapterEvents(done <-chan struct{}) tea.Cmd {
	return func() tea.Msg {
		sub, ok := bluetooth.AdapterEvents().Subscribe()
		if !ok {
			return nil
		}

		select {
		case data := <-sub.UpdatedEvents:
			sub.Unsubscribe()
			return adapterUpdatedMsg{data}
		case <-sub.Done:
			return nil
		case <-done:
			sub.Unsubscribe()
			return nil
		}
	}
}

// ScanInterval is the time between background device list refreshes.
const ScanInterval = 5 * time.Second

func scheduleScanTick() tea.Cmd {
	return tea.Tick(ScanInterval, func(time.Time) tea.Msg {
		return scanTickMsg{}
	})
}
