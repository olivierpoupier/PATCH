//go:build darwin

package platform

// Detect returns the platform capabilities for macOS.
func Detect() Capabilities {
	return Capabilities{
		HasBluetooth: true,
		HasWiFi:      true,
		HasUSB:       true,
		HasStorage:   true,
		HasSSH:       true,
		OS:           "darwin",
	}
}
