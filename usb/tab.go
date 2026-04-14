package usb

import (
	"fmt"
	"strings"

	"github.com/olivierpoupier/patch/tui"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
)

// Model is the USB tab's Bubbletea model.
type Model struct {
	scanner     *USBScanner
	groups      []PortGroup
	nodes       []TreeNode
	collapsed   map[string]bool
	cursor      int
	err         error
	width       int
	height      int
	active      bool
	focused     bool
	initialized bool
	viewport    viewport.Model
	tableView   bool
	tableRows   []TableRow
	tableCursor int
}

// New creates a new USB tab model.
func New() *Model {
	return &Model{
		collapsed: make(map[string]bool),
	}
}

// Name returns the tab display name.
func (m *Model) Name() string { return "USB" }

// Init returns the initial command to set up the scanner.
func (m *Model) Init() tea.Cmd {
	return initScanner()
}

// Update handles messages and returns the updated tab.
func (m *Model) Update(msg tea.Msg) (tui.Tab, tea.Cmd) {
	switch msg := msg.(type) {
	case initMsg:
		m.scanner = msg.scanner
		m.initialized = true
		return m, tea.Batch(fetchData(m.scanner), scheduleRefreshTick())

	case dataMsg:
		m.groups = msg.groups
		m.rebuildNodes()
		m.rebuildTableRows()
		m.err = nil
		return m, nil

	case refreshTickMsg:
		if !m.active || m.scanner == nil {
			return m, nil
		}
		return m, tea.Batch(fetchData(m.scanner), scheduleRefreshTick())

	case errorMsg:
		m.err = msg.err
		if !m.initialized {
			return m, nil
		}
		return m, nil

	case tea.KeyPressMsg:
		if !m.focused || !m.initialized {
			return m, nil
		}
		switch msg.String() {
		case "t":
			m.tableView = !m.tableView
			m.viewport.SetYOffset(0)
		case "up", "k":
			if m.tableView {
				if m.tableCursor > 0 {
					m.tableCursor--
				}
			} else {
				if m.cursor > 0 {
					m.cursor--
				}
			}
		case "down", "j":
			if m.tableView {
				if m.tableCursor < len(m.tableRows)-1 {
					m.tableCursor++
				}
			} else {
				if m.cursor < len(m.nodes)-1 {
					m.cursor++
				}
			}
		case "enter", " ":
			if !m.tableView && m.cursor < len(m.nodes) {
				node := m.nodes[m.cursor]
				if node.HasChildren {
					m.collapsed[node.NodeID] = !m.collapsed[node.NodeID]
					m.rebuildNodes()
					if m.cursor >= len(m.nodes) {
						m.cursor = len(m.nodes) - 1
					}
				}
			}
		}
	}
	return m, nil
}

// SetFocused is called when the tab content gains or loses keyboard focus.
func (m *Model) SetFocused(focused bool) (tui.Tab, tea.Cmd) {
	m.focused = focused
	return m, nil
}

// SetActive is called when the tab becomes the visible tab or is switched away from.
func (m *Model) SetActive(active bool) (tui.Tab, tea.Cmd) {
	m.active = active
	if !m.initialized || m.scanner == nil {
		return m, nil
	}
	if active {
		return m, tea.Batch(fetchData(m.scanner), scheduleRefreshTick())
	}
	return m, nil
}

