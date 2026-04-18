package tui

// Cross-tab message types for inter-tab communication.
// These are exported so any tab package can import and match on them.
// Tab-internal messages should remain unexported in their own packages.

// DeviceEventMsg is emitted by any tab when a device connects or disconnects.
// Other tabs can react (e.g., Storage tab reacts to USB storage connection).
type DeviceEventMsg struct {
	DeviceID   string
	DeviceName string
	Type       string // "usb", "bluetooth", "wifi"
	Event      string // "connected", "disconnected"
}

// NetworkStateMsg is emitted by the WiFi tab when network state changes.
// Other tabs (e.g., SSH) can react to network availability.
type NetworkStateMsg struct {
	Connected bool
	SSID      string
}

// VolumeEntry describes a mounted volume for the file system view, neutral
// to the source tab.
type VolumeEntry struct {
	Label      string
	MountPoint string
	TotalBytes int64
	UsedBytes  int64
}

// OpenFSViewMsg is emitted by any tab to request the app open the file
// system view. Multiple entries trigger a synthetic volume-picker root.
type OpenFSViewMsg struct {
	Entries []VolumeEntry
	Source  string
}

// CloseFSViewMsg is emitted by the file system view when it wants the app
// to tear down the overlay.
type CloseFSViewMsg struct{}
