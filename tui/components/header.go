package components

import (
	"image/color"
	"strings"

	"github.com/olivierpoupier/patch/tui"

	"charm.land/lipgloss/v2"
)

// HeaderField represents a single label-value pair in the header.
type HeaderField struct {
	Label string
	Value string
	Color color.Color // Optional override color for the value (nil = use theme.Value)
}

// RenderHeader renders rows of label-value pairs with consistent styling.
// Each inner slice represents one row of fields.
func RenderHeader(theme *tui.Theme, rows [][]HeaderField, width int) string {
	var b strings.Builder
	for _, row := range rows {
		b.WriteString("  ")
		for i, field := range row {
			if i > 0 {
				b.WriteString("    ")
			}
			b.WriteString(theme.Label.Render(field.Label + ":"))
			b.WriteString(" ")
			if field.Color != nil {
				b.WriteString(lipgloss.NewStyle().Foreground(field.Color).Render(field.Value))
			} else {
				b.WriteString(theme.Value.Render(field.Value))
			}
		}
		b.WriteString("\n")
	}
	_ = width
	return b.String()
}
