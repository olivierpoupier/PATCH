package bluetooth

import (
	"log/slog"
	"time"

	"github.com/bluetuith-org/bluetooth-classic/api/bluetooth"

	tea "charm.land/bubbletea/v2"
)

// Message types for the bluetooth tab.
type (
	initMsg        struct{ adapter *CompositeAdapter }
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
	scanTickMsg       struct{}
	bleScanTickMsg    struct{}
	sysProfTickMsg    struct{}
	scrollAnimTickMsg struct{}
)

// Commands

func initCompositeAdapter() tea.Cmd {
	return func() tea.Msg {
		adapter, err := NewCompositeAdapter()
		if err != nil {
			slog.Error("failed to initialize adapter", "error", err)
			return errorMsg{err}
		}
		slog.Info("composite adapter initialized")
		return initMsg{adapter: adapter}
	}
}

func fetchAdapterInfo(adapter *CompositeAdapter) tea.Cmd {
	return func() tea.Msg {
		info, err := adapter.Info()
		if err != nil {
			slog.Error("failed to fetch adapter info", "error", err)
			return errorMsg{err}
		}
		return adapterInfoMsg(info)
	}
}

func fetchDevices(adapter *CompositeAdapter) tea.Cmd {
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

func startDiscovery(adapter *CompositeAdapter) tea.Cmd {
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

func stopDiscovery(adapter *CompositeAdapter) tea.Cmd {
	return func() tea.Msg {
		adapter.StopDiscovery()
		return nil
	}
}

func connectDevice(adapter *CompositeAdapter, dev DeviceInfo) tea.Cmd {
	return func() tea.Msg {
		if err := adapter.ConnectDevice(dev); err != nil {
			slog.Error("failed to connect device", "address", dev.Address, "error", err)
			return errorMsg{err}
		}
		slog.Info("device connected", "address", dev.Address)
		devs, err := adapter.Devices()
		if err != nil {
			slog.Error("failed to fetch devices after connect", "error", err)
			return errorMsg{err}
		}
		return devicesMsg(devs)
	}
}

func disconnectDevice(adapter *CompositeAdapter, dev DeviceInfo) tea.Cmd {
	return func() tea.Msg {
		if err := adapter.DisconnectDevice(dev); err != nil {
			slog.Error("failed to disconnect device", "address", dev.Address, "error", err)
			return errorMsg{err}
		}
		slog.Info("device disconnected", "address", dev.Address)
		devs, err := adapter.Devices()
		if err != nil {
			slog.Error("failed to fetch devices after disconnect", "error", err)
			return errorMsg{err}
		}
		return devicesMsg(devs)
	}
}

func togglePower(adapter *CompositeAdapter) tea.Cmd {
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

func scheduleBLEScanTick() tea.Cmd {
	return tea.Tick(ScanInterval, func(time.Time) tea.Msg {
		return bleScanTickMsg{}
	})
}

// SysProfInterval is the time between system profiler refreshes.
const SysProfInterval = 10 * time.Second

func scheduleSysProfTick() tea.Cmd {
	return tea.Tick(SysProfInterval, func(time.Time) tea.Msg {
		return sysProfTickMsg{}
	})
}

func refreshSystemProfiler(adapter *CompositeAdapter) tea.Cmd {
	return func() tea.Msg {
		if err := adapter.RefreshSystemProfiler(); err != nil {
			slog.Warn("system profiler refresh failed", "error", err)
		}
		devs, err := adapter.Devices()
		if err != nil {
			return errorMsg{err}
		}
		return devicesMsg(devs)
	}
}

// ScrollAnimInterval is the time between scroll animation frames.
const ScrollAnimInterval = 400 * time.Millisecond

func scheduleScrollAnimTick() tea.Cmd {
	return tea.Tick(ScrollAnimInterval, func(time.Time) tea.Msg {
		return scrollAnimTickMsg{}
	})
}
