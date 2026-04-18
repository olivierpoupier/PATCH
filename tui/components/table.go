package components

import (
	"strings"

	"github.com/olivierpoupier/patch/tui"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
)

// Column defines a table column with a title and proportional width.
type Column struct {
	Title   string
	Percent int // percentage of available width (0-100)
}

// DeviceTable renders a bordered table with cursor-based row selection
// and proportional column widths.
type DeviceTable struct {
	columns  []Column
	rows     [][]string
	cursor   int
	theme    *tui.Theme
	width    int
	focused  bool
	colWidth []int // computed column widths
}

// NewDeviceTable creates a new device table with the given columns.
func NewDeviceTable(theme *tui.Theme, columns []Column) DeviceTable {
	return DeviceTable{
		columns: columns,
		theme:   theme,
	}
}

// SetRows sets the table data rows.
func (t DeviceTable) SetRows(rows [][]string) DeviceTable {
	t.rows = rows
	if t.cursor >= len(t.rows) && len(t.rows) > 0 {
		t.cursor = len(t.rows) - 1
	}
	return t
}

// SetWidth sets the available width for the table.
func (t DeviceTable) SetWidth(w int) DeviceTable {
	t.width = w
	t.computeColumnWidths()
	return t
}

// SetFocused sets whether the table cursor should be highlighted.
func (t DeviceTable) SetFocused(focused bool) DeviceTable {
	t.focused = focused
	return t
}

// CursorUp moves the cursor up one row.
func (t DeviceTable) CursorUp() DeviceTable {
	if t.cursor > 0 {
		t.cursor--
	}
	return t
}

// CursorDown moves the cursor down one row.
func (t DeviceTable) CursorDown() DeviceTable {
	if t.cursor < len(t.rows)-1 {
		t.cursor++
	}
	return t
}

// SetCursor sets the cursor position directly.
func (t DeviceTable) SetCursor(pos int) DeviceTable {
	t.cursor = pos
	if t.cursor < 0 {
		t.cursor = 0
	}
	if t.cursor >= len(t.rows) && len(t.rows) > 0 {
		t.cursor = len(t.rows) - 1
	}
	return t
}

// Cursor returns the current cursor position.
func (t DeviceTable) Cursor() int {
	return t.cursor
}

// RowCount returns the number of data rows.
func (t DeviceTable) RowCount() int {
	return len(t.rows)
}

// SelectedRow returns the currently selected row data, or nil if no rows.
func (t DeviceTable) SelectedRow() []string {
	if len(t.rows) == 0 || t.cursor >= len(t.rows) {
		return nil
	}
	return t.rows[t.cursor]
}

// StyleFunc is an optional per-cell style override. If set, it is called for
// each cell with (row, col, colWidth) and should return the style to use.
// row == -1 means header row.
type StyleFunc func(row, col, colWidth int) lipgloss.Style

// View renders the table as a string. The optional styleFn allows per-cell overrides.
func (t DeviceTable) View(styleFn StyleFunc) string {
	if len(t.columns) == 0 {
		return ""
	}

	const tableMargin = 2
	tableWidth := t.width - tableMargin*2

	t.computeColumnWidths()

	// Extract headers.
	headers := make([]string, len(t.columns))
	for i, col := range t.columns {
		headers[i] = col.Title
	}

	cursor := t.cursor
	colWidths := t.colWidth
	theme := t.theme
	focused := t.focused

	tb := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(theme.Active)).
		Width(tableWidth).
		Headers(headers...).
		Rows(t.rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			w := 0
			if col < len(colWidths) {
				w = colWidths[col]
			}

			if styleFn != nil {
				headerRow := row
				if row == table.HeaderRow {
					headerRow = -1
				}
				return styleFn(headerRow, col, w)
			}

			if row == table.HeaderRow {
				return theme.TableHeader.Width(w)
			}
			if focused && row == cursor {
				return theme.Selected.Width(w)
			}
			return theme.TableCell.Width(w)
		})

	var b strings.Builder
	for _, line := range strings.Split(tb.String(), "\n") {
		b.WriteString("  ")
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

func (t *DeviceTable) computeColumnWidths() {
	const tableMargin = 2
	const borderOverhead = 5 // left(1) + right(1) + N-1 column separators
	const cellPadding = 2    // Padding(0, 1) = 1 left + 1 right

	tableWidth := t.width - tableMargin*2
	available := tableWidth - borderOverhead
	if available < 40 {
		available = 40
	}

	// Adjust border overhead for actual column count.
	actualBorderOverhead := len(t.columns) + 1
	available = tableWidth - actualBorderOverhead
	if available < 40 {
		available = 40
	}

	t.colWidth = make([]int, len(t.columns))
	for i, col := range t.columns {
		t.colWidth[i] = available * col.Percent / 100
	}

	// Give remainder to last column.
	if len(t.colWidth) > 0 {
		used := 0
		for _, w := range t.colWidth[:len(t.colWidth)-1] {
			used += w
		}
		t.colWidth[len(t.colWidth)-1] = available - used
	}

	// Enforce minimums.
	for i := range t.colWidth {
		if t.colWidth[i] < cellPadding+4 {
			t.colWidth[i] = cellPadding + 4
		}
	}
}

// ColumnWidth returns the computed width for a given column index.
func (t DeviceTable) ColumnWidth(col int) int {
	if col < 0 || col >= len(t.colWidth) {
		return 0
	}
	return t.colWidth[col]
}

// ContentWidth returns the usable content width for a column (minus cell padding).
func (t DeviceTable) ContentWidth(col int) int {
	w := t.ColumnWidth(col)
	if w <= 2 {
		return 1
	}
	return w - 2 // subtract padding
}
