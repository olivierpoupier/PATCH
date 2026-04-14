package wifi

import "fmt"

// SecurityType represents the WiFi security protocol.
type SecurityType int

const (
	SecurityNone SecurityType = iota
	SecurityWEP
	SecurityWPAPersonal
	SecurityWPA2Personal
	SecurityWPA3Personal
	SecurityWPAEnterprise
	SecurityWPA2Enterprise
	SecurityWPA3Enterprise
	SecurityUnknown
)

func (s SecurityType) String() string {
	switch s {
	case SecurityNone:
		return "Open"
	case SecurityWEP:
		return "WEP"
	case SecurityWPAPersonal:
		return "WPA"
	case SecurityWPA2Personal:
		return "WPA2"
	case SecurityWPA3Personal:
		return "WPA3"
	case SecurityWPAEnterprise:
		return "WPA Ent"
	case SecurityWPA2Enterprise:
		return "WPA2 Ent"
	case SecurityWPA3Enterprise:
		return "WPA3 Ent"
	default:
		return "Unknown"
	}
}

// PHYMode represents the 802.11 standard in use.
type PHYMode int

const (
	PHYModeNone PHYMode = iota
	PHYMode11a
	PHYMode11b
	PHYMode11g
	PHYMode11n
	PHYMode11ac
	PHYMode11ax
	PHYMode11be
)

func (p PHYMode) String() string {
	switch p {
	case PHYMode11a:
		return "802.11a"
	case PHYMode11b:
		return "802.11b"
	case PHYMode11g:
		return "802.11g"
	case PHYMode11n:
		return "802.11n"
	case PHYMode11ac:
		return "802.11ac"
	case PHYMode11ax:
		return "802.11ax"
	case PHYMode11be:
		return "802.11be"
	default:
		return "Unknown"
	}
}

// InterfaceInfo holds the state of the WiFi interface.
type InterfaceInfo struct {
	Name         string // e.g. "en0"
	HardwareAddr string // MAC address
	PowerOn      bool
}

// ConnectionInfo holds details about the current WiFi connection.
// Fields are zero-valued when not connected.
type ConnectionInfo struct {
	Connected bool
	SSID      string
	BSSID     string
	RSSI      int     // dBm, e.g. -45
	Noise     int     // dBm, e.g. -90
	Channel   int
	Band      string  // "2.4 GHz", "5 GHz", "6 GHz"
	Security  SecurityType
	TXRate    float64 // Mbps
	PHYMode   PHYMode
}

// NetworkInfo represents a single scanned WiFi network.
type NetworkInfo struct {
	SSID     string
	BSSID    string
	RSSI     int // dBm
	Channel  int
	Band     string // "2.4 GHz", "5 GHz", "6 GHz"
	Security SecurityType
}

// signalBars returns a visual signal strength indicator.
func signalBars(rssi int) string {
	switch {
	case rssi >= -50:
		return "▂▄▆█"
	case rssi >= -60:
		return "▂▄▆_"
	case rssi >= -70:
		return "▂▄__"
	case rssi >= -80:
		return "▂___"
	default:
		return "____"
	}
}

// signalString returns signal bars with dBm value.
func signalString(rssi int) string {
	return fmt.Sprintf("%s %d", signalBars(rssi), rssi)
}
