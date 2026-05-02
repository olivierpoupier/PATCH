//go:build linux

package wifi

import (
	"errors"
	"fmt"
	"sync"

	gnm "github.com/Wifx/gonetworkmanager"
	"github.com/google/uuid"
)

// WLANClient wraps NetworkManager D-Bus access for the primary wireless device.
// All methods are safe for concurrent use.
type WLANClient struct {
	mu       sync.Mutex
	nm       gnm.NetworkManager
	settings gnm.Settings
	device   gnm.DeviceWireless
	ifname   string
	hwAddr   string
	initErr  error
}

// NewWLANClient connects to the NetworkManager D-Bus service and locates the
// primary wireless device. If NetworkManager is unavailable or no wireless
// device is present, errors are deferred until the first method call so the
// tab can still render an explanatory message.
func NewWLANClient() *WLANClient {
	c := &WLANClient{}

	nm, err := gnm.NewNetworkManager()
	if err != nil {
		c.initErr = fmt.Errorf("NetworkManager unavailable: %w", err)
		return c
	}
	c.nm = nm

	settings, err := gnm.NewSettings()
	if err != nil {
		c.initErr = fmt.Errorf("NetworkManager Settings unavailable: %w", err)
		return c
	}
	c.settings = settings

	devices, err := nm.GetPropertyAllDevices()
	if err != nil {
		c.initErr = fmt.Errorf("list NetworkManager devices: %w", err)
		return c
	}
	for _, d := range devices {
		dt, err := d.GetPropertyDeviceType()
		if err != nil || dt != gnm.NmDeviceTypeWifi {
			continue
		}
		// GetPropertyAllDevices returns plain *device values; wrap with
		// NewDeviceWireless to get the wireless-specific method set.
		dw, err := gnm.NewDeviceWireless(d.GetPath())
		if err != nil {
			c.initErr = fmt.Errorf("open wireless device: %w", err)
			return c
		}
		c.device = dw
		if name, err := d.GetPropertyInterface(); err == nil {
			c.ifname = name
		}
		if mac, err := dw.GetPropertyHwAddress(); err == nil {
			c.hwAddr = mac
		}
		break
	}
	if c.device == nil {
		c.initErr = errors.New("no wireless device found")
	}
	return c
}

// LocationAuthorized is a macOS-only concept; on Linux it is always true so
// the SSID-visibility warning in the tab UI stays hidden.
func (w *WLANClient) LocationAuthorized() bool { return true }

// InterfaceInfo returns the wireless interface name, MAC, and the global
// WirelessEnabled flag from NetworkManager.
func (w *WLANClient) InterfaceInfo() (InterfaceInfo, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.initErr != nil {
		return InterfaceInfo{}, w.initErr
	}

	on, err := w.nm.GetPropertyWirelessEnabled()
	if err != nil {
		return InterfaceInfo{}, fmt.Errorf("read WirelessEnabled: %w", err)
	}
	return InterfaceInfo{
		Name:         w.ifname,
		HardwareAddr: w.hwAddr,
		PowerOn:      on,
	}, nil
}

// ConnectionInfo describes the active access point on the wireless device.
// Noise and PHYMode are not exposed by NetworkManager and are returned as
// zero values; the tab renderer treats those as "not available".
func (w *WLANClient) ConnectionInfo() (ConnectionInfo, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.initErr != nil {
		return ConnectionInfo{}, w.initErr
	}

	state, err := w.device.GetPropertyState()
	if err != nil {
		return ConnectionInfo{}, fmt.Errorf("read device state: %w", err)
	}
	if state != gnm.NmDeviceStateActivated {
		return ConnectionInfo{}, nil
	}

	ap, err := w.device.GetPropertyActiveAccessPoint()
	if err != nil || ap == nil {
		return ConnectionInfo{}, nil
	}

	ssid, _ := ap.GetPropertySSID()
	bssid, _ := ap.GetPropertyHWAddress()
	freq, _ := ap.GetPropertyFrequency()
	strength, _ := ap.GetPropertyStrength()
	flags, _ := ap.GetPropertyFlags()
	wpaFlags, _ := ap.GetPropertyWPAFlags()
	rsnFlags, _ := ap.GetPropertyRSNFlags()
	bitrateKbps, _ := w.device.GetPropertyBitrate()

	channel, band := frequencyToChannel(freq)

	return ConnectionInfo{
		Connected: true,
		SSID:      ssid,
		BSSID:     bssid,
		RSSI:      strengthToRSSI(strength),
		Channel:   channel,
		Band:      band,
		Security:  mapAPSecurity(flags, wpaFlags, rsnFlags),
		TXRate:    float64(bitrateKbps) / 1000.0,
		PHYMode:   PHYModeNone,
	}, nil
}

