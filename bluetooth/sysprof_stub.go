//go:build !darwin

package bluetooth

// SystemProfilerScanner is a no-op on non-darwin platforms.
type SystemProfilerScanner struct{}

// NewSystemProfilerScanner returns a no-op scanner.
func NewSystemProfilerScanner() *SystemProfilerScanner { return &SystemProfilerScanner{} }

// Refresh is a no-op on non-darwin platforms.
func (s *SystemProfilerScanner) Refresh() error { return nil }

// Devices returns nil on non-darwin platforms.
func (s *SystemProfilerScanner) Devices() []DeviceInfo { return nil }
