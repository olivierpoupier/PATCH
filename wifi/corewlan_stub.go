//go:build !darwin

package wifi

// WLANClient is a no-op stub for non-macOS platforms.
type WLANClient struct{}

// NewWLANClient returns a stub client.
func NewWLANClient() *WLANClient { return &WLANClient{} }

// LocationAuthorized always returns false on non-macOS.
func (w *WLANClient) LocationAuthorized() bool { return false }

// InterfaceInfo returns empty interface info on non-macOS.
func (w *WLANClient) InterfaceInfo() (InterfaceInfo, error) {
	return InterfaceInfo{}, nil
}

// ConnectionInfo returns empty connection info on non-macOS.
func (w *WLANClient) ConnectionInfo() (ConnectionInfo, error) {
	return ConnectionInfo{}, nil
}

// ScanNetworks returns nil on non-macOS.
func (w *WLANClient) ScanNetworks() ([]NetworkInfo, error) {
	return nil, nil
}
