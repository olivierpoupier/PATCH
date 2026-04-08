package bluetooth

import (
	"log/slog"
	"strings"
	"sync"
	"time"

	"tinygo.org/x/bluetooth"
)

const bleDeviceExpiry = 60 * time.Second

// bleDevice holds internal state for a discovered BLE device.
type bleDevice struct {
	address   bluetooth.Address
	name      string
	rssi      int16
	connected bool
	lastSeen  time.Time
}

// BLEScanner wraps tinygo.org/x/bluetooth for BLE device discovery.
type BLEScanner struct {
	adapter  *bluetooth.Adapter
	devices  map[string]*bleDevice // keyed by address string
	mu       sync.RWMutex
	scanning bool
}

// NewBLEScanner creates and enables a BLE scanner.
func NewBLEScanner() (*BLEScanner, error) {
	adapter := bluetooth.DefaultAdapter
	if err := adapter.Enable(); err != nil {
		return nil, err
	}
	slog.Debug("BLE adapter enabled")
	return &BLEScanner{
		adapter: adapter,
		devices: make(map[string]*bleDevice),
	}, nil
}

// StartScan begins BLE scanning in a background goroutine.
// It stops when the done channel is closed.
func (s *BLEScanner) StartScan(done <-chan struct{}) {
	s.mu.Lock()
	if s.scanning {
		s.mu.Unlock()
		return
	}
	s.scanning = true
	s.mu.Unlock()

	go func() {
		slog.Debug("BLE scan starting")
		err := s.adapter.Scan(func(_ *bluetooth.Adapter, result bluetooth.ScanResult) {
			name := result.LocalName()
			if name == "" {
				return // skip unnamed devices
			}

			key := result.Address.String()
			s.mu.Lock()
			dev, exists := s.devices[key]
			if exists {
				dev.name = name
				dev.rssi = result.RSSI
				dev.lastSeen = time.Now()
			} else {
				s.devices[key] = &bleDevice{
					address:  result.Address,
					name:     name,
					rssi:     result.RSSI,
					lastSeen: time.Now(),
				}
			}
			s.mu.Unlock()
		})
		if err != nil {
			slog.Error("BLE scan error", "error", err)
		}
	}()

	go func() {
		<-done
		s.StopScan()
	}()
}

// StopScan stops BLE scanning.
func (s *BLEScanner) StopScan() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.scanning {
		return
	}
	s.scanning = false
	if err := s.adapter.StopScan(); err != nil {
		slog.Error("BLE stop scan error", "error", err)
	}
	slog.Debug("BLE scan stopped")
}

// Devices returns a snapshot of discovered BLE devices, pruning stale entries.
func (s *BLEScanner) Devices() []DeviceInfo {
	now := time.Now()
	s.mu.Lock()
	for key, dev := range s.devices {
		if now.Sub(dev.lastSeen) > bleDeviceExpiry {
			delete(s.devices, key)
		}
	}
	s.mu.Unlock()

	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]DeviceInfo, 0, len(s.devices))
	for _, dev := range s.devices {
		result = append(result, DeviceInfo{
			Name:      dev.name,
			Address:   dev.address.String(),
			Type:      classifyBLEDevice(dev.name),
			Connected: dev.connected,
			Transport: TransportBLE,
		})
	}
	return result
}

// Connect connects to a BLE device by address string.
func (s *BLEScanner) Connect(addr string) error {
	s.mu.RLock()
	dev, ok := s.devices[addr]
	s.mu.RUnlock()
	if !ok {
		return nil
	}

	slog.Debug("BLE connecting", "address", addr)
	_, err := s.adapter.Connect(dev.address, bluetooth.ConnectionParams{})
	if err != nil {
		return err
	}

	s.mu.Lock()
	dev.connected = true
	s.mu.Unlock()
	return nil
}

// Disconnect disconnects from a BLE device by address string.
func (s *BLEScanner) Disconnect(addr string) error {
	s.mu.RLock()
	dev, ok := s.devices[addr]
	s.mu.RUnlock()
	if !ok {
		return nil
	}

	slog.Debug("BLE disconnecting", "address", addr)
	device, err := s.adapter.Connect(dev.address, bluetooth.ConnectionParams{})
	if err != nil {
		return err
	}
	err = device.Disconnect()

	s.mu.Lock()
	dev.connected = false
	s.mu.Unlock()
	return err
}

// Close stops scanning.
func (s *BLEScanner) Close() {
	s.StopScan()
}

// classifyBLEDevice infers a device type from its name when no class info is available.
func classifyBLEDevice(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "keyboard"):
		return "Keyboard"
	case strings.Contains(lower, "mouse"):
		return "Mouse"
	case strings.Contains(lower, "trackpad"):
		return "Mouse"
	case strings.Contains(lower, "headphone"), strings.Contains(lower, "airpod"),
		strings.Contains(lower, "earbud"):
		return "Headphones"
	case strings.Contains(lower, "speaker"):
		return "Speakers"
	case strings.Contains(lower, "headset"):
		return "Headset"
	case strings.Contains(lower, "watch"):
		return "Wearable"
	case strings.Contains(lower, "phone"), strings.Contains(lower, "iphone"),
		strings.Contains(lower, "android"):
		return "Phone"
	case strings.Contains(lower, "ipad"), strings.Contains(lower, "tablet"):
		return "Computer"
	case strings.Contains(lower, "gamepad"), strings.Contains(lower, "controller"),
		strings.Contains(lower, "joy-con"):
		return "Gaming input"
	default:
		return "BLE Device"
	}
}