// ScanNetworks returns the access points currently known to NetworkManager
// and asynchronously triggers a fresh scan. Results from the new scan land on
// the next poll tick; this avoids blocking the UI while NM rescans.
func (w *WLANClient) ScanNetworks() ([]NetworkInfo, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.initErr != nil {
		return nil, w.initErr
	}

	// Best-effort: ignore errors here (a scan may already be in progress, or
	// the device may be busy). The cached AP list is returned regardless.
	_ = w.device.RequestScan()

	aps, err := w.device.GetAccessPoints()
	if err != nil {
		return nil, fmt.Errorf("list access points: %w", err)
	}

	out := make([]NetworkInfo, 0, len(aps))
	for _, ap := range aps {
		ssid, _ := ap.GetPropertySSID()
		bssid, _ := ap.GetPropertyHWAddress()
		freq, _ := ap.GetPropertyFrequency()
		strength, _ := ap.GetPropertyStrength()
		flags, _ := ap.GetPropertyFlags()
		wpaFlags, _ := ap.GetPropertyWPAFlags()
		rsnFlags, _ := ap.GetPropertyRSNFlags()

		channel, band := frequencyToChannel(freq)
		out = append(out, NetworkInfo{
			SSID:     ssid,
			BSSID:    bssid,
			RSSI:     strengthToRSSI(strength),
			Channel:  channel,
			Band:     band,
			Security: mapAPSecurity(flags, wpaFlags, rsnFlags),
		})
	}
	return out, nil
}

// SetPower toggles the global WirelessEnabled property on NetworkManager.
func (w *WLANClient) SetPower(on bool) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.initErr != nil {
		return w.initErr
	}
	return w.nm.SetPropertyWirelessEnabled(on)
}

// Connect activates a wireless connection for the given SSID. If a saved
// profile already matches the SSID, that profile is activated; otherwise a
// new connection is created with the supplied password and activated.
func (w *WLANClient) Connect(ssid, password string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.initErr != nil {
		return w.initErr
	}
	if ssid == "" {
		return errors.New("empty SSID")
	}

	// Find the access point object for this SSID so NM associates with the
	// strongest BSSID rather than guessing.
	var targetAP gnm.AccessPoint
	if aps, err := w.device.GetAccessPoints(); err == nil {
		for _, ap := range aps {
			if got, _ := ap.GetPropertySSID(); got == ssid {
				targetAP = ap
				break
			}
		}
	}

	// Look for an existing saved profile with this SSID.
	if existing := w.findConnectionBySSID(ssid); existing != nil {
		if targetAP != nil {
			_, err := w.nm.ActivateWirelessConnection(existing, w.device, targetAP)
			return err
		}
		_, err := w.nm.ActivateConnection(existing, w.device, nil)
		return err
	}

	connSettings := buildWirelessConnection(ssid, password)
	if targetAP != nil {
		_, err := w.nm.AddAndActivateWirelessConnection(connSettings, w.device, targetAP)
		return err
	}
	_, err := w.nm.AddAndActivateConnection(connSettings, w.device)
	return err
}

// Disconnect calls Disconnect on the wireless device, which deactivates the
// current connection without removing the saved profile.
func (w *WLANClient) Disconnect() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.initErr != nil || w.device == nil {
		return
	}
	_ = w.device.Disconnect()
}

