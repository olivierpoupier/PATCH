package fsview

import (
	"path/filepath"

	tea "charm.land/bubbletea/v2"
)

// Update handles messages for the file system view.
func (m *Model) Update(msg tea.Msg) (*Model, tea.Cmd) {
	switch msg := msg.(type) {
	case dirLoadedMsg:
		return m.handleDirLoaded(msg), nil
	case openFileDoneMsg:
		return m.handleOpenFileDone(msg), nil
	case tea.KeyPressMsg:
		return m.handleKeyPress(msg)
	}
	return m, nil
}

func (m *Model) handleDirLoaded(msg dirLoadedMsg) *Model {
	m.loading = false
	if msg.err != nil {
		m.err = msg.err
		return m
	}
	m.err = nil
	m.entries = msg.entries
	if m.cursor >= len(m.entries) {
		m.cursor = 0
	}
	return m
}

func (m *Model) handleOpenFileDone(msg openFileDoneMsg) *Model {
	if msg.err != nil {
		m.status = "error: " + msg.err.Error()
		return m
	}
	m.status = "opened " + msg.path
	return m
}

func (m *Model) handleKeyPress(k tea.KeyPressMsg) (*Model, tea.Cmd) {
	m.status = ""

	switch k.String() {
	case "esc":
		return m, requestCloseCmd()

	case "up", "k":
		m.cursorUp()
		return m, nil

	case "down", "j":
		m.cursorDown()
		return m, nil

	case "g":
		m.setCursor(0)
		return m, nil

	case "G":
		m.setCursor(m.cursorMax())
		return m, nil

	case "enter":
		return m.activateSelection()

	case "backspace", "left", "h":
		return m.goUp()

	case ".":
		if m.atSyntheticRoot() {
			return m, nil
		}
		m.showHidden = !m.showHidden
		if dir := m.currentDir(); dir != "" {
			m.loading = true
			return m, loadDirCmd(dir, m.showHidden)
		}
		return m, nil

	case "c":
		if path := m.currentSelectedPath(); path != "" {
			m.status = "copied " + path
			return m, tea.SetClipboard(path)
		}
		return m, nil

	case "o":
		if path := m.currentSelectedPath(); path != "" {
			return m, openFile(path)
		}
		return m, nil
	}
	return m, nil
}

func (m *Model) activateSelection() (*Model, tea.Cmd) {
	if m.atSyntheticRoot() {
		if m.rootCursor < 0 || m.rootCursor >= len(m.rootEntries) {
			return m, nil
		}
		target := m.rootEntries[m.rootCursor].mountPoint
		m.stack = append(m.stack, target)
		m.cursor = 0
		m.loading = true
		return m, loadDirCmd(target, m.showHidden)
	}
	if m.cursor < 0 || m.cursor >= len(m.entries) {
		return m, nil
	}
	e := m.entries[m.cursor]
	path := filepath.Join(m.currentDir(), e.name)
	if e.isDir {
		m.stack = append(m.stack, path)
		m.cursor = 0
		m.loading = true
		return m, loadDirCmd(path, m.showHidden)
	}
	return m, openFile(path)
}

func (m *Model) goUp() (*Model, tea.Cmd) {
	if m.atSyntheticRoot() {
		return m, nil
	}
	if len(m.stack) == 0 {
		return m, nil
	}
	if len(m.stack) == 1 {
		if len(m.rootEntries) > 0 {
			m.stack = m.stack[:0]
			m.entries = nil
			m.cursor = 0
			m.err = nil
			return m, nil
		}
		return m, nil
	}
	m.stack = m.stack[:len(m.stack)-1]
	m.cursor = 0
	m.loading = true
	return m, loadDirCmd(m.currentDir(), m.showHidden)
}

func (m *Model) cursorUp() {
	if m.atSyntheticRoot() {
		if m.rootCursor > 0 {
			m.rootCursor--
		}
		return
	}
	if m.cursor > 0 {
		m.cursor--
	}
}

func (m *Model) cursorDown() {
	if m.atSyntheticRoot() {
		if m.rootCursor < len(m.rootEntries)-1 {
			m.rootCursor++
		}
		return
	}
	if m.cursor < len(m.entries)-1 {
		m.cursor++
	}
}

func (m *Model) setCursor(i int) {
	if m.atSyntheticRoot() {
		m.rootCursor = clamp(i, 0, len(m.rootEntries)-1)
		return
	}
	m.cursor = clamp(i, 0, len(m.entries)-1)
}

func (m *Model) cursorMax() int {
	if m.atSyntheticRoot() {
		return len(m.rootEntries) - 1
	}
	return len(m.entries) - 1
}

func (m *Model) currentSelectedPath() string {
	if m.atSyntheticRoot() {
		if m.rootCursor < 0 || m.rootCursor >= len(m.rootEntries) {
			return ""
		}
		return m.rootEntries[m.rootCursor].mountPoint
	}
	if m.cursor < 0 || m.cursor >= len(m.entries) {
		return m.currentDir()
	}
	return filepath.Join(m.currentDir(), m.entries[m.cursor].name)
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
