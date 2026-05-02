package serialterm

import "bytes"

// defaultScrollbackCapacity caps the terminal history to prevent unbounded
// memory growth. Overridable per Scrollback via NewScrollback.
const defaultScrollbackCapacity = 4000

// Scrollback accumulates bytes from the serial device into terminal-friendly
// lines. It is a line-mode sanitiser, not a full VT100:
//
//   - SGR sequences (\e[...m) are preserved so lipgloss-rendered colors flow
//     through unchanged.
//   - Every other CSI sequence (cursor motion, erase, scroll, alternate
//     screens, save/restore) is discarded so it can't corrupt bubbletea's
//     rendered frame.
//   - LF ends a line. CRLF counts as LF only. Lone CR resets the current
//     line back to the carried-forward style, which handles both prompt
//     redraws and simple progress-bar re-render patterns.
//   - The currently-active SGR state is carried forward across line breaks,
//     matching real-terminal behaviour.
type Scrollback struct {
	lines     []string
	cur       []byte
	col       int    // visible column, advanced only by printable writes
	activeSGR []byte // last non-reset SGR sequence (empty after explicit \e[0m)
	gotCR     bool   // true between a standalone \r and the next byte
	state     ansiState
	pending   []byte
	capacity  int
}

type ansiState int

const (
	stateNormal ansiState = iota
	stateEsc
	stateCSI
)

// NewScrollback creates a Scrollback with the given line-capacity limit. When
// capacity <= 0 the default (4000 lines) is used.
func NewScrollback(capacity int) *Scrollback {
	if capacity <= 0 {
		capacity = defaultScrollbackCapacity
	}
	return &Scrollback{capacity: capacity}
}

// Write feeds bytes through the state machine.
func (s *Scrollback) Write(p []byte) {
	for _, b := range p {
		s.feed(b)
	}
}

// Lines returns committed lines plus (if non-empty) the in-progress line.
// A cur that contains only the carried-forward SGR prefix is treated as
// empty so it doesn't render as a trailing blank line.
func (s *Scrollback) Lines() []string {
	if s.col == 0 && bytes.Equal(s.cur, s.activeSGR) {
		return s.lines
	}
	out := make([]string, 0, len(s.lines)+1)
	out = append(out, s.lines...)
	out = append(out, string(s.cur))
	return out
}

// Clear resets all scrollback state.
func (s *Scrollback) Clear() {
	s.lines = nil
	s.cur = s.cur[:0]
	s.col = 0
	s.activeSGR = s.activeSGR[:0]
	s.gotCR = false
	s.state = stateNormal
	s.pending = s.pending[:0]
}

func (s *Scrollback) feed(b byte) {
	// Resolve a pending CR when the next byte arrives.
	if s.gotCR && b != '\n' && s.state == stateNormal {
		s.resetLine()
		s.gotCR = false
	}
	switch s.state {
	case stateNormal:
		s.feedNormal(b)
	case stateEsc:
		s.feedEsc(b)
	case stateCSI:
		s.feedCSI(b)
	}
}

func (s *Scrollback) feedNormal(b byte) {
	switch b {
	case 0x1b:
		s.state = stateEsc
		s.pending = append(s.pending[:0], b)
	case '\n':
		s.commitLine()
		s.gotCR = false
	case '\r':
		s.gotCR = true
	case '\b':
		if s.col > 0 {
			s.col--
			// Truncate the last visible byte. With SGR interleaved we can't
			// easily "cut out" a middle byte, so approximate by dropping the
			// trailing byte only if it's a visible character.
			if n := len(s.cur); n > 0 && s.cur[n-1] >= 0x20 {
				s.cur = s.cur[:n-1]
			}
		}
	case '\t':
		for i := 0; i < 4; i++ {
			s.writeVisible(' ')
		}
	case 0x07:
		// BEL — drop silently.
	default:
		if b >= 0x20 {
			s.writeVisible(b)
		}
	}
}

func (s *Scrollback) feedEsc(b byte) {
	s.pending = append(s.pending, b)
	switch b {
	case '[':
		s.state = stateCSI
	case 'P', ']', 'X', '^', '_':
		s.state = stateNormal
		s.pending = s.pending[:0]
	default:
		s.state = stateNormal
		s.pending = s.pending[:0]
	}
}

func (s *Scrollback) feedCSI(b byte) {
	s.pending = append(s.pending, b)
	if b >= 0x40 && b <= 0x7E {
		if b == 'm' {
			// SGR — emit into the current line and track carry-forward state.
			s.cur = append(s.cur, s.pending...)
			if isSGRReset(s.pending) {
				s.activeSGR = s.activeSGR[:0]
			} else {
				s.activeSGR = append(s.activeSGR[:0], s.pending...)
			}
		}
		s.state = stateNormal
		s.pending = s.pending[:0]
	}
}

func isSGRReset(seq []byte) bool {
	if len(seq) < 3 || seq[len(seq)-1] != 'm' {
		return false
	}
	params := seq[2 : len(seq)-1]
	if len(params) == 0 {
		return true
	}
	return bytes.Equal(params, []byte("0"))
}

func (s *Scrollback) writeVisible(b byte) {
	s.cur = append(s.cur, b)
	s.col++
}

// resetLine clears the in-progress line back to the active carried style.
func (s *Scrollback) resetLine() {
	s.cur = append(s.cur[:0], s.activeSGR...)
	s.col = 0
}

func (s *Scrollback) commitLine() {
	line := string(s.cur)
	s.lines = append(s.lines, line)
	if len(s.lines) > s.capacity {
		drop := len(s.lines) - s.capacity
		s.lines = s.lines[drop:]
	}
	s.cur = append(s.cur[:0], s.activeSGR...)
	s.col = 0
}
