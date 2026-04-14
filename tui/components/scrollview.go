package components

import (
	"strings"

	"github.com/olivierpoupier/patch/tui"

	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
)

// ScrollableView wraps a viewport with a scrollbar overlay and auto-scroll.
type ScrollableView struct {
	viewport viewport.Model
	theme    *tui.Theme
	width    int
	height   int
}

// NewScrollableView creates a new scrollable view.
func NewScrollableView(theme *tui.Theme) ScrollableView {
	return ScrollableView{theme: theme}
}

// SetContent sets the viewport's content string.
func (s ScrollableView) SetContent(content string) ScrollableView {
	s.viewport.SetContent(content)
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
func (s ScrollableView) overlayScrollbar(vpView string) string {
	vpHeight := s.height
	totalLines := s.viewport.TotalLineCount()
	if totalLines <= vpHeight {
		return vpView
	}

	trackStyle := lipgloss.NewStyle().Foreground(s.theme.Inactive)
	thumbStyle := lipgloss.NewStyle().Foreground(s.theme.Active)

	thumbSize := vpHeight * vpHeight / totalLines
	if thumbSize < 1 {
		thumbSize = 1
	}
	scrollable := totalLines - vpHeight
	if scrollable < 0 {
		scrollable = 0
	}
	yOffset := s.viewport.YOffset()
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
