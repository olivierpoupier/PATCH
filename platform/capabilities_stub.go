//go:build !darwin && !linux

package platform

// Detect returns the platform capabilities for unsupported platforms.
func Detect() Capabilities {
	return Capabilities{
		HasBluetooth: false,
		HasWiFi:      false,
		HasUSB:       false,
		HasStorage:   true,
		HasSSH:       true,
		OS:           "unsupported",
	}
}
