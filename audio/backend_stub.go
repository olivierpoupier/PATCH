//go:build !darwin && !linux

package audio

import "errors"

var errUnsupported = errors.New("audio: unsupported platform")

type stubBackend struct{}

func newPlatformBackend() Backend { return &stubBackend{} }

func (s *stubBackend) Init() error                                  { return errUnsupported }
func (s *stubBackend) Close() error                                 { return nil }
func (s *stubBackend) ListSources() ([]Source, error)               { return nil, errUnsupported }
func (s *stubBackend) ListOutputs() ([]Output, error)               { return nil, errUnsupported }
func (s *stubBackend) SetVolume(string, float32) error              { return errUnsupported }
func (s *stubBackend) ToggleMute(string) error                      { return errUnsupported }
func (s *stubBackend) TogglePauseSource(string) error               { return errUnsupported }
func (s *stubBackend) PauseAll() error                              { return errUnsupported }
func (s *stubBackend) ToggleOutput(string, bool) error              { return errUnsupported }
