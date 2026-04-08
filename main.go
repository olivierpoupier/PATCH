package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/olivierpoupier/patch/bluetooth"
	"github.com/olivierpoupier/patch/logging"
	"github.com/olivierpoupier/patch/tui"
	"github.com/olivierpoupier/patch/usb"

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

	m := newModel([]tui.Tab{
		usb.New(),
		bluetooth.New(),
	})
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		slog.Error("program exited with error", "error", err)
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