// View renders the tab content.
func (m *Model) View(width, height int) string {
	m.width = width
	m.height = height

	if !m.initialized {
		return "  Initializing USB..."
	}
	if m.err != nil && m.scanner == nil {
		return fmt.Sprintf("  USB error: %v", m.err)
	}

	// Header.
	var header strings.Builder
	m.renderHeader(&header)
	headerStr := header.String()

	// Footer.
	var footer strings.Builder
	if m.err != nil {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4444"))
		footer.WriteString(errStyle.Render(fmt.Sprintf("  Error: %v", m.err)))
		footer.WriteString("\n")
	}
	helpStyle := lipgloss.NewStyle().Foreground(tui.InactiveColor)
	if m.tableView {
		footer.WriteString(helpStyle.Render("  ↑↓/jk navigate  t tree view  esc back"))
	} else {
		footer.WriteString(helpStyle.Render("  ↑↓/jk navigate  enter collapse/expand  t table view  esc back"))
	}
	footerStr := footer.String()

	// Body height.
	headerHeight := lipgloss.Height(headerStr)
	footerHeight := lipgloss.Height(footerStr)
	bodyHeight := height - headerHeight - footerHeight - 1
	if bodyHeight < 3 {
		bodyHeight = 3
	}

	// Render body.
	tableWidth := width - 1 // reserve for scrollbar
	var body strings.Builder
	if m.tableView {
		m.renderTable(&body, tableWidth)
	} else {
		m.renderTree(&body, tableWidth)
	}
	bodyStr := body.String()
	contentLines := lipgloss.Height(bodyStr)

	// Pad below so the last row can scroll up.
	for i := 0; i < bodyHeight/2; i++ {
		bodyStr += "\n"
	}

	m.viewport.SetWidth(tableWidth)
	m.viewport.SetHeight(bodyHeight)
	m.viewport.SetContent(bodyStr)

	// Auto-scroll to keep cursor visible.
	activeCursor := m.cursor
	activeLen := len(m.nodes)
	if m.tableView {
		activeCursor = m.tableCursor
		activeLen = len(m.tableRows)
	}
	if activeLen > 0 {
		rowLine := activeCursor
		vpHeight := m.viewport.Height()
		margin := vpHeight / 5
		if margin < 1 {
			margin = 1
		}
		yOffset := m.viewport.YOffset()
		if rowLine < yOffset+margin {
			m.viewport.SetYOffset(rowLine - margin)
		} else if rowLine >= yOffset+vpHeight-margin {
			m.viewport.SetYOffset(rowLine - vpHeight + margin + 1)
		}
		if m.viewport.YOffset() < 0 {
			m.viewport.SetYOffset(0)
		}
	}

	vpView := m.viewport.View()
	vpView = m.overlayScrollbar(vpView, bodyHeight, contentLines)

	return headerStr + "\n" + vpView + "\n" + footerStr
}

func (m *Model) renderHeader(b *strings.Builder) {
	labelStyle := lipgloss.NewStyle().Bold(true).Foreground(tui.ActiveColor)
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))

	total := countDevices(m.groups)
	ports := len(m.groups)

	viewLabel := "Tree"
	if m.tableView {
		viewLabel = "Table"
	}

	b.WriteString(fmt.Sprintf("  %s %s    %s %s    %s %s\n",
		labelStyle.Render("Devices:"),
		valueStyle.Render(fmt.Sprintf("%d", total)),
		labelStyle.Render("Ports:"),
		valueStyle.Render(fmt.Sprintf("%d", ports)),
		labelStyle.Render("View:"),
		valueStyle.Render(viewLabel),
	))
}

func (m *Model) renderTree(b *strings.Builder, width int) {
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(tui.ActiveColor)
	selectedBg := lipgloss.NewStyle().Background(tui.ActiveColor).Foreground(lipgloss.Color("#000000"))
	normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	dimStyle := lipgloss.NewStyle().Foreground(tui.InactiveColor)

	typeW := 12

	for i, node := range m.nodes {
		isSelected := m.focused && i == m.cursor

		if node.IsHeader {
			text := "  " + node.Label
			if isSelected {
				b.WriteString(selectedBg.Render(padRight(text, width)))
			} else {
				b.WriteString(headerStyle.Render(text))
			}
			b.WriteString("\n")
			continue
		}

		// Build tree prefix.
		var prefix strings.Builder
		prefix.WriteString("  ") // left margin

		for d := 0; d < node.Depth-1; d++ {
			if d < len(node.Ancestors) && node.Ancestors[d] {
				prefix.WriteString("   ")
			} else {
				prefix.WriteString("│  ")
			}
		}

		if node.Depth > 0 {
			if node.IsLast {
				prefix.WriteString("└── ")
			} else {
				prefix.WriteString("├── ")
			}
		}

		// Collapse indicator.
		if node.HasChildren {
			if node.Collapsed {
				prefix.WriteString("▶ ")
			} else {
				prefix.WriteString("▼ ")
			}
		}

		prefixStr := prefix.String()
		prefixWidth := lipgloss.Width(prefixStr)
		nameW := width - prefixWidth - typeW - 2
		if nameW < 10 {
			nameW = 10
		}

		nameText := node.Label
		if node.IsBus {
			nameText = node.Label
		}
		if len([]rune(nameText)) > nameW {
			nameText = truncateText(nameText, nameW)
		}

		if isSelected {
			content := padRight(nameText, nameW) + "  " + padRight(node.TypeLabel, typeW)
			b.WriteString(prefixStr)
			b.WriteString(selectedBg.Render(padRight(content, width-prefixWidth)))
		} else {
			style := normalStyle
			if node.IsBuiltIn || node.IsBus {
				style = dimStyle
			}
			b.WriteString(prefixStr)
			b.WriteString(style.Render(padRight(nameText, nameW)))
			b.WriteString("  ")
			b.WriteString(dimStyle.Render(padRight(node.TypeLabel, typeW)))
		}
		b.WriteString("\n")
	}
}

