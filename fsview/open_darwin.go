//go:build darwin

package fsview

import (
	"os/exec"

	tea "charm.land/bubbletea/v2"
)

func openFile(path string) tea.Cmd {
	return func() tea.Msg {
		err := exec.Command("open", path).Start()
		return openFileDoneMsg{path: path, err: err}
	}
}
