package fsview

import (
	"github.com/olivierpoupier/patch/tui"
	"github.com/olivierpoupier/patch/tui/components"

	tea "charm.land/bubbletea/v2"
)

type rootEntry struct {
	label      string
	mountPoint string
	totalBytes int64
	usedBytes  int64
}

// Model holds the file system browser state.
type Model struct {
	theme *tui.Theme

	rootEntries []rootEntry
	rootCursor  int

	stack   []string
	entries []entry
	cursor  int
	loading bool
	err     error

	showHidden bool
	width      int
	height     int

	scroll components.ScrollableView

	status string
}

// New creates an empty file system view model.
func New(theme *tui.Theme) *Model {
	return &Model{
		theme:  theme,
		scroll: components.NewScrollableView(theme),
	}
}

// Open activates the view for one or more mounted volumes. Returns the cmd
// that kicks off the initial directory load (nil when showing the synthetic
// root).
func (m *Model) Open(entries []tui.VolumeEntry) tea.Cmd {
	m.rootEntries = m.rootEntries[:0]
	m.rootCursor = 0
	m.stack = m.stack[:0]
	m.entries = nil
	m.cursor = 0
	m.loading = false
	m.err = nil
	m.status = ""

	if len(entries) == 0 {
		return nil
	}
	if len(entries) == 1 {
		m.stack = append(m.stack, entries[0].MountPoint)
		m.loading = true
		return loadDirCmd(entries[0].MountPoint, m.showHidden)
	}
	for _, e := range entries {
		m.rootEntries = append(m.rootEntries, rootEntry{
			label:      e.Label,
			mountPoint: e.MountPoint,
			totalBytes: e.TotalBytes,
			usedBytes:  e.UsedBytes,
		})
	}
	return nil
}

// Close resets state so the next Open starts fresh.
func (m *Model) Close() {
	m.rootEntries = nil
	m.rootCursor = 0
	m.stack = nil
	m.entries = nil
	m.cursor = 0
	m.loading = false
	m.err = nil
	m.status = ""
}

// Active reports whether the file system view should be rendered.
func (m *Model) Active() bool {
	return len(m.stack) > 0 || len(m.rootEntries) > 0
}

// SetSize caches the full-screen dimensions.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m *Model) atSyntheticRoot() bool {
	return len(m.stack) == 0 && len(m.rootEntries) > 0
}

func (m *Model) currentDir() string {
	if len(m.stack) == 0 {
		return ""
	}
	return m.stack[len(m.stack)-1]
}
