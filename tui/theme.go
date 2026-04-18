package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Theme holds all application-wide colors and pre-built styles.
// Pass to tabs via their constructor: usb.New(theme).
type Theme struct {
	// Semantic colors.
	Active   color.Color
	Inactive color.Color
	Error    color.Color
	Warning  color.Color
	Text     color.Color
	TextDim  color.Color
	BgDark   color.Color

	// Pre-built styles (created once, reused everywhere).
	Label     lipgloss.Style // Bold + Active foreground
	Value     lipgloss.Style // Text foreground
	Dim       lipgloss.Style // TextDim foreground
	ErrorText lipgloss.Style // Error foreground
	Help      lipgloss.Style // Inactive foreground

	// Table styles.
	TableHeader lipgloss.Style // Bold + Active + padding
	TableCell   lipgloss.Style // Text + padding
	Selected    lipgloss.Style // Active background + black foreground + padding

	// Tab bar styles.
	TabActive   lipgloss.Style
	TabInactive lipgloss.Style

	// Border style.
	Border lipgloss.Style
}

// NewTheme creates the default PATCH theme with all colors and styles pre-built.
func NewTheme() *Theme {
	active := lipgloss.Color("#04B575")
	inactive := lipgloss.Color("#888888")
	errColor := lipgloss.Color("#FF4444")
	warning := lipgloss.Color("#FFAA00")
	text := lipgloss.Color("#FFFFFF")
	textDim := lipgloss.Color("#888888")
	bgDark := lipgloss.Color("#000000")

	return &Theme{
		Active:   active,
		Inactive: inactive,
		Error:    errColor,
		Warning:  warning,
		Text:     text,
		TextDim:  textDim,
		BgDark:   bgDark,

		Label:     lipgloss.NewStyle().Bold(true).Foreground(active),
		Value:     lipgloss.NewStyle().Foreground(text),
		Dim:       lipgloss.NewStyle().Foreground(textDim),
		ErrorText: lipgloss.NewStyle().Foreground(errColor),
		Help:      lipgloss.NewStyle().Foreground(inactive),

		TableHeader: lipgloss.NewStyle().Bold(true).Foreground(active).Padding(0, 1),
		TableCell:   lipgloss.NewStyle().Foreground(text).Padding(0, 1),
		Selected:    lipgloss.NewStyle().Background(active).Foreground(bgDark).Padding(0, 1),

		TabActive: lipgloss.NewStyle().
			Bold(true).
			Foreground(active).
			Border(lipgloss.RoundedBorder(), true, true, false, true).
			BorderForeground(active).
			Padding(0, 1),

		TabInactive: lipgloss.NewStyle().
			Foreground(inactive).
			Border(lipgloss.RoundedBorder(), true, true, true, true).
			BorderForeground(inactive).
			Padding(0, 1),

		Border: lipgloss.NewStyle().Foreground(active),
	}
}
