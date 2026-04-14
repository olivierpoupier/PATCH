//go:build linux

package usb

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

const sysfsUSBPath = "/sys/bus/usb/devices"

var (
	rootHubRe = regexp.MustCompile(`^usb(\d+)$`)
	deviceRe  = regexp.MustCompile(`^(\d+)-[\d.]+$`)
)

// USBScanner queries Linux sysfs for USB device information.
type USBScanner struct {
	buses []USBBus
	ports []ThunderboltPort
	mu    sync.RWMutex
}

// NewUSBScanner creates a new scanner.
func NewUSBScanner() *USBScanner {
	return &USBScanner{}
}

// Refresh re-reads USB data from sysfs and storage volumes from lsblk.
func (s *USBScanner) Refresh() error {
	buses, err := parseUSBDevices()
	if err != nil {
		return err
	}

	volumes := parseLsblkVolumes()
	if len(volumes) > 0 {
		correlateLinuxVolumes(buses, volumes)
	}

	s.mu.Lock()
	s.buses = buses
	s.ports = nil // no Thunderbolt port info on Linux
	s.mu.Unlock()
	return nil
}

// Data returns copies of the cached buses and ports.
func (s *USBScanner) Data() ([]USBBus, []ThunderboltPort) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	buses := make([]USBBus, len(s.buses))
	copy(buses, s.buses)
	return buses, nil
}

