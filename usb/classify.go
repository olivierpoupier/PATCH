package usb

import "strings"

// DeviceType classifies a USB device for display.
type DeviceType int

const (
	DeviceTypeUnknown DeviceType = iota
	DeviceTypeHub
	DeviceTypeKeyboard
	DeviceTypeMouse
	DeviceTypeStorage
	DeviceTypeAudio
	DeviceTypeVideo
	DeviceTypeNetwork
	DeviceTypePrinter
	DeviceTypePhone
	DeviceTypeDock
	DeviceTypeBillboard
)

// String returns a display label for the device type.
func (dt DeviceType) String() string {
	switch dt {
	case DeviceTypeHub:
		return "Hub"
	case DeviceTypeKeyboard:
		return "Keyboard"
	case DeviceTypeMouse:
		return "Mouse"
	case DeviceTypeStorage:
		return "Storage"
	case DeviceTypeAudio:
		return "Audio"
	case DeviceTypeVideo:
		return "Video"
	case DeviceTypeNetwork:
		return "Network"
	case DeviceTypePrinter:
		return "Printer"
	case DeviceTypePhone:
		return "Phone"
	case DeviceTypeDock:
		return "Dock"
	case DeviceTypeBillboard:
		return "Billboard"
	default:
		return "Device"
	}
}

// classifyDevice determines a device type from its name and vendor using keyword heuristics.
func classifyDevice(name, vendor string) DeviceType {
	combined := strings.ToLower(name + " " + vendor)

	if strings.Contains(combined, "hub") {
		return DeviceTypeHub
	}
	if strings.Contains(combined, "billboard") {
		return DeviceTypeBillboard
	}
	if containsAny(combined, "dock", "40ay") {
		return DeviceTypeDock
	}
	if containsAny(combined, "keyboard", "kbd", "adv360", "kinesis") {
		return DeviceTypeKeyboard
	}
	if containsAny(combined, "mouse", "trackpad", "trackball", "pointing") {
		return DeviceTypeMouse
	}
	if containsAny(combined, "storage", "disk", "flash", "thumb", "ssd", "hdd", "card reader") {
		return DeviceTypeStorage
	}
	if containsAny(combined, "audio", "headset", "microphone", "speaker", "dac") {
		return DeviceTypeAudio
	}
	if containsAny(combined, "camera", "webcam", "video", "capture") {
		return DeviceTypeVideo
	}
	if containsAny(combined, "ethernet", "lan", "wifi", "wireless adapter", "network") {
		return DeviceTypeNetwork
	}
	if containsAny(combined, "printer", "scanner") {
		return DeviceTypePrinter
	}
	if containsAny(combined, "iphone", "ipad", "android", "phone") {
		return DeviceTypePhone
	}

	return DeviceTypeUnknown
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