func (m *Model) rebuildNodes() {
	m.nodes = flattenTree(m.groups, m.collapsed)
	if m.cursor >= len(m.nodes) && len(m.nodes) > 0 {
		m.cursor = len(m.nodes) - 1
	}
}

func (m *Model) rebuildTableRows() {
	m.tableRows = collectTableRows(m.groups)
	if m.tableCursor >= len(m.tableRows) && len(m.tableRows) > 0 {
		m.tableCursor = len(m.tableRows) - 1
	}
}

func (m *Model) renderTable(b *strings.Builder, width int) {
	type rowData struct {
		name, typ, port, storage, action string
	}
	var rowInfos []rowData

	if len(m.tableRows) == 0 {
		rowInfos = []rowData{{"No peripherals detected", "", "", "", ""}}
	} else {
		for _, r := range m.tableRows {
			rowInfos = append(rowInfos, rowData{r.Name, r.Type, r.Port, r.Storage, ""})
		}
	}

	headers := [5]string{"Name", "Type", "Port", "Storage", "Action"}

	const tableMargin = 2
	tableWidth := width - tableMargin*2
	const borderOverhead = 6
	const cellPadding = 2

	available := tableWidth - borderOverhead
	if available < 40 {
		available = 40
	}

	nameW := available * 28 / 100
	typeW := available * 12 / 100
	portW := available * 15 / 100
	storageW := available * 30 / 100
	actionW := available - nameW - typeW - portW - storageW

	if nameW < 10+cellPadding {
		nameW = 10 + cellPadding
	}
	nameContentW := nameW - cellPadding

	var rows [][]string
	for _, r := range rowInfos {
		name := r.name
		if len([]rune(name)) > nameContentW {
			name = truncateText(name, nameContentW)
		}
		rows = append(rows, []string{name, r.typ, r.port, r.storage, r.action})
	}

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(tui.ActiveColor).Padding(0, 1)
	selectedStyle := lipgloss.NewStyle().Background(tui.ActiveColor).Foreground(lipgloss.Color("#000000")).Padding(0, 1)
	normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Padding(0, 1)

	cursor := m.tableCursor
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(tui.ActiveColor)).
		Width(tableWidth).
		Headers(headers[0], headers[1], headers[2], headers[3], headers[4]).
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			var w int
			switch col {
			case 0:
				w = nameW
			case 1:
				w = typeW
			case 2:
				w = portW
			case 3:
				w = storageW
			default:
				w = actionW
			}

			if row == table.HeaderRow {
				return headerStyle.Width(w)
			}

			if m.focused && row == cursor {
				return selectedStyle.Width(w)
			}

			base := normalStyle.Width(w)
			if len(m.tableRows) == 0 {
				return base.Foreground(tui.InactiveColor)
			}
			return base
		})

	for _, line := range strings.Split(t.String(), "\n") {
		b.WriteString("  ")
		b.WriteString(line)
		b.WriteString("\n")
	}
}

// overlayScrollbar renders a scrollbar track on the right edge of the viewport.
func (m *Model) overlayScrollbar(vpView string, vpHeight int, contentLines int) string {
	if contentLines <= vpHeight {
		return vpView
	}

	trackStyle := lipgloss.NewStyle().Foreground(tui.InactiveColor)
	thumbStyle := lipgloss.NewStyle().Foreground(tui.ActiveColor)

	thumbSize := vpHeight * vpHeight / contentLines
	if thumbSize < 1 {
		thumbSize = 1
	}
	scrollable := contentLines - vpHeight
	if scrollable < 0 {
		scrollable = 0
	}
	yOffset := m.viewport.YOffset()
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
