package devices

import (
	"testing"

	"github.com/olivierpoupier/patch/tui"
)

// To keep this test hermetic the test registers its own descriptors into the
// global registry. Since Register appends and the fallback is overwritten,
// tests must run after any real device packages register (go test picks up
// init() from imported packages). We snapshot and restore around each test.
func withClean(t *testing.T, fn func()) {
	t.Helper()
	mu.Lock()
	savedDesc := descriptors
	savedFB := fallback
	descriptors = nil
	fallback = nil
	mu.Unlock()
	defer func() {
		mu.Lock()
		descriptors = savedDesc
		fallback = savedFB
		mu.Unlock()
	}()
	fn()
}

func TestResolveExactMatch(t *testing.T) {
	withClean(t, func() {
		Register(Descriptor{
			Key:   "flipper",
			Name:  "Flipper Zero",
			Baud:  115200,
			Match: MatchVIDPID("0483", "5740"),
		})
		info := tui.SerialDeviceInfo{VID: "0483", PID: "5740"}
		got, ok := Resolve(info)
		if !ok || got.Key != "flipper" {
			t.Fatalf("exact match: got %+v ok=%v, want flipper", got, ok)
		}
	})
}

func TestResolveVIDOnlyWildcard(t *testing.T) {
	withClean(t, func() {
		Register(Descriptor{
			Key:   "esp32",
			Baud:  115200,
			Match: MatchVIDPID("303a", ""),
		})
		info := tui.SerialDeviceInfo{VID: "303a", PID: "deadbeef"}
		got, ok := Resolve(info)
		if !ok || got.Key != "esp32" {
			t.Fatalf("VID wildcard: got %+v ok=%v", got, ok)
		}
	})
}

func TestResolvePrefersEarlierRegistration(t *testing.T) {
	withClean(t, func() {
		// A specific (VID, PID) descriptor registered before a broader
		// VID-only wildcard should win when both match.
		Register(Descriptor{
			Key:   "marauder",
			Match: MatchVIDPID("303a", "1001"),
		})
		Register(Descriptor{
			Key:   "esp32",
			Match: MatchVIDPID("303a", ""),
		})
		got, ok := Resolve(tui.SerialDeviceInfo{VID: "303a", PID: "1001"})
		if !ok || got.Key != "marauder" {
			t.Fatalf("specific-first: got %+v ok=%v", got, ok)
		}
		// A device without the specific PID still falls into the wildcard.
		got, ok = Resolve(tui.SerialDeviceInfo{VID: "303a", PID: "0001"})
		if !ok || got.Key != "esp32" {
			t.Fatalf("wildcard fallback: got %+v ok=%v", got, ok)
		}
	})
}

func TestResolveFallback(t *testing.T) {
	withClean(t, func() {
		Register(Descriptor{Key: "generic", Name: "USB Serial Device", Baud: 115200})
		got, ok := Resolve(tui.SerialDeviceInfo{VID: "dead", PID: "beef"})
		if !ok || got.Key != "generic" {
			t.Fatalf("fallback: got %+v ok=%v", got, ok)
		}
	})
}

func TestResolveNoMatch(t *testing.T) {
	withClean(t, func() {
		_, ok := Resolve(tui.SerialDeviceInfo{VID: "dead", PID: "beef"})
		if ok {
			t.Fatal("expected ok=false with empty registry")
		}
	})
}

func TestHasCustom(t *testing.T) {
	withClean(t, func() {
		Register(Descriptor{Key: "flipper", Match: MatchVIDPID("0483", "5740")})
		Register(Descriptor{Key: "generic", Match: nil}) // fallback
		if !HasCustom(tui.SerialDeviceInfo{VID: "0483", PID: "5740"}) {
			t.Error("Flipper should be a custom device")
		}
		if HasCustom(tui.SerialDeviceInfo{VID: "dead", PID: "beef"}) {
			t.Error("unknown device should not be custom (fallback is not counted)")
		}
	})
}

func TestMatchVIDPIDNormalizesHexPrefix(t *testing.T) {
	m := MatchVIDPID("0x0483", "0x5740")
	if !m(tui.SerialDeviceInfo{VID: "0483", PID: "5740"}) {
		t.Error("0x-prefixed matcher should normalize")
	}
	if !m(tui.SerialDeviceInfo{VID: "0X0483", PID: "0X5740"}) {
		t.Error("case-insensitive 0X should match")
	}
}

func TestNormalizeHex(t *testing.T) {
	cases := []struct{ in, want string }{
		{"0483", "0483"},
		{"0x0483", "0483"},
		{"0X0483", "0483"},
		{"  0483  ", "0483"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := NormalizeHex(tc.in); got != tc.want {
			t.Errorf("NormalizeHex(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
