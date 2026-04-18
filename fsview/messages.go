package fsview

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/olivierpoupier/patch/tui"

	tea "charm.land/bubbletea/v2"
)

type entry struct {
	name      string
	size      int64
	mode      os.FileMode
	modTime   time.Time
	isDir     bool
	isSymlink bool
	target    string
}

type dirLoadedMsg struct {
	path    string
	entries []entry
	err     error
}

type openFileDoneMsg struct {
	path string
	err  error
}

func loadDirCmd(path string, showHidden bool) tea.Cmd {
	return func() tea.Msg {
		raw, err := os.ReadDir(path)
		if err != nil {
			return dirLoadedMsg{path: path, err: fmt.Errorf("read %s: %w", path, err)}
		}
		out := make([]entry, 0, len(raw))
		for _, de := range raw {
			info, ierr := de.Info()
			if ierr != nil {
				continue
			}
			e := entry{
				name:      de.Name(),
				size:      info.Size(),
				mode:      info.Mode(),
				modTime:   info.ModTime(),
				isDir:     de.IsDir(),
				isSymlink: info.Mode()&os.ModeSymlink != 0,
			}
			if e.isSymlink {
				if tgt, terr := os.Readlink(filepath.Join(path, e.name)); terr == nil {
					e.target = tgt
				}
			}
			out = append(out, e)
		}
		return dirLoadedMsg{path: path, entries: sortEntries(out, showHidden)}
	}
}

func requestCloseCmd() tea.Cmd {
	return func() tea.Msg { return tui.CloseFSViewMsg{} }
}
