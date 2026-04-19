package generic

import (
	"github.com/olivierpoupier/patch/devices"
	"github.com/olivierpoupier/patch/tui"
)

// init registers matchers for common USB-to-serial bridge chips and common
// device families that don't (yet) have their own custom view. All funnel
// into the generic view. A nil-Match descriptor is registered last as the
// fallback-of-last-resort so any unrecognised USB-CDC device still opens.
func init() {
	entries := []struct {
		key  string
		vid  string
		pid  string
		name string
		baud int
	}{
		{"esp32", "303a", "", "ESP32", 115200},
		{"arduino", "2341", "", "Arduino", 115200},
		{"pluto_sdr", "0456", "b673", "Pluto SDR", 115200},
		{"ftdi", "0403", "", "FTDI USB-Serial", 115200},
		{"ch340", "1a86", "7523", "CH340 USB-Serial", 115200},
		{"cp2102", "10c4", "ea60", "CP2102 USB-Serial", 115200},
	}
	for _, e := range entries {
		devices.Register(devices.Descriptor{
			Key:   e.key,
			Name:  e.name,
			Baud:  e.baud,
			Match: devices.MatchVIDPID(e.vid, e.pid),
			New:   factory,
		})
	}
	devices.Register(devices.Descriptor{
		Key:   "generic",
		Name:  "USB Serial Device",
		Baud:  115200,
		Match: nil, // fallback-of-last-resort
		New:   factory,
	})
}

func factory(theme *tui.Theme) tui.DeviceView {
	return New(theme)
}
