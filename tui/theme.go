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

	// Terminal-mode sub-theme used by the serial terminal modal.
	Terminal *TerminalTheme
}

// TerminalTheme is a distinct palette used only inside the serial terminal
// modal to convey the feeling of entering a separate mode.
type TerminalTheme struct {
	// Semantic colors.
	Accent     color.Color // headings, borders
	AccentDim  color.Color // muted version of Accent
	Text       color.Color // body text
	TextDim    color.Color // secondary text
	Bg         color.Color // background
	Disconnect color.Color // "DEVICE DISCONNECTED" banner
	Prompt     color.Color // prompt marker / highlight

	// Pre-built styles.
	Banner     lipgloss.Style // ASCII banner text
	HeaderKey  lipgloss.Style // "Firmware"
	HeaderVal  lipgloss.Style // "0.95.1"
	Body       lipgloss.Style // scrollback default
	Dim        lipgloss.Style // muted scrollback lines
	Separator  lipgloss.Style // horizontal rule glyph style
	Border           lipgloss.Style // heavy-border foreground
	DisconnectBanner lipgloss.Style // red banner for disconnect
	Help             lipgloss.Style // footer hints
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

		Terminal: newTerminalTheme(),
	}
}

// newTerminalTheme builds the amber-on-dark palette used by the serial
// terminal modal.
func newTerminalTheme() *TerminalTheme {
	accent := lipgloss.Color("#FFB000")     // amber
	accentDim := lipgloss.Color("#8A5C00")  // dim amber
	text := lipgloss.Color("#FFD47F")       // warm text
	textDim := lipgloss.Color("#7A6140")    // faded text
	bg := lipgloss.Color("#0A0A0A")         // near-black
	disconnect := lipgloss.Color("#FF3030") // bright red
	prompt := lipgloss.Color("#FFD700")     // slightly brighter for prompt

	return &TerminalTheme{
		Accent:     accent,
		AccentDim:  accentDim,
		Text:       text,
		TextDim:    textDim,
		Bg:         bg,
		Disconnect: disconnect,
		Prompt:     prompt,

		Banner:    lipgloss.NewStyle().Bold(true).Foreground(accent),
		HeaderKey: lipgloss.NewStyle().Foreground(accentDim),
		HeaderVal: lipgloss.NewStyle().Bold(true).Foreground(text),
		Body:      lipgloss.NewStyle().Foreground(text),
		Dim:       lipgloss.NewStyle().Foreground(textDim),
		Separator: lipgloss.NewStyle().Foreground(accentDim),
		Border:    lipgloss.NewStyle().Foreground(accent),
		DisconnectBanner: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#000000")).
			Background(disconnect).
			Padding(0, 1),
		Help: lipgloss.NewStyle().Foreground(textDim),
	}
}
