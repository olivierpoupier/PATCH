//go:build darwin

package usb

import (
	"encoding/json"
	"log/slog"
	"os/exec"
	"sync"
)

// JSON structures matching system_profiler SPUSBHostDataType output.
type spUSBOutput struct {
	SPUSBHostDataType []spUSBBus `json:"SPUSBHostDataType"`
}

type spUSBBus struct {
	Name         string      `json:"_name"`
	Driver       string      `json:"Driver"`
	HardwareType string      `json:"USBKeyHardwareType"`
	LocationID   string      `json:"USBKeyLocationID"`
	Items        []spUSBItem `json:"_items"`
}

type spUSBItem struct {
	Name            string      `json:"_name"`
	VendorName      string      `json:"USBDeviceKeyVendorName"`
	VendorID        string      `json:"USBDeviceKeyVendorID"`
	ProductID       string      `json:"USBDeviceKeyProductID"`
	LinkSpeed       string      `json:"USBDeviceKeyLinkSpeed"`
	HardwareType    string      `json:"USBKeyHardwareType"`
	LocationID      string      `json:"USBKeyLocationID"`
	SerialNumber    string      `json:"USBDeviceKeySerialNumber"`
	PowerAllocation string      `json:"USBDeviceKeyPowerAllocation"`
	Items           []spUSBItem `json:"_items"`
}

// JSON structures matching system_profiler SPThunderboltDataType output.
type spTBOutput struct {
	SPThunderboltDataType []spTBBus `json:"SPThunderboltDataType"`
}

type spTBBus struct {
	Name        string          `json:"_name"`
	DeviceName  string          `json:"device_name_key"`
	VendorName  string          `json:"vendor_name_key"`
	Receptacle1 *spTBReceptacle `json:"receptacle_1_tag"`
	Receptacle2 *spTBReceptacle `json:"receptacle_2_tag"`
}

type spTBReceptacle struct {
	ReceptacleID string `json:"receptacle_id_key"`
	Status       string `json:"receptacle_status_key"`
	CurrentSpeed string `json:"current_speed_key"`
}

// USBScanner queries macOS system_profiler for USB device information.
type USBScanner struct {
	buses []USBBus
	ports []ThunderboltPort
	mu    sync.RWMutex
}

// NewUSBScanner creates a new scanner.
func NewUSBScanner() *USBScanner {
	return &USBScanner{}
}

// Refresh re-reads USB and Thunderbolt data from system_profiler.
func (s *USBScanner) Refresh() error {
	usbOut, err := exec.Command("system_profiler", "SPUSBHostDataType", "-json").Output()
	if err != nil {
		return err
	}
	var usbParsed spUSBOutput
	if err := json.Unmarshal(usbOut, &usbParsed); err != nil {
		return err
	}

	// Thunderbolt is best-effort; not all Macs have it.
	var ports []ThunderboltPort
	tbOut, tbErr := exec.Command("system_profiler", "SPThunderboltDataType", "-json").Output()
	if tbErr == nil {
		var tbParsed spTBOutput
		if json.Unmarshal(tbOut, &tbParsed) == nil {
			ports = parseTBPorts(tbParsed)
		}
	} else {
		slog.Debug("thunderbolt profiler unavailable", "error", tbErr)
	}

	buses := convertBuses(usbParsed)

	s.mu.Lock()
	s.buses = buses
	s.ports = ports
	s.mu.Unlock()
	return nil
}

// Data returns copies of the cached buses and ports.
func (s *USBScanner) Data() ([]USBBus, []ThunderboltPort) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	buses := make([]USBBus, len(s.buses))
	copy(buses, s.buses)
	ports := make([]ThunderboltPort, len(s.ports))
	copy(ports, s.ports)
	return buses, ports
}

func convertBuses(parsed spUSBOutput) []USBBus {
	var buses []USBBus
	for _, spBus := range parsed.SPUSBHostDataType {
		bus := USBBus{
			Name:         spBus.Name,
			HardwareType: spBus.HardwareType,
			LocationID:   spBus.LocationID,
		}
		for _, item := range spBus.Items {
			bus.Devices = append(bus.Devices, convertItem(item))
		}
		buses = append(buses, bus)
	}
	return buses
}

func convertItem(item spUSBItem) *USBDevice {
	dev := &USBDevice{
		Name:         item.Name,
		VendorName:   item.VendorName,
		VendorID:     item.VendorID,
		ProductID:    item.ProductID,
		LinkSpeed:    item.LinkSpeed,
		HardwareType: item.HardwareType,
		LocationID:   item.LocationID,
		SerialNumber: item.SerialNumber,
	}
	dev.Type = classifyDevice(dev.Name, dev.VendorName)
	for _, child := range item.Items {
		dev.Children = append(dev.Children, convertItem(child))
	}
	return dev
}

func parseTBPorts(parsed spTBOutput) []ThunderboltPort {
	var ports []ThunderboltPort
	for _, bus := range parsed.SPThunderboltDataType {
		for _, rec := range []*spTBReceptacle{bus.Receptacle1, bus.Receptacle2} {
			if rec == nil {
				continue
			}
			ports = append(ports, ThunderboltPort{
				BusName:      bus.Name,
				ReceptacleID: rec.ReceptacleID,
				CurrentSpeed: rec.CurrentSpeed,
			})
		}
	}
	return ports
}
