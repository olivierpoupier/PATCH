//go:build linux

package platform

// Detect returns the platform capabilities for Linux.
func Detect() Capabilities {
	return Capabilities{
		HasBluetooth: true,
		HasWiFi:      true,
		HasUSB:       true,
		HasStorage:   true,
		HasSSH:       true,
		OS:           "linux",
	}
}
