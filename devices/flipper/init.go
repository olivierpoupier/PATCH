package flipper

import (
	"github.com/olivierpoupier/patch/devices"
	"github.com/olivierpoupier/patch/tui"
)

func init() {
	devices.Register(devices.Descriptor{
		Key:   "flipper_zero",
		Name:  "Flipper Zero",
		Baud:  115200,
		Match: devices.MatchVIDPID("0483", "5740"),
		New: func(theme *tui.Theme) tui.DeviceView {
			return New(theme)
		},
	})
}
