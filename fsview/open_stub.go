//go:build !darwin && !linux

package fsview

import (
	"errors"

	tea "charm.land/bubbletea/v2"
)

func openFile(path string) tea.Cmd {
	return func() tea.Msg {
		return openFileDoneMsg{path: path, err: errors.New("opening files not supported on this platform")}
	}
}
