//go:build darwin

package audio

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/grandcat/zeroconf"
)

type darwinBackend struct {
	mu             sync.RWMutex
	cachedOutputs  []Output
	cachedSources  []Source
	airplayDevices []Output
}

func newPlatformBackend() Backend {
	return &darwinBackend{}
}

func (d *darwinBackend) Init() error {
	// Verify osascript is available.
	if _, err := exec.LookPath("osascript"); err != nil {
		return fmt.Errorf("osascript not found: %w", err)
	}
	return nil
}

func (d *darwinBackend) Close() error {
	return nil
}

func (d *darwinBackend) ListSources() ([]Source, error) {
	vol, muted, err := d.getSystemVolume()
	if err != nil {
		return nil, err
	}

	// System source is always first (used by the header bar in the UI).
	sources := []Source{
		{
			ID:                 "system",
			Name:               "System Audio",
			Volume:             vol,
			Muted:              muted,
			Playing:            !muted && vol > 0,
			IsSystem:           true,
			VolumeControllable: true,
		},
	}

	// Enumerate per-app audio sources via CoreAudio (macOS 14+).
	// Only include processes actively producing audio output.
	processes, err := coreAudioListProcesses()
	if err != nil {
		slog.Debug("coreAudioListProcesses failed", "error", err)
	}

	var appSources []Source
	seen := make(map[string]bool) // deduplicate by name
	for _, p := range processes {
		if p.PID <= 0 || !p.IsRunningOutput {
			continue
		}
		// Skip system daemons.
		lower := strings.ToLower(p.Name)
		if lower == "coreaudiod" || lower == "windowserver" || lower == "audiomxd" {
			continue
		}
		// Deduplicate by display name (e.g. multiple Chrome helper processes).
		if seen[p.Name] {
			continue
		}
		seen[p.Name] = true

		appSources = append(appSources, Source{
			ID:                 fmt.Sprintf("pid:%d", p.PID),
			Name:               p.Name,
			Playing:            true,
			VolumeControllable: false,
		})
	}

	// Sort app sources alphabetically.
	sort.Slice(appSources, func(i, j int) bool {
		return strings.ToLower(appSources[i].Name) < strings.ToLower(appSources[j].Name)
	})

	sources = append(sources, appSources...)

	// Cache for appNameForSource lookups.
	d.mu.Lock()
	d.cachedSources = sources
	d.mu.Unlock()

	return sources, nil
}

func (d *darwinBackend) ListOutputs() ([]Output, error) {
	// Use CoreAudio device enumeration for accurate output list.
	devices, err := coreAudioListOutputDevices()
	if err != nil {
		slog.Warn("coreAudioListOutputDevices failed, falling back", "error", err)
		// Fall back to system_profiler if cgo fails.
		devices = nil
	}

	var outputs []Output
	for _, dev := range devices {
		outputs = append(outputs, Output{
			ID:     fmt.Sprintf("%d", dev.ID),
			Name:   dev.Name,
			Active: dev.IsDefault,
		})
	}

	// Discover AirPlay devices not yet connected.
	airplay := d.discoverAirPlay()
	seen := make(map[string]bool)
	for _, o := range outputs {
		seen[strings.ToLower(o.Name)] = true
	}
	for _, ap := range airplay {
		if !seen[strings.ToLower(ap.Name)] {
			outputs = append(outputs, ap)
		}
	}

	return outputs, nil
}

func (d *darwinBackend) SetVolume(sourceID string, volume float32) error {
	if sourceID != "system" {
		return fmt.Errorf("unknown source: %s", sourceID)
	}
	vol := int(volume * 100)
	if vol < 0 {
		vol = 0
	}
	if vol > 100 {
		vol = 100
	}
	cmd := fmt.Sprintf("set volume output volume %d", vol)
	_, err := exec.Command("osascript", "-e", cmd).Output()
	return err
}

