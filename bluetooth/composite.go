package bluetooth

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"

	btapi "github.com/bluetuith-org/bluetooth-classic/api/bluetooth"
)

// CompositeAdapter merges Classic, BLE, and system-level device sources.
type CompositeAdapter struct {
	classic *Adapter
	ble     *BLEScanner
	sysProf *SystemProfilerScanner
}

// NewCompositeAdapter initializes both Classic and BLE adapters.
// Succeeds if at least one adapter is available.
func NewCompositeAdapter() (*CompositeAdapter, error) {
	c := &CompositeAdapter{}

	classic, classicErr := NewAdapter()
	if classicErr != nil {
		slog.Warn("classic bluetooth unavailable", "error", classicErr)
	} else {
		c.classic = classic
	}

	ble, bleErr := NewBLEScanner()
	if bleErr != nil {
		slog.Warn("BLE unavailable", "error", bleErr)
	} else {
		c.ble = ble
	}

	c.sysProf = NewSystemProfilerScanner()
	if err := c.sysProf.Refresh(); err != nil {
		slog.Warn("system profiler initial refresh failed", "error", err)
	}

	if c.classic == nil && c.ble == nil && len(c.sysProf.Devices()) == 0 {
		return nil, fmt.Errorf("no bluetooth adapters available: classic=%v, ble=%v", classicErr, bleErr)
	}
	return c, nil
}

// Info retrieves adapter properties from the Classic adapter.
func (c *CompositeAdapter) Info() (AdapterInfo, error) {
	if c.classic != nil {
		return c.classic.Info()
	}
	return AdapterInfo{Name: "BLE Only", Powered: true}, nil
}

// Devices returns a merged, deduplicated list from all adapters.
func (c *CompositeAdapter) Devices() ([]DeviceInfo, error) {
	var sources [][]DeviceInfo

	// System profiler first — it has authoritative Connected/Paired state.
	if c.sysProf != nil {
		sources = append(sources, c.sysProf.Devices())
	}

	if c.classic != nil {
		classicDevs, err := c.classic.Devices()
		if err != nil {
			slog.Error("failed to fetch classic devices", "error", err)
		} else {
			sources = append(sources, classicDevs)
		}
	}

	if c.ble != nil {
		sources = append(sources, c.ble.Devices())
	}

	merged := mergeDevices(sources...)
	return merged, nil
}

// RefreshSystemProfiler refreshes the macOS system profiler device cache.
func (c *CompositeAdapter) RefreshSystemProfiler() error {
	if c.sysProf != nil {
		return c.sysProf.Refresh()
	}
	return nil
}

// StartDiscovery starts discovery on both adapters.
func (c *CompositeAdapter) StartDiscovery() error {
	if c.classic != nil {
		if err := c.classic.StartDiscovery(); err != nil {
			slog.Error("classic discovery failed", "error", err)
		}
	}
	return nil
}

// StopDiscovery stops discovery on both adapters.
func (c *CompositeAdapter) StopDiscovery() error {
	if c.classic != nil {
		c.classic.StopDiscovery()
	}
	return nil
}

// StartBLEScan starts BLE scanning with a done channel for cancellation.
func (c *CompositeAdapter) StartBLEScan(done <-chan struct{}) {
	if c.ble != nil {
		c.ble.StartScan(done)
	}
}

// StopBLEScan stops BLE scanning.
func (c *CompositeAdapter) StopBLEScan() {
	if c.ble != nil {
		c.ble.StopScan()
	}
}

// ConnectDevice routes to the correct adapter based on transport.
func (c *CompositeAdapter) ConnectDevice(dev DeviceInfo) error {
	switch dev.Transport {
	case TransportBLE:
		if c.ble != nil {
			return c.ble.Connect(dev.Address)
		}
		return fmt.Errorf("BLE not available")
	default:
		if c.classic != nil {
			return c.classic.ConnectDevice(dev.ClassicAddr)
		}
		return fmt.Errorf("classic bluetooth not available")
	}
}

// DisconnectDevice routes to the correct adapter based on transport.
func (c *CompositeAdapter) DisconnectDevice(dev DeviceInfo) error {
	switch dev.Transport {
	case TransportBLE:
		if c.ble != nil {
			return c.ble.Disconnect(dev.Address)
		}
		return fmt.Errorf("BLE not available")
	default:
		if c.classic != nil {
			return c.classic.DisconnectDevice(dev.ClassicAddr)
		}
		return fmt.Errorf("classic bluetooth not available")
	}
}

// TogglePower toggles the Classic adapter power state.
func (c *CompositeAdapter) TogglePower() error {
	if c.classic != nil {
		return c.classic.TogglePower()
	}
	return fmt.Errorf("classic bluetooth not available")
}

// HasClassic returns true if the Classic adapter is available.
func (c *CompositeAdapter) HasClassic() bool {
	return c.classic != nil
}

// ClassicAdapter returns the underlying Classic adapter (for event subscriptions).
func (c *CompositeAdapter) ClassicAdapter() *Adapter {
	return c.classic
}

// Close cleans up both adapters.
func (c *CompositeAdapter) Close() error {
	if c.ble != nil {
		c.ble.Close()
	}
	if c.classic != nil {
		return c.classic.Close()
	}
	return nil
}

// ClassicMACAddress parses a DeviceInfo's address back to a MacAddress for Classic operations.
func (c *CompositeAdapter) ClassicMACAddress(dev DeviceInfo) btapi.MacAddress {
	return dev.ClassicAddr
}

// mergeDevices combines device lists from multiple sources, deduplicating by name.
// Sources should be ordered by authority (system profiler first, then classic, then BLE).
func mergeDevices(sources ...[]DeviceInfo) []DeviceInfo {
	byName := make(map[string]int) // name -> index in result
	var result []DeviceInfo

	for _, source := range sources {
		for i := range source {
			dev := source[i]
			key := strings.ToLower(dev.Name)
			if idx, ok := byName[key]; ok {
				// Seen from multiple sources: mark dual transport.
				if result[idx].Transport != dev.Transport {
					result[idx].Transport = TransportDual
				}
				// Prefer connected/paired from any source.
				if dev.Connected {
					result[idx].Connected = true
				}
				if dev.Paired {
					result[idx].Paired = true
				}
			} else {
				byName[key] = len(result)
				result = append(result, dev)
			}
		}
	}

	// Sort: connected first, then paired, then alphabetical.
	sort.Slice(result, func(i, j int) bool {
		if result[i].Connected != result[j].Connected {
			return result[i].Connected
		}
		if result[i].Paired != result[j].Paired {
			return result[i].Paired
		}
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})

	return result
}
