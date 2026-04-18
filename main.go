package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/olivierpoupier/patch/bluetooth"
	"github.com/olivierpoupier/patch/logging"
	"github.com/olivierpoupier/patch/platform"
	"github.com/olivierpoupier/patch/tui"
	"github.com/olivierpoupier/patch/usb"
	"github.com/olivierpoupier/patch/wifi"

	tea "charm.land/bubbletea/v2"
)

func main() {
	cleanup, err := logging.Setup("patch.log")
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to set up logging:", err)
		os.Exit(1)
	}
	defer cleanup()

	slog.Info("starting patch")

	theme := tui.NewTheme()
	caps := platform.Detect()

	var tabs []tui.Tab
	if caps.HasUSB {
		tabs = append(tabs, usb.New(theme))
	}
	if caps.HasBluetooth {
		tabs = append(tabs, bluetooth.New(theme))
	}
	if caps.HasWiFi {
		tabs = append(tabs, wifi.New(theme))
	}


	m := newModel(theme, tabs)

	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		slog.Error("program exited with error", "error", err)
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