func (d *darwinBackend) ToggleMute(sourceID string) error {
	if sourceID != "system" {
		return fmt.Errorf("unknown source: %s", sourceID)
	}
	_, muted, err := d.getSystemVolume()
	if err != nil {
		return err
	}
	if muted {
		_, err = exec.Command("osascript", "-e", "set volume without output muted").Output()
	} else {
		_, err = exec.Command("osascript", "-e", "set volume with output muted").Output()
	}
	return err
}

func (d *darwinBackend) TogglePauseSource(sourceID string) error {
	if sourceID == "system" {
		return d.ToggleMute("system")
	}
	// For any per-app source or "active", toggle the active media player via nowplaying-cli.
	// macOS only exposes a single "active" media player, so we can't target a specific app.
	return d.toggleNowPlaying()
}

func (d *darwinBackend) PauseAll() error {
	// Try nowplaying-cli first for proper media pause.
	if hasNowPlayingCLI() {
		_ = exec.Command("nowplaying-cli", "pause").Run()
	}
	// Also mute system audio as a safety net.
	_, err := exec.Command("osascript", "-e", "set volume with output muted").Output()
	return err
}

// toggleNowPlaying sends a play/pause toggle to the active media player.
func (d *darwinBackend) toggleNowPlaying() error {
	if hasNowPlayingCLI() {
		return exec.Command("nowplaying-cli", "togglePlayPause").Run()
	}
	// Fallback: simulate the media play/pause key via AppleScript.
	// Key code 100 with no modifiers = F8 (media play/pause on Mac keyboards).
	_, err := exec.Command("osascript", "-e",
		`tell application "System Events" to key code 100`).CombinedOutput()
	return err
}

var nowPlayingCLIAvailable *bool

func hasNowPlayingCLI() bool {
	if nowPlayingCLIAvailable != nil {
		return *nowPlayingCLIAvailable
	}
	_, err := exec.LookPath("nowplaying-cli")
	available := err == nil
	nowPlayingCLIAvailable = &available
	if !available {
		slog.Info("nowplaying-cli not found, falling back to media key simulation (brew install nowplaying-cli)")
	}
	return available
}

func (d *darwinBackend) ToggleOutput(outputID string, active bool) error {
	if !active {
		return nil
	}
	// Parse device ID and set as default output via CoreAudio.
	var deviceID uint32
	if _, err := fmt.Sscanf(outputID, "%d", &deviceID); err != nil {
		return fmt.Errorf("invalid output ID %q: %w", outputID, err)
	}
	return coreAudioSetDefaultOutput(deviceID)
}

// getSystemVolume returns the system volume (0.0–1.0) and muted state.
func (d *darwinBackend) getSystemVolume() (float32, bool, error) {
	out, err := exec.Command("osascript", "-e", "output volume of (get volume settings)").Output()
	if err != nil {
		return 0, false, fmt.Errorf("get volume: %w", err)
	}
	volStr := strings.TrimSpace(string(out))
	vol, err := strconv.Atoi(volStr)
	if err != nil {
		return 0, false, fmt.Errorf("parse volume %q: %w", volStr, err)
	}

	mutedOut, err := exec.Command("osascript", "-e", "output muted of (get volume settings)").Output()
	if err != nil {
		return float32(vol) / 100.0, false, nil // best effort
	}
	muted := strings.TrimSpace(string(mutedOut)) == "true"

	return float32(vol) / 100.0, muted, nil
}

func (d *darwinBackend) discoverAirPlay() []Output {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		slog.Debug("zeroconf resolver failed", "error", err)
		return nil
	}

	entries := make(chan *zeroconf.ServiceEntry, 16)
	var results []Output
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for entry := range entries {
			results = append(results, Output{
				ID:        entry.Instance,
				Name:      entry.Instance,
				IsAirPlay: true,
			})
		}
	}()

	// Browse sends discovered entries to the channel and closes it when ctx expires.
	if err := resolver.Browse(ctx, "_raop._tcp", "local.", entries); err != nil {
		slog.Debug("airplay browse failed", "error", err)
		return nil
	}

	<-ctx.Done()
	// Wait for the consumer goroutine to drain the channel (closed by zeroconf).
	wg.Wait()

	slog.Debug("airplay discovery", "found", len(results))
	return results
}
