package components

import (
	"fmt"
	"strings"

	"github.com/olivierpoupier/patch/tui"
)

// KeyBinding represents a keyboard shortcut and its description for help text.
type KeyBinding struct {
	Keys        string // e.g. "enter", "↑↓/jk"
	Description string // e.g. "connect", "navigate"
}

// RenderFooter renders keybinding help text in a consistent format.
func RenderFooter(theme *tui.Theme, bindings []KeyBinding, width int) string {
	var parts []string
	for _, b := range bindings {
		parts = append(parts, b.Keys+" "+b.Description)
	}
	help := "  " + strings.Join(parts, "  ")
	_ = width
	return theme.Help.Render(help)
}

// RenderError renders an error message with consistent styling.
func RenderError(theme *tui.Theme, err error, width int) string {
	_ = width
	return theme.ErrorText.Render(fmt.Sprintf("  Error: %v", err))
}
