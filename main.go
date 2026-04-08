package main

import (
	"fmt"
	"os"

	"github.com/olivierpoupier/patch/bluetooth"
	"github.com/olivierpoupier/patch/tui"
	"github.com/olivierpoupier/patch/usb"

	tea "charm.land/bubbletea/v2"
)

func main() {
	m := newModel([]tui.Tab{
		usb.New(),
		bluetooth.New(),
	})
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
