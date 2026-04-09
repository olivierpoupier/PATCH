package audio

// Source represents an audio source (application or system).
type Source struct {
	ID                 string  // platform-specific identifier
	Name               string  // display name (e.g. app name or "System Audio")
	Volume             float32 // 0.0–1.0
	Muted              bool
	Playing            bool
	IsSystem           bool // true for system-wide volume entry (macOS)
	VolumeControllable bool // true if volume can be adjusted (false for per-app on macOS)
}

// Output represents an audio output device.
type Output struct {
	ID        string
	Name      string
	Active    bool // currently the default/selected output
	IsAirPlay bool
	Volume    float32
}

// Backend abstracts platform-specific audio operations.
type Backend interface {
	// Init connects to the audio subsystem.
	Init() error

	// Close tears down connections.
	Close() error

	// ListSources returns audio sources sorted playing-first.
	ListSources() ([]Source, error)

	// ListOutputs returns all output devices including AirPlay.
	ListOutputs() ([]Output, error)

	// SetVolume sets volume for a source by ID (0.0–1.0).
	SetVolume(sourceID string, volume float32) error

	// ToggleMute toggles mute for a source by ID.
	ToggleMute(sourceID string) error

	// TogglePauseSource pauses or resumes a specific source.
	// For system audio this toggles mute; for app sources it sends pause/play.
	TogglePauseSource(sourceID string) error

	// PauseAll mutes all sources.
	PauseAll() error

	// ToggleOutput activates or deactivates an output device.
	ToggleOutput(outputID string, active bool) error
}
