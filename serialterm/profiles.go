package serialterm

import (
	"bufio"
	"strings"
)

// Profile keys are used by banners and header renderers to pick
// device-specific assets.
const (
	profileFlipperZero = "flipper_zero"
	profileESP32       = "esp32"
	profileArduino     = "arduino"
	profilePlutoSDR    = "pluto_sdr"
	profileFTDI        = "ftdi"
	profileCH340       = "ch340"
	profileCP2102      = "cp2102"
	profileGeneric     = "generic"
)

// Profile captures everything the terminal modal needs to specialise for a
// given family of USB-CDC devices.
type Profile struct {
	Key  string
	VID  string // lowercase hex, no 0x prefix. "" means wildcard.
	PID  string // "" matches any PID when VID is set.
	Name string
	Baud int
	// HeaderQueries are sent to the device on modal open; replies are parsed
	// by HeaderParser into labelled fields shown in the header.
	HeaderQueries []string
	HeaderParser  func([]byte) map[string]string
	// HeaderFields lists the keys (in order) to render from the parser output.
	HeaderFields []string
}

// profiles is the curated table. Lookup prefers exact (VID, PID) match, then
// VID-only wildcard match, then the generic profile.
var profiles = []Profile{
	{
		Key:           profileFlipperZero,
		VID:           "0483",
		PID:           "5740",
		Name:          "Flipper Zero",
		Baud:          115200,
		// device_info is a legacy alias that outputs underscore-joined keys
		// (firmware_version, hardware_ver, ...). Power info is only available
		// as the modern "info power" subcommand, which uses dot-joined keys
		// (charge.level, battery.voltage, ...). See furi_hal_power_info_get
		// in the flipperzero-firmware source.
		HeaderQueries: []string{"device_info\r\n", "info power\r\n"},
		HeaderParser:  parseFlipperKV,
		HeaderFields:  []string{"firmware_version", "hardware_ver", "charge.level"},
	},
	{
		Key:  profileESP32,
		VID:  "303a", // Espressif
		Name: "ESP32",
		Baud: 115200,
	},
	{
		Key:  profileArduino,
		VID:  "2341", // Arduino LLC
		Name: "Arduino",
		Baud: 115200,
	},
	{
		Key:  profilePlutoSDR,
		VID:  "0456",
		PID:  "b673",
		Name: "Pluto SDR",
		Baud: 115200,
	},
	{
		Key:  profileFTDI,
		VID:  "0403",
		Name: "FTDI USB-Serial",
		Baud: 115200,
	},
	{
		Key:  profileCH340,
		VID:  "1a86",
		PID:  "7523",
		Name: "CH340 USB-Serial",
		Baud: 115200,
	},
	{
		Key:  profileCP2102,
		VID:  "10c4",
		PID:  "ea60",
		Name: "CP2102 USB-Serial",
		Baud: 115200,
	},
}

var genericProfile = Profile{
	Key:  profileGeneric,
	Name: "USB Serial Device",
	Baud: 115200,
}

// LookupProfile returns the best-matching profile for a VID/PID pair. VID and
// PID should be lowercase hex strings with or without a leading "0x".
func LookupProfile(vid, pid string) Profile {
	vid = normalizeHex(vid)
	pid = normalizeHex(pid)

	// Exact match first.
	for _, p := range profiles {
		if p.VID == vid && p.PID == pid {
			return p
		}
	}
	// VID-only wildcard match.
	for _, p := range profiles {
		if p.VID == vid && p.PID == "" {
			return p
		}
	}
	return genericProfile
}

// HasProfile reports whether a non-generic profile exists for the given
// VID/PID pair. Consumers use this to flag devices that support the serial
// terminal modal (e.g. the USB table's Action column).
func HasProfile(vid, pid string) bool {
	return LookupProfile(vid, pid).Key != profileGeneric
}

func normalizeHex(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.TrimPrefix(s, "0x")
	return s
}

// parseFlipperKV parses Flipper CLI "key : value" lines, tolerant of the
// prompt characters and padding the firmware uses ("hardware_ver       : 13").
func parseFlipperKV(raw []byte) map[string]string {
	out := make(map[string]string)
	sc := bufio.NewScanner(strings.NewReader(string(raw)))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, ">:") {
			continue
		}
		idx := strings.Index(line, ":")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		if key == "" || val == "" {
			continue
		}
		out[key] = val
	}
	return out
}