// readSysfsAttr reads a single sysfs attribute file, returning "" on any error.
func readSysfsAttr(devicePath, attr string) string {
	data, err := os.ReadFile(filepath.Join(devicePath, attr))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// parseSpeedToLinkSpeed converts a Linux sysfs speed value (in Mbps) to a
// human-readable label matching the macOS format.
func parseSpeedToLinkSpeed(speedMbps string) string {
	switch speedMbps {
	case "1.5":
		return "Up to 1.5 Mb/s"
	case "12":
		return "Up to 12 Mb/s"
	case "480":
		return "Up to 480 Mb/s"
	case "5000":
		return "Up to 5 Gb/s"
	case "10000":
		return "Up to 10 Gb/s"
	case "20000":
		return "Up to 20 Gb/s"
	default:
		if speedMbps != "" {
			return speedMbps + " Mb/s"
		}
		return ""
	}
}

// synthesizeLocationID builds a LocationID in the format "0xBBPPPPPP" from
// a bus number and port path string. The high byte encodes the bus number
// so that busIndex() in tree.go can extract it correctly.
func synthesizeLocationID(busNum int, portPath string) string {
	loc := uint32(busNum) << 24
	if portPath != "" {
		parts := strings.Split(portPath, ".")
		for i, p := range parts {
			if i >= 3 {
				break
			}
			n, _ := strconv.Atoi(p)
			loc |= uint32(n&0xFF) << uint(16-i*8)
		}
	}
	return fmt.Sprintf("0x%08x", loc)
}

// parentSysfsName returns the parent device's sysfs name.
// "1-2.3.1" → "1-2.3", "1-2.3" → "1-2", "1-2" → "" (direct bus child).
func parentSysfsName(name string) string {
	if idx := strings.LastIndex(name, "."); idx != -1 {
		return name[:idx]
	}
	return ""
}

// parseBusNum extracts the bus number from a sysfs entry name.
// "usb3" → 3, "1-2.3" → 1.
func parseBusNum(sysfsName string) int {
	if m := rootHubRe.FindStringSubmatch(sysfsName); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	if idx := strings.Index(sysfsName, "-"); idx > 0 {
		n, _ := strconv.Atoi(sysfsName[:idx])
		return n
	}
	return 0
}

// portPath extracts the port path portion of a sysfs device name.
// "1-2" → "2", "1-2.3" → "2.3".
func portPath(sysfsName string) string {
	if idx := strings.Index(sysfsName, "-"); idx >= 0 {
		return sysfsName[idx+1:]
	}
	return ""
}

// classifyByDeviceClass maps USB bDeviceClass codes to DeviceType.
func classifyByDeviceClass(class string) DeviceType {
	switch strings.TrimSpace(class) {
	case "09":
		return DeviceTypeHub
	case "08":
		return DeviceTypeStorage
	case "01":
		return DeviceTypeAudio
	case "0e":
		return DeviceTypeVideo
	case "07":
		return DeviceTypePrinter
	case "02":
		return DeviceTypeNetwork
	default:
		return DeviceTypeUnknown
	}
}

// parseUSBDevices reads /sys/bus/usb/devices and builds a []USBBus hierarchy.
func parseUSBDevices() ([]USBBus, error) {
	entries, err := os.ReadDir(sysfsUSBPath)
	if err != nil {
		return nil, fmt.Errorf("reading sysfs USB devices: %w", err)
	}

	type devEntry struct {
		sysfsName string
		device    *USBDevice
		busNum    int
	}

	busMap := make(map[int]*USBBus)     // bus number → USBBus
	devMap := make(map[string]*devEntry) // sysfs name → parsed device

	// First pass: identify root hubs and create buses.
	for _, e := range entries {
		name := e.Name()
		if m := rootHubRe.FindStringSubmatch(name); m != nil {
			busNum, _ := strconv.Atoi(m[1])
			path := filepath.Join(sysfsUSBPath, name)
			product := readSysfsAttr(path, "product")
			if product == "" {
				product = fmt.Sprintf("USB Bus %d", busNum)
			}
			busMap[busNum] = &USBBus{
				Name:         product,
				HardwareType: "Built-in",
				LocationID:   synthesizeLocationID(busNum, ""),
			}
		}
	}

	// Second pass: parse all non-root-hub device entries.
	for _, e := range entries {
		name := e.Name()
		if !deviceRe.MatchString(name) {
			continue
		}

		path := filepath.Join(sysfsUSBPath, name)
		busNum := parseBusNum(name)

		// Ensure bus exists (shouldn't happen, but be safe).
		if _, ok := busMap[busNum]; !ok {
			busMap[busNum] = &USBBus{
				Name:         fmt.Sprintf("USB Bus %d", busNum),
				HardwareType: "Built-in",
				LocationID:   synthesizeLocationID(busNum, ""),
			}
		}

		devName := readSysfsAttr(path, "product")
		vendorID := readSysfsAttr(path, "idVendor")
		productID := readSysfsAttr(path, "idProduct")
		if devName == "" && vendorID != "" && productID != "" {
			devName = vendorID + ":" + productID
		}

		removable := readSysfsAttr(path, "removable")
		var hwType string
		switch removable {
		case "removable":
			hwType = "Removable"
		case "fixed":
			hwType = "Non-removable"
		}

		dev := &USBDevice{
			Name:         devName,
			VendorName:   readSysfsAttr(path, "manufacturer"),
			VendorID:     prefixHex(vendorID),
			ProductID:    prefixHex(productID),
			LinkSpeed:    parseSpeedToLinkSpeed(readSysfsAttr(path, "speed")),
			HardwareType: hwType,
			LocationID:   synthesizeLocationID(busNum, portPath(name)),
			SerialNumber: readSysfsAttr(path, "serial"),
		}

		// Two-stage classification: USB class code first, then keyword fallback.
		devClass := readSysfsAttr(path, "bDeviceClass")
		dev.Type = classifyByDeviceClass(devClass)
		if dev.Type == DeviceTypeUnknown {
			dev.Type = classifyDevice(dev.Name, dev.VendorName)
		}

		devMap[name] = &devEntry{
			sysfsName: name,
			device:    dev,
			busNum:    busNum,
		}
	}

	// Third pass: build parent→child relationships.
	topLevel := make(map[int][]*USBDevice) // bus number → direct children
	for name, de := range devMap {
		parent := parentSysfsName(name)
		if parent == "" {
			// Direct child of a bus.
			topLevel[de.busNum] = append(topLevel[de.busNum], de.device)
		} else if pe, ok := devMap[parent]; ok {
			pe.device.Children = append(pe.device.Children, de.device)
		} else {
			// Orphan — attach to bus directly.
			slog.Debug("usb: orphan device, attaching to bus", "device", name, "bus", de.busNum)
			topLevel[de.busNum] = append(topLevel[de.busNum], de.device)
		}
	}

	// Assemble buses.
	var buses []USBBus
	for busNum, bus := range busMap {
		bus.Devices = topLevel[busNum]
		buses = append(buses, *bus)
	}
	return buses, nil
}

func prefixHex(s string) string {
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "0x") {
		return s
	}
	return "0x" + s
}

