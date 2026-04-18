---
name: patch-components
description: API reference for PATCH TUI shared components (tui/components/ and tui/theme.go). Auto-apply whenever working with UI layout, rendering, ScrollableView, DeviceTable, headers, footers, text utilities, the Theme system, or View() composition in PATCH. Use this even for simple edits — it prevents common mistakes like forgetting StyleFunc, using wrong color types, or miscalculating layout heights.
---

# PATCH Component Library Reference

Quick-reference for the shared UI primitives in `tui/components/` and `tui/theme.go`. Every tab uses these — prefer them over raw lipgloss/viewport/table code.

---

## ScrollableView (`tui/components/scrollview.go`)

Wraps `viewport.Model` with a scrollbar overlay. **Never create raw viewports in tabs — use this instead.**

```go
// Construction
sv := components.NewScrollableView(theme)  // theme is *tui.Theme

// Configuration (value-receiver, returns new copy)
sv = sv.SetContent(renderedString)
sv = sv.SetSize(width, height)

// Scrolling
sv = sv.ScrollToLine(lineNumber)
sv = sv.SetYOffset(offset)
offset := sv.YOffset()

// Querying
h := sv.Height()

// Rendering
output := sv.View()  // content + scrollbar overlay
```

All setters return a new `ScrollableView` (value semantics) — always reassign: `m.scroll = m.scroll.SetContent(...)`.

---

## DeviceTable (`tui/components/table.go`)

Proportional-width table with cursor navigation and custom cell styling.

```go
// Types
type Column struct {
    Title   string
    Percent int      // proportional width (not float64)
}

type StyleFunc func(row, col, colWidth int) lipgloss.Style

// Construction
dt := components.NewDeviceTable(theme, []components.Column{
    {Title: "Name",   Percent: 40},
    {Title: "Status", Percent: 30},
    {Title: "Type",   Percent: 30},
})

// Configuration (value semantics)
dt = dt.SetRows([][]string{{"Keyboard", "Connected", "HID"}})
dt = dt.SetWidth(terminalWidth)
dt = dt.SetFocused(true)

// Navigation
dt = dt.CursorUp()
dt = dt.CursorDown()
dt = dt.SetCursor(index)
pos := dt.Cursor()
count := dt.RowCount()
row := dt.SelectedRow()  // []string of current row

// Column sizing queries
colW := dt.ColumnWidth(colIndex)    // full column width
contentW := dt.ContentWidth(colIndex) // width minus padding

// Rendering — View() requires a StyleFunc
output := dt.View(func(row, col, colWidth int) lipgloss.Style {
    return lipgloss.NewStyle().Width(colWidth)
})
```

**Important:** `View()` takes a `StyleFunc` — don't call it without one.

---

## StatusHeader (`tui/components/header.go`)

Renders rows of labeled key-value pairs.

```go
type HeaderField struct {
    Label string
    Value string
    Color color.Color  // image/color.Color, NOT lipgloss.Color
}

// Render rows of field pairs: "Label: Value  Label: Value"
header := components.RenderHeader(theme, [][]HeaderField{
    {
        {Label: "Name", Value: device.Name, Color: theme.Active},
        {Label: "Status", Value: "Connected", Color: theme.Active},
    },
    {
        {Label: "Address", Value: device.Address, Color: theme.Text},
    },
}, width)
```

Each inner slice is one row. Fields are spaced evenly across the width.

---

## HelpFooter (`tui/components/footer.go`)

Renders a key-binding help bar and error messages.

```go
type KeyBinding struct {
    Keys        string  // e.g. "j/k", "enter", "q"
    Description string  // e.g. "navigate", "select", "quit"
}

footer := components.RenderFooter(theme, []KeyBinding{
    {Keys: "j/k", Description: "navigate"},
    {Keys: "enter", Description: "select"},
    {Keys: "q", Description: "quit"},
}, width)

errorLine := components.RenderError(theme, err, width)
```

---

## Text Utilities (`tui/components/text.go`)

```go
s := components.TruncateText("long string", maxWidth)           // ellipsis truncation
s := components.PadRight("short", targetWidth)                   // right-pad with spaces
s := components.ScrollText("long string", visibleWidth, offset)  // marquee/sliding window
```

Use `TruncateText` before rendering in bordered panels — never rely on auto-wrap.

---

## Theme (`tui/theme.go`)

Constructed once, passed to every tab and component via `*tui.Theme`.

```go
theme := tui.NewTheme()  // returns *Theme with defaults
```

### Semantic Colors (`color.Color`)

| Field      | Usage                          |
|------------|--------------------------------|
| `Active`   | Connected/enabled states       |
| `Inactive` | Disconnected/disabled states   |
| `Error`    | Error text and indicators      |
| `Warning`  | Warning states                 |
| `Text`     | Primary text                   |
| `TextDim`  | Secondary/muted text           |
| `BgDark`   | Dark background for panels     |

### Pre-built Styles (`lipgloss.Style`)

| Field         | Usage                              |
|---------------|------------------------------------|
| `Label`       | Field labels in headers            |
| `Value`       | Field values in headers            |
| `Dim`         | De-emphasized text                 |
| `ErrorText`   | Error messages                     |
| `Help`        | Help bar text                      |
| `TableHeader` | Table column headers               |
| `TableCell`   | Table body cells                   |
| `Selected`    | Currently selected/highlighted row |
| `TabActive`   | Active tab in tab bar              |
| `TabInactive` | Inactive tab in tab bar            |
| `Border`      | Border styling for panels          |

---

## Standard View() Layout Pattern

Every tab's `View()` method should follow this structure:

```go
func (m Model) View() string {
    header := components.RenderHeader(m.theme, m.headerFields(), m.width)
    footer := components.RenderFooter(m.theme, m.keyBindings(), m.width)

    bodyHeight := m.height - lipgloss.Height(header) - lipgloss.Height(footer)
    m.scroll = m.scroll.SetSize(m.width, bodyHeight)

    return header + "\n" + m.scroll.View() + "\n" + footer
}
```

Calculate available space **before** rendering the body — never after.

---

## Bubbletea Layout Golden Rules

1. **Account for borders.** Bordered panels consume 2 lines (top + bottom) and 2 columns (left + right). Subtract these before passing dimensions to child components.

2. **Never auto-wrap in bordered panels.** Always truncate text explicitly with `TruncateText()`. Auto-wrap inside a bordered container causes unpredictable height, breaking layout math.

3. **Calculate space before rendering.** Compute `bodyHeight` from known header/footer heights, then size the body to fit — not the other way around.

4. **Verify vertical sums.** When composing sections vertically, ensure `header + body + footer == total`. Off-by-one errors cause scrollbar jitter and content clipping.
