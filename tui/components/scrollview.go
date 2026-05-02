package components

import (
	"strings"

	"github.com/olivierpoupier/patch/tui"

	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
)

// ScrollableView wraps a viewport with a scrollbar overlay and auto-scroll.
type ScrollableView struct {
	viewport      viewport.Model
	theme         *tui.Theme
	width         int
	height        int
	content       string
	contentLines  int
	bottomPadding int
}

// NewScrollableView creates a new scrollable view.
func NewScrollableView(theme *tui.Theme) ScrollableView {
	return ScrollableView{theme: theme}
}

// SetContent sets the viewport's content string. Bottom padding (if set via
// SetBottomPadding) is appended internally; the scrollbar's range tracks only
// the real content so the thumb reaches its extremes when the user is at the
// first or last line.
func (s ScrollableView) SetContent(content string) ScrollableView {
	s.content = content
	s.contentLines = strings.Count(content, "\n") + 1
	return s.applyContent()
}

// SetBottomPadding sets a number of blank lines appended after the content so
// the last real row can be scrolled up off the bottom edge (vim scrolloff).
// The scrollbar excludes these lines from its calculations.
func (s ScrollableView) SetBottomPadding(n int) ScrollableView {
	if n < 0 {
		n = 0
	}
	if s.bottomPadding == n {
		return s
	}
	s.bottomPadding = n
	return s.applyContent()
}

func (s ScrollableView) applyContent() ScrollableView {
	padded := s.content
	if s.bottomPadding > 0 {
		padded += strings.Repeat("\n", s.bottomPadding)
	}
	s.viewport.SetContent(padded)
	return s
}

// SetSize sets the viewport dimensions.
func (s ScrollableView) SetSize(w, h int) ScrollableView {
	s.width = w
	s.height = h
	s.viewport.SetWidth(w)
	s.viewport.SetHeight(h)
	return s
}

// ScrollToLine adjusts the viewport offset to keep the given line in view,
// using a 20% margin on top and bottom (vim scrolloff style).
func (s ScrollableView) ScrollToLine(line int) ScrollableView {
	vpHeight := s.viewport.Height()
	if vpHeight <= 0 {
		return s
	}
	margin := vpHeight / 5
	if margin < 1 {
		margin = 1
	}
	yOffset := s.viewport.YOffset()
	if line < yOffset+margin {
		s.viewport.SetYOffset(line - margin)
	} else if line >= yOffset+vpHeight-margin {
		s.viewport.SetYOffset(line - vpHeight + margin + 1)
	}
	if s.viewport.YOffset() < 0 {
		s.viewport.SetYOffset(0)
	}
	return s
}

// YOffset returns the current vertical scroll offset.
func (s ScrollableView) YOffset() int {
	return s.viewport.YOffset()
}

// SetYOffset sets the vertical scroll offset.
func (s ScrollableView) SetYOffset(offset int) ScrollableView {
	s.viewport.SetYOffset(offset)
	return s
}

// Height returns the viewport height.
func (s ScrollableView) Height() int {
	return s.viewport.Height()
}

// View renders the viewport content with a scrollbar overlay on the right edge.
func (s ScrollableView) View() string {
	vpView := s.viewport.View()
	return s.overlayScrollbar(vpView)
}

// overlayScrollbar renders a scrollbar track on the right edge of the viewport.
// The thumb is sized and positioned relative to the *real* content (excluding
// any bottom padding added via SetBottomPadding), so the thumb reaches the
// bottom of its range exactly when the last real line is on screen.
func (s ScrollableView) overlayScrollbar(vpView string) string {
	vpHeight := s.height
	realLines := s.contentLines
	if realLines <= vpHeight {
		return vpView
	}

	trackStyle := lipgloss.NewStyle().Foreground(s.theme.Inactive)
	thumbStyle := lipgloss.NewStyle().Foreground(s.theme.Active)

	thumbSize := vpHeight * vpHeight / realLines
	if thumbSize < 1 {
		thumbSize = 1
	}
	if thumbSize > vpHeight {
		thumbSize = vpHeight
	}
	scrollable := realLines - vpHeight
	yOffset := s.viewport.YOffset()
	if yOffset < 0 {
		yOffset = 0
	}
	if yOffset > scrollable {
		yOffset = scrollable
	}
	thumbPos := 0
	if scrollable > 0 {
		if yOffset >= scrollable {
			thumbPos = vpHeight - thumbSize
		} else {
			thumbPos = yOffset * (vpHeight - thumbSize) / scrollable
		}
	}

	lines := strings.Split(vpView, "\n")
	if len(lines) > vpHeight {
		lines = lines[:vpHeight]
	}

	var b strings.Builder
	for i := 0; i < len(lines); i++ {
		b.WriteString(lines[i])
		if i >= thumbPos && i < thumbPos+thumbSize {
			b.WriteString(thumbStyle.Render("┃"))
		} else {
			b.WriteString(trackStyle.Render("│"))
		}
		if i < len(lines)-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}
