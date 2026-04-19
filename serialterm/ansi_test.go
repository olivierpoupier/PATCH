package serialterm

import (
	"reflect"
	"testing"
)

func TestScrollbackPlainText(t *testing.T) {
	s := newScrollback()
	s.Write([]byte("hello\nworld\n"))
	got := s.Lines()
	want := []string{"hello", "world"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("plain text: got %q, want %q", got, want)
	}
}

func TestScrollbackInProgressLine(t *testing.T) {
	s := newScrollback()
	s.Write([]byte("one\ntwo"))
	got := s.Lines()
	want := []string{"one", "two"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("in-progress: got %q, want %q", got, want)
	}
}

func TestScrollbackSGRPassthrough(t *testing.T) {
	s := newScrollback()
	s.Write([]byte("\x1b[31mRED\x1b[0m\n"))
	got := s.Lines()
	want := []string{"\x1b[31mRED\x1b[0m"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SGR passthrough: got %q, want %q", got, want)
	}
}

func TestScrollbackStripsCUP(t *testing.T) {
	// Cursor-position and erase escapes must be dropped.
	s := newScrollback()
	s.Write([]byte("a\x1b[2;5Hb\x1b[Kc\n"))
	got := s.Lines()
	want := []string{"abc"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CUP+EL strip: got %q, want %q", got, want)
	}
}

func TestScrollbackCRResetsVisible(t *testing.T) {
	// Our line-mode scanner treats CR as "redraw current line": visible
	// bytes are cleared and subsequent writes start at column 0. This gives
	// clean output for common cases like progress bars and Flipper prompt
	// redraws without needing a full VT emulator.
	s := newScrollback()
	s.Write([]byte("first\rse\n"))
	got := s.Lines()
	want := []string{"se"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CR reset: got %q, want %q", got, want)
	}
}

func TestScrollbackSGRResetStopsCarry(t *testing.T) {
	// The explicit reset stays in the committed line (the device sent it),
	// but it also clears the carried-forward style so subsequent lines are
	// not tinted with the previous color.
	s := newScrollback()
	s.Write([]byte("\x1b[31mRED\x1b[0m\nplain\n"))
	got := s.Lines()
	want := []string{"\x1b[31mRED\x1b[0m", "plain"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SGR reset stops carry: got %q, want %q", got, want)
	}
}

func TestScrollbackSGRCarriesAcrossLines(t *testing.T) {
	// Without an explicit reset the active SGR should carry to the next line
	// so the user sees continuous color, matching real terminal behaviour.
	s := newScrollback()
	s.Write([]byte("\x1b[31mRED\nstill-red\n"))
	got := s.Lines()
	want := []string{"\x1b[31mRED", "\x1b[31mstill-red"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SGR carry: got %q, want %q", got, want)
	}
}

func TestScrollbackBackspace(t *testing.T) {
	s := newScrollback()
	s.Write([]byte("abc\b\bX\n"))
	got := s.Lines()
	want := []string{"aX"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("backspace: got %q, want %q", got, want)
	}
}

func TestScrollbackTab(t *testing.T) {
	s := newScrollback()
	s.Write([]byte("a\tb\n"))
	got := s.Lines()
	want := []string{"a    b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tab expand: got %q, want %q", got, want)
	}
}

func TestScrollbackClear(t *testing.T) {
	s := newScrollback()
	s.Write([]byte("a\nb\n"))
	s.Clear()
	if got := s.Lines(); len(got) != 0 {
		t.Fatalf("clear: expected empty, got %q", got)
	}
}

func TestScrollbackCapacityRing(t *testing.T) {
	s := &scrollback{capacity: 3}
	s.Write([]byte("a\nb\nc\nd\ne\n"))
	got := s.Lines()
	want := []string{"c", "d", "e"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ring: got %q, want %q", got, want)
	}
}

func TestScrollbackFlipperSample(t *testing.T) {
	// Representative of a Flipper "device_info" reply intermixed with ANSI
	// prompt formatting. All cursor-movement escapes should be stripped,
	// colors should be kept, and key/value lines should appear.
	input := []byte(
		"\x1b[31;1m>:\x1b[0m device_info\r\n" +
			"hardware_ver        : 13\r\n" +
			"firmware_version    : 0.95.1\r\n" +
			"\x1b[31;1m>:\x1b[0m ")
	s := newScrollback()
	s.Write(input)
	lines := s.Lines()
	if len(lines) < 3 {
		t.Fatalf("flipper sample: expected at least 3 lines, got %d: %q", len(lines), lines)
	}
	// Parser should be able to find key/value lines from the buffer.
	parsed := parseFlipperKV(input)
	if parsed["hardware_ver"] != "13" {
		t.Errorf("parseFlipperKV hardware_ver: got %q", parsed["hardware_ver"])
	}
	if parsed["firmware_version"] != "0.95.1" {
		t.Errorf("parseFlipperKV firmware_version: got %q", parsed["firmware_version"])
	}
}
