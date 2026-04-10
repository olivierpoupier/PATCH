//go:build !darwin

package usb

// USBScanner is a no-op stub on non-Darwin platforms.
type USBScanner struct{}

// NewUSBScanner creates a new scanner.
func NewUSBScanner() *USBScanner { return &USBScanner{} }

// Refresh is a no-op on non-Darwin platforms.
func (s *USBScanner) Refresh() error { return nil }

// Data returns empty slices on non-Darwin platforms.
func (s *USBScanner) Data() ([]USBBus, []ThunderboltPort) { return nil, nil }
