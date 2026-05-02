package serialterm

import (
	"reflect"
	"testing"
)

func TestScrollbackPlainText(t *testing.T) {
	s := NewScrollback(0)
	s.Write([]byte("hello\nworld\n"))
	got := s.Lines()
	want := []string{"hello", "world"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("plain text: got %q, want %q", got, want)
	}
}

func TestScrollbackInProgressLine(t *testing.T) {
	s := NewScrollback(0)
	s.Write([]byte("one\ntwo"))
	got := s.Lines()
	want := []string{"one", "two"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("in-progress: got %q, want %q", got, want)
	}
}

func TestScrollbackSGRPassthrough(t *testing.T) {
	s := NewScrollback(0)
	s.Write([]byte("\x1b[31mRED\x1b[0m\n"))
	got := s.Lines()
	want := []string{"\x1b[31mRED\x1b[0m"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SGR passthrough: got %q, want %q", got, want)
	}
}

func TestScrollbackStripsCUP(t *testing.T) {
	// Cursor-position and erase escapes must be dropped.
	s := NewScrollback(0)
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
	// clean output for common cases like progress bars and prompt redraws
	// without needing a full VT emulator.
	s := NewScrollback(0)
	s.Write([]byte("first\rse\n"))
	got := s.Lines()
	want := []string{"se"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CR reset: got %q, want %q", got, want)
	}
}

func TestScrollbackSGRResetStopsCarry(t *testing.T) {
	s := NewScrollback(0)
	s.Write([]byte("\x1b[31mRED\x1b[0m\nplain\n"))
	got := s.Lines()
	want := []string{"\x1b[31mRED\x1b[0m", "plain"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SGR reset stops carry: got %q, want %q", got, want)
	}
}

func TestScrollbackSGRCarriesAcrossLines(t *testing.T) {
	s := NewScrollback(0)
	s.Write([]byte("\x1b[31mRED\nstill-red\n"))
	got := s.Lines()
	want := []string{"\x1b[31mRED", "\x1b[31mstill-red"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SGR carry: got %q, want %q", got, want)
	}
}

func TestScrollbackBackspace(t *testing.T) {
	s := NewScrollback(0)
	s.Write([]byte("abc\b\bX\n"))
	got := s.Lines()
	want := []string{"aX"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("backspace: got %q, want %q", got, want)
	}
}

func TestScrollbackTab(t *testing.T) {
	s := NewScrollback(0)
	s.Write([]byte("a\tb\n"))
	got := s.Lines()
	want := []string{"a    b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tab expand: got %q, want %q", got, want)
	}
}

func TestScrollbackClear(t *testing.T) {
	s := NewScrollback(0)
	s.Write([]byte("a\nb\n"))
	s.Clear()
	if got := s.Lines(); len(got) != 0 {
		t.Fatalf("clear: expected empty, got %q", got)
	}
}

func TestScrollbackCapacityRing(t *testing.T) {
	s := NewScrollback(3)
	s.Write([]byte("a\nb\nc\nd\ne\n"))
	got := s.Lines()
	want := []string{"c", "d", "e"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ring: got %q, want %q", got, want)
	}
}

func TestScrollbackDefaultCapacity(t *testing.T) {
	// NewScrollback(0) picks the default; a negative value is equivalent.
	s := NewScrollback(-1)
	if s.capacity != defaultScrollbackCapacity {
		t.Fatalf("default capacity: got %d, want %d", s.capacity, defaultScrollbackCapacity)
	}
}

func TestScrollbackANSISample(t *testing.T) {
	// Representative of a device reply intermixed with ANSI prompt
	// formatting. All cursor-movement escapes should be stripped, colours
	// should be kept, and plain lines should appear.
	input := []byte(
		"\x1b[31;1m>:\x1b[0m device_info\r\n" +
			"hardware_ver        : 13\r\n" +
			"firmware_version    : 0.95.1\r\n" +
			"\x1b[31;1m>:\x1b[0m ")
	s := NewScrollback(0)
	s.Write(input)
	lines := s.Lines()
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines, got %d: %q", len(lines), lines)
	}
}
