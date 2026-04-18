package platform

// Capabilities describes what peripherals are available on the current platform.
// Used by main.go to decide which tabs to register.
type Capabilities struct {
	HasBluetooth bool
	HasWiFi      bool
	HasUSB       bool
	HasStorage   bool
	HasSSH       bool
	OS           string
}