func (w *WLANClient) findConnectionBySSID(ssid string) gnm.Connection {
	conns, err := w.settings.ListConnections()
	if err != nil {
		return nil
	}
	for _, c := range conns {
		s, err := c.GetSettings()
		if err != nil {
			continue
		}
		wireless, ok := s["802-11-wireless"]
		if !ok {
			continue
		}
		// SSID is stored as []byte in the connection settings.
		raw, ok := wireless["ssid"]
		if !ok {
			continue
		}
		switch v := raw.(type) {
		case []byte:
			if string(v) == ssid {
				return c
			}
		case string:
			if v == ssid {
				return c
			}
		}
	}
	return nil
}

func buildWirelessConnection(ssid, password string) gnm.ConnectionSettings {
	id, _ := uuid.NewUUID()
	cs := gnm.ConnectionSettings{
		"connection": map[string]interface{}{
			"id":   ssid,
			"type": "802-11-wireless",
			"uuid": id.String(),
		},
		"802-11-wireless": map[string]interface{}{
			"ssid": []byte(ssid),
			"mode": "infrastructure",
		},
		"ipv4": map[string]interface{}{"method": "auto"},
		"ipv6": map[string]interface{}{"method": "auto"},
	}
	if password != "" {
		cs["802-11-wireless-security"] = map[string]interface{}{
			"key-mgmt": "wpa-psk",
			"psk":      password,
		}
	}
	return cs
}

// frequencyToChannel converts an AP frequency in MHz to a channel number and
// human-readable band label.
func frequencyToChannel(freqMHz uint32) (int, string) {
	switch {
	case freqMHz == 0:
		return 0, ""
	case freqMHz == 2484:
		return 14, "2.4 GHz"
	case freqMHz >= 2412 && freqMHz <= 2472:
		return int((freqMHz - 2407) / 5), "2.4 GHz"
	case freqMHz >= 5160 && freqMHz <= 5885:
		return int((freqMHz - 5000) / 5), "5 GHz"
	case freqMHz >= 5955 && freqMHz <= 7115:
		return int((freqMHz-5950)/5) + 1, "6 GHz"
	default:
		return 0, ""
	}
}

// strengthToRSSI maps NetworkManager's percent-quality reading onto a rough
// dBm value for display. NM does not expose raw RSSI; -100..-30 dBm is the
// conventional usable range.
func strengthToRSSI(strength uint8) int {
	if strength == 0 {
		return -100
	}
	if strength >= 100 {
		return -30
	}
	// Linear map: 0% -> -100 dBm, 100% -> -30 dBm.
	return int(strength)*70/100 - 100
}

// mapAPSecurity translates the AP/WPA/RSN flag bitmasks into the SecurityType
// values used by the rest of the WiFi tab.
func mapAPSecurity(flags, wpaFlags, rsnFlags uint32) SecurityType {
	hasPrivacy := flags&uint32(gnm.Nm80211APFlagsPrivacy) != 0

	if !hasPrivacy && wpaFlags == 0 && rsnFlags == 0 {
		return SecurityNone
	}

	is8021X := func(f uint32) bool {
		return f&uint32(gnm.Nm80211APSecKeyMgmt8021X) != 0
	}
	hasSAE := func(f uint32) bool {
		return f&uint32(gnm.Nm80211APSecKeyMgmtSAE) != 0
	}

	if rsnFlags != 0 {
		switch {
		case hasSAE(rsnFlags) && is8021X(rsnFlags):
			return SecurityWPA3Enterprise
		case hasSAE(rsnFlags):
			return SecurityWPA3Personal
		case is8021X(rsnFlags):
			return SecurityWPA2Enterprise
		default:
			return SecurityWPA2Personal
		}
	}
	if wpaFlags != 0 {
		if is8021X(wpaFlags) {
			return SecurityWPAEnterprise
		}
		return SecurityWPAPersonal
	}
	// Privacy bit but no WPA/RSN means legacy WEP (or proprietary equivalent).
	return SecurityWEP
}

