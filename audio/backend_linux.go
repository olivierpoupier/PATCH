//go:build linux

package audio

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/mafik/pulseaudio"
)

type linuxBackend struct {
	client *pulseaudio.Client
	mu     sync.Mutex
}

func newPlatformBackend() Backend {
	return &linuxBackend{}
}

func (l *linuxBackend) Init() error {
	client, err := pulseaudio.NewClient()
	if err != nil {
		return fmt.Errorf("pulseaudio: %w", err)
	}
	l.client = client
	return nil
}

func (l *linuxBackend) Close() error {
	if l.client != nil {
		l.client.Close()
		l.client = nil
	}
	return nil
}

// pactl JSON structures for sink-inputs.
type pactlSinkInput struct {
	Index      int               `json:"index"`
	State      string            `json:"state"`
	Name       string            `json:"name"`
	Volume     map[string]pactlVol `json:"volume"`
	Mute       bool              `json:"mute"`
	Properties pactlProperties   `json:"properties"`
}

type pactlVol struct {
	ValuePercent string `json:"value_percent"`
}

type pactlProperties struct {
	AppName string `json:"application.name"`
}

func (l *linuxBackend) ListSources() ([]Source, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	var sources []Source

	// System volume as first source.
	if l.client != nil {
		vol, err := l.client.Volume()
		if err == nil {
			muted, _ := l.client.Mute()
			sources = append(sources, Source{
				ID:                 "system",
				Name:               "System Audio",
				Volume:             vol,
				Muted:              muted,
				Playing:            !muted && vol > 0,
				IsSystem:           true,
				VolumeControllable: true,
			})
		}
	}

	// Per-app sources via pactl.
	appSources, err := l.listSinkInputs()
	if err != nil {
		slog.Debug("pactl sink-inputs failed", "error", err)
	} else {
		sources = append(sources, appSources...)
	}

	// Sort: playing first.
	sort.SliceStable(sources, func(i, j int) bool {
		if sources[i].IsSystem {
			return true
		}
		if sources[j].IsSystem {
			return false
		}
		if sources[i].Playing != sources[j].Playing {
			return sources[i].Playing
		}
		return false
	})

	return sources, nil
}

func (l *linuxBackend) listSinkInputs() ([]Source, error) {
	out, err := exec.Command("pactl", "-f", "json", "list", "sink-inputs").Output()
	if err != nil {
		return nil, fmt.Errorf("pactl: %w", err)
	}

	var inputs []pactlSinkInput
	if err := json.Unmarshal(out, &inputs); err != nil {
		return nil, fmt.Errorf("parse sink-inputs: %w", err)
	}

	var sources []Source
	for _, input := range inputs {
		name := input.Properties.AppName
		if name == "" {
			name = input.Name
		}

		// Parse volume from first channel.
		var vol float32
		for _, v := range input.Volume {
			pct := strings.TrimSuffix(v.ValuePercent, "%")
			pct = strings.TrimSpace(pct)
			if n, err := strconv.Atoi(pct); err == nil {
				vol = float32(n) / 100.0
			}
			break
		}

		playing := input.State == "RUNNING"
		sources = append(sources, Source{
			ID:                 strconv.Itoa(input.Index),
			Name:               name,
			Volume:             vol,
			Muted:              input.Mute,
			Playing:            playing,
			VolumeControllable: true,
		})
	}
	return sources, nil
}

func (l *linuxBackend) ListOutputs() ([]Output, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.client == nil {
		return nil, fmt.Errorf("pulseaudio not connected")
	}

	paOutputs, activeIdx, err := l.client.Outputs()
	if err != nil {
		return nil, fmt.Errorf("list outputs: %w", err)
	}

	var outputs []Output
	for i, o := range paOutputs {
		// Skip the "None" entry.
		if o.CardID == "all" && o.PortID == "none" {
			continue
		}
		name := o.PortName
		if o.CardName != "" {
			name = o.CardName + " - " + o.PortName
		}
		outputs = append(outputs, Output{
			ID:     o.CardID + ":" + o.PortID,
			Name:   name,
			Active: i == activeIdx,
		})
	}
	return outputs, nil
}

func (l *linuxBackend) SetVolume(sourceID string, volume float32) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if sourceID == "system" {
		if l.client == nil {
			return fmt.Errorf("pulseaudio not connected")
		}
		return l.client.SetVolume(volume)
	}

	// Per-app volume via pactl.
	pct := int(volume * 100)
	if pct < 0 {
		pct = 0
	}
	if pct > 150 {
		pct = 150
	}
	_, err := exec.Command("pactl", "set-sink-input-volume", sourceID, fmt.Sprintf("%d%%", pct)).Output()
	return err
}

func (l *linuxBackend) ToggleMute(sourceID string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if sourceID == "system" {
		if l.client == nil {
			return fmt.Errorf("pulseaudio not connected")
		}
		_, err := l.client.ToggleMute()
		return err
	}

	_, err := exec.Command("pactl", "set-sink-input-mute", sourceID, "toggle").Output()
	return err
}

func (l *linuxBackend) TogglePauseSource(sourceID string) error {
	return l.ToggleMute(sourceID)
}

func (l *linuxBackend) PauseAll() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.client != nil {
		_ = l.client.SetMute(true)
	}

	// Also mute all sink-inputs.
	out, err := exec.Command("pactl", "-f", "json", "list", "sink-inputs").Output()
	if err != nil {
		return err
	}
	var inputs []pactlSinkInput
	if err := json.Unmarshal(out, &inputs); err != nil {
		return err
	}
	for _, input := range inputs {
		exec.Command("pactl", "set-sink-input-mute", strconv.Itoa(input.Index), "1").Run()
	}
	return nil
}

func (l *linuxBackend) ToggleOutput(outputID string, active bool) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !active || l.client == nil {
		return nil
	}

	// Find and activate the matching output.
	paOutputs, _, err := l.client.Outputs()
	if err != nil {
		return err
	}
	for _, o := range paOutputs {
		id := o.CardID + ":" + o.PortID
		if id == outputID {
			return o.Activate()
		}
	}
	return fmt.Errorf("output %s not found", outputID)
}
