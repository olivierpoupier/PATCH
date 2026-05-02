// Package devices is the registry that maps a USB serial device to its
// per-device view. Each supported device lives in its own subpackage
// (devices/flipper, devices/generic, …) and registers a Descriptor in an
// init() function. The top-level binary blank-imports those subpackages so
// their registrations are linked in at compile time; this follows the same
// idiom as image.RegisterFormat and database/sql.Register.
package devices

import (
	"strings"
	"sync"

	"github.com/olivierpoupier/patch/tui"
)

// Descriptor describes one device type registered with the dispatcher.
//
// A descriptor with Match == nil is treated as the registry's fallback and is
// selected only when no matcher-bearing descriptor matches. Only one fallback
// is kept (last registration wins).
type Descriptor struct {
	// Key is a short identifier used in logs and messages (e.g. "flipper_zero").
	Key string

	// Name is the display name for devices that do not provide their own
	// (e.g. the Product string from the USB enumerator was empty).
	Name string

	// Baud is the default baud rate when SerialDeviceInfo.Baud is 0.
	Baud int

	// Match reports whether this descriptor applies to the given device info.
	// Matchers are tried in registration order; the first true wins.
	Match func(info tui.SerialDeviceInfo) bool

	// New constructs a DeviceView. Called once per modal opening.
	New func(theme *tui.Theme) tui.DeviceView
}

var (
	mu          sync.RWMutex
	descriptors []Descriptor
	fallback    *Descriptor
)

// Register adds a descriptor to the registry. Intended to be called from an
// init() function in each device package. Descriptors should be registered
// most-specific-first: an exact (VID, PID) matcher before a VID-only
// wildcard, so a device served by its own package wins over a generic
// fallback package that claims the broader VID.
func Register(d Descriptor) {
	mu.Lock()
	defer mu.Unlock()
	if d.Match == nil {
		cp := d
		fallback = &cp
		return
	}
	descriptors = append(descriptors, d)
}

// Resolve returns the descriptor that matches info, falling back to the
// generic entry if no device-specific matcher succeeds. Reports ok=false if
// no matcher matched and no fallback has been registered.
func Resolve(info tui.SerialDeviceInfo) (Descriptor, bool) {
	mu.RLock()
	defer mu.RUnlock()
	for _, d := range descriptors {
		if d.Match(info) {
			return d, true
		}
	}
	if fallback != nil {
		return *fallback, true
	}
	return Descriptor{}, false
}

// HasCustom reports whether a non-fallback descriptor exists for info. The
// USB tab uses this to decide whether to show the "open serial terminal"
// affordance for a given device.
func HasCustom(info tui.SerialDeviceInfo) bool {
	mu.RLock()
	defer mu.RUnlock()
	for _, d := range descriptors {
		if d.Match(info) {
			return true
		}
	}
	return false
}

// MatchVIDPID returns a matcher for a specific VID/PID pair. An empty pid
// acts as a VID-only wildcard. Both values are normalised before comparison
// so "0x0483" and "0483" match the same devices.
func MatchVIDPID(vid, pid string) func(tui.SerialDeviceInfo) bool {
	vid = NormalizeHex(vid)
	pid = NormalizeHex(pid)
	return func(info tui.SerialDeviceInfo) bool {
		if NormalizeHex(info.VID) != vid {
			return false
		}
		if pid != "" && NormalizeHex(info.PID) != pid {
			return false
		}
		return true
	}
}

// NormalizeHex lowercases and strips a "0x" prefix from a hex VID/PID string.
func NormalizeHex(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.TrimPrefix(s, "0x")
	return s
}
