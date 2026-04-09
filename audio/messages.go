package audio

import (
	"log/slog"
	"time"

	tea "charm.land/bubbletea/v2"
)

// Message types for the audio tab (unexported for natural filtering).
type (
	initBackendMsg struct{ backend Backend }
	sourcesMsg     struct{ sources []Source }
	outputsMsg     struct{ outputs []Output }
	errorMsg       struct{ err error }
	refreshTickMsg struct{}
)

// RefreshInterval is the time between audio state refreshes.
const RefreshInterval = 2 * time.Second

func initBackendCmd() tea.Cmd {
	return func() tea.Msg {
		b := newPlatformBackend()
		if err := b.Init(); err != nil {
			slog.Error("audio backend init failed", "error", err)
			return errorMsg{err}
		}
		slog.Info("audio backend initialized")
		return initBackendMsg{backend: b}
	}
}

func fetchSources(b Backend) tea.Cmd {
	return func() tea.Msg {
		sources, err := b.ListSources()
		if err != nil {
			return errorMsg{err}
		}
		return sourcesMsg{sources}
	}
}

func fetchOutputs(b Backend) tea.Cmd {
	return func() tea.Msg {
		outputs, err := b.ListOutputs()
		if err != nil {
			return errorMsg{err}
		}
		return outputsMsg{outputs}
	}
}

func scheduleRefreshTick() tea.Cmd {
	return tea.Tick(RefreshInterval, func(time.Time) tea.Msg {
		return refreshTickMsg{}
	})
}

func setVolumeCmd(b Backend, id string, vol float32) tea.Cmd {
	return func() tea.Msg {
		if err := b.SetVolume(id, vol); err != nil {
			return errorMsg{err}
		}
		sources, err := b.ListSources()
		if err != nil {
			return errorMsg{err}
		}
		return sourcesMsg{sources}
	}
}

func togglePauseSourceCmd(b Backend, id string) tea.Cmd {
	return func() tea.Msg {
		if err := b.TogglePauseSource(id); err != nil {
			return errorMsg{err}
		}
		sources, err := b.ListSources()
		if err != nil {
			return errorMsg{err}
		}
		return sourcesMsg{sources}
	}
}

func toggleMuteCmd(b Backend, id string) tea.Cmd {
	return func() tea.Msg {
		if err := b.ToggleMute(id); err != nil {
			return errorMsg{err}
		}
		sources, err := b.ListSources()
		if err != nil {
			return errorMsg{err}
		}
		return sourcesMsg{sources}
	}
}

func pauseAllCmd(b Backend) tea.Cmd {
	return func() tea.Msg {
		if err := b.PauseAll(); err != nil {
			return errorMsg{err}
		}
		sources, err := b.ListSources()
		if err != nil {
			return errorMsg{err}
		}
		return sourcesMsg{sources}
	}
}

func toggleOutputCmd(b Backend, id string, active bool) tea.Cmd {
	return func() tea.Msg {
		if err := b.ToggleOutput(id, active); err != nil {
			return errorMsg{err}
		}
		outputs, err := b.ListOutputs()
		if err != nil {
			return errorMsg{err}
		}
		return outputsMsg{outputs}
	}
}
