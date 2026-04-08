//go:build darwin

package bluetooth

import (
	"encoding/json"
	"log/slog"
	"os/exec"
	"sync"
)

// spOutput is the top-level structure from system_profiler SPBluetoothDataType -json.
type spOutput struct {
	SPBluetoothDataType []spEntry `json:"SPBluetoothDataType"`
}

// spEntry represents one Bluetooth controller section.
type spEntry struct {
	DeviceConnected    []map[string]spDeviceProps `json:"device_connected"`
	DeviceNotConnected []map[string]spDeviceProps `json:"device_not_connected"`
}

// spDeviceProps holds properties for a single device.
type spDeviceProps struct {
	Address   string `json:"device_address"`
	MinorType string `json:"device_minorType"`
}

// SystemProfilerScanner queries macOS system_profiler for paired Bluetooth devices.
type SystemProfilerScanner struct {
	devices []DeviceInfo
	mu      sync.RWMutex
}

// NewSystemProfilerScanner creates a new scanner.
func NewSystemProfilerScanner() *SystemProfilerScanner {
	return &SystemProfilerScanner{}
}

// Refresh runs system_profiler and updates the cached device list.
func (s *SystemProfilerScanner) Refresh() error {
	out, err := exec.Command("system_profiler", "SPBluetoothDataType", "-json").Output()
	if err != nil {
		return err
	}

	var parsed spOutput
	if err := json.Unmarshal(out, &parsed); err != nil {
		return err
	}

	var devices []DeviceInfo
	for _, entry := range parsed.SPBluetoothDataType {
		devices = append(devices, parseDeviceList(entry.DeviceConnected, true)...)
		devices = append(devices, parseDeviceList(entry.DeviceNotConnected, false)...)
	}

	s.mu.Lock()
	s.devices = devices
	s.mu.Unlock()

	slog.Debug("system_profiler refresh", "devices", len(devices))
	return nil
}

// Devices returns a snapshot of the cached device list.
func (s *SystemProfilerScanner) Devices() []DeviceInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]DeviceInfo, len(s.devices))
	copy(result, s.devices)
	return result
}

// parseDeviceList converts a system_profiler device array into DeviceInfo entries.
func parseDeviceList(list []map[string]spDeviceProps, connected bool) []DeviceInfo {
	var result []DeviceInfo
	for _, entry := range list {
		for name, props := range entry {
			devType := props.MinorType
			if devType == "" {
				devType = "Unknown"
			}
			result = append(result, DeviceInfo{
				Name:      name,
				Address:   props.Address,
				Type:      devType,
				Connected: connected,
				Paired:    true,
				Transport: TransportClassic,
			})
		}
	}
	return result
}