// lsblk JSON structures.
type lsblkOutput struct {
	Blockdevices []lsblkDevice `json:"blockdevices"`
}

type lsblkDevice struct {
	Name       string         `json:"name"`
	Size       json.Number    `json:"size"`
	FsUsed     *json.Number   `json:"fsused"`
	MountPoint *string        `json:"mountpoint"`
	Tran       *string        `json:"tran"`
	Serial     *string        `json:"serial"`
	Vendor     *string        `json:"vendor"`
	Model      *string        `json:"model"`
	Children   []lsblkDevice  `json:"children"`
}

// parseLsblkVolumes runs lsblk and returns USB storage volumes keyed by serial number.
func parseLsblkVolumes() map[string][]VolumeInfo {
	out, err := exec.Command("lsblk", "-J", "-b", "-o", "NAME,SIZE,FSUSED,MOUNTPOINT,TRAN,SERIAL,VENDOR,MODEL").Output()
	if err != nil {
		slog.Debug("lsblk unavailable, skipping volume info", "error", err)
		return nil
	}

	var parsed lsblkOutput
	if err := json.Unmarshal(out, &parsed); err != nil {
		slog.Debug("lsblk parse error", "error", err)
		return nil
	}

	result := make(map[string][]VolumeInfo)
	for _, dev := range parsed.Blockdevices {
		if dev.Tran == nil || *dev.Tran != "usb" {
			continue
		}
		serial := ""
		if dev.Serial != nil {
			serial = strings.TrimSpace(*dev.Serial)
		}
		if serial == "" {
			// Fall back to model name as key.
			if dev.Model != nil {
				serial = "model:" + strings.TrimSpace(*dev.Model)
			} else {
				continue
			}
		}

		// Collect mounted partitions from children.
		for _, part := range dev.Children {
			if part.MountPoint == nil || *part.MountPoint == "" {
				continue
			}
			totalBytes, _ := part.Size.Int64()
			var usedBytes int64
			if part.FsUsed != nil {
				usedBytes, _ = part.FsUsed.Int64()
			}
			result[serial] = append(result[serial], VolumeInfo{
				VolumeName: part.Name,
				TotalBytes: totalBytes,
				UsedBytes:  usedBytes,
				MountPoint: *part.MountPoint,
			})
		}

		// If no children but the device itself is mounted, use it directly.
		if len(dev.Children) == 0 && dev.MountPoint != nil && *dev.MountPoint != "" {
			totalBytes, _ := dev.Size.Int64()
			var usedBytes int64
			if dev.FsUsed != nil {
				usedBytes, _ = dev.FsUsed.Int64()
			}
			result[serial] = append(result[serial], VolumeInfo{
				VolumeName: dev.Name,
				TotalBytes: totalBytes,
				UsedBytes:  usedBytes,
				MountPoint: *dev.MountPoint,
			})
		}
	}
	return result
}

// correlateLinuxVolumes attaches volume info to matching USB devices.
func correlateLinuxVolumes(buses []USBBus, volumesByKey map[string][]VolumeInfo) {
	for i := range buses {
		for _, dev := range buses[i].Devices {
			attachLinuxVolumes(dev, volumesByKey)
		}
	}
}

func attachLinuxVolumes(dev *USBDevice, volumesByKey map[string][]VolumeInfo) {
	// Try matching by serial number.
	serial := strings.TrimSpace(dev.SerialNumber)
	if serial != "" {
		if vols, ok := volumesByKey[serial]; ok {
			dev.HasVolume = true
			dev.Volumes = append(dev.Volumes, vols...)
		}
	}

	// Try matching by model/product name.
	name := strings.TrimSpace(dev.Name)
	if !dev.HasVolume && name != "" {
		modelKey := "model:" + name
		if vols, ok := volumesByKey[modelKey]; ok {
			dev.HasVolume = true
			dev.Volumes = append(dev.Volumes, vols...)
		}
	}

	for _, child := range dev.Children {
		attachLinuxVolumes(child, volumesByKey)
	}
}
