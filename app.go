package main

import (
	"strings"

	"github.com/olivierpoupier/patch/fsview"
	"github.com/olivierpoupier/patch/tui"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type model struct {
	tabs          []tui.Tab
	activeTab     int
	focusContent  bool
	width         int
	height        int
	theme         *tui.Theme
	fsview        *fsview.Model
	fsOpen        bool
	fsReturnFocus bool
}

func newModel(theme *tui.Theme, tabs []tui.Tab) model {
	return model{
		tabs:   tabs,
		theme:  theme,
		fsview: fsview.New(theme),
	}
}

func (m model) Init() tea.Cmd {
	var cmds []tea.Cmd
	for _, t := range m.tabs {
		if cmd := t.Init(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	// Activate the initial tab.
	if len(m.tabs) > 0 {
		tab, cmd := m.tabs[m.activeTab].SetActive(true)
		m.tabs[m.activeTab] = tab
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return tea.Batch(cmds...)
}

func (m *model) switchTab(newIdx int) tea.Cmd {
	if newIdx == m.activeTab {
		return nil
	}
	var cmds []tea.Cmd
	tab, cmd := m.tabs[m.activeTab].SetActive(false)
	m.tabs[m.activeTab] = tab
	if cmd != nil {
		cmds = append(cmds, cmd)
	}
	m.activeTab = newIdx
	tab, cmd = m.tabs[m.activeTab].SetActive(true)
	m.tabs[m.activeTab] = tab
	if cmd != nil {
		cmds = append(cmds, cmd)
	}
	return tea.Batch(cmds...)
}

func (m *model) openFSView(msg tui.OpenFSViewMsg) tea.Cmd {
	m.fsReturnFocus = m.focusContent
	if m.focusContent {
		tab, _ := m.tabs[m.activeTab].SetFocused(false)
		m.tabs[m.activeTab] = tab
		m.focusContent = false
	}
	m.fsview.SetSize(m.width, m.height)
	cmd := m.fsview.Open(msg.Entries)
	m.fsOpen = true
	return cmd
}

func (m *model) closeFSView() tea.Cmd {
	m.fsview.Close()
	m.fsOpen = false
	if m.fsReturnFocus {
		tab, cmd := m.tabs[m.activeTab].SetFocused(true)
		m.tabs[m.activeTab] = tab
		m.focusContent = true
		return cmd
	}
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.fsview != nil {
			m.fsview.SetSize(msg.Width, msg.Height)
		}

	case tui.OpenFSViewMsg:
		cmd := m.openFSView(msg)
		return m, cmd

	case tui.CloseFSViewMsg:
		cmd := m.closeFSView()
		return m, cmd

	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		if m.fsOpen {
			fsm, cmd := m.fsview.Update(msg)
			m.fsview = fsm
			return m, cmd
		}

		if msg.String() == "q" {
			return m, tea.Quit
		}

		if m.focusContent {
			if msg.String() == "esc" {
				tab, cmd := m.tabs[m.activeTab].SetFocused(false)
				m.tabs[m.activeTab] = tab
				m.focusContent = false
				return m, cmd
			}
			tab, cmd := m.tabs[m.activeTab].Update(msg)
			m.tabs[m.activeTab] = tab
			return m, cmd
		}

		// Tab bar navigation.
		switch msg.String() {
		case "tab", "right", "l":
			newIdx := (m.activeTab + 1) % len(m.tabs)
			cmd := m.switchTab(newIdx)
			return m, cmd
		case "shift+tab", "left", "h":
			newIdx := (m.activeTab - 1 + len(m.tabs)) % len(m.tabs)
			cmd := m.switchTab(newIdx)
			return m, cmd
		case "enter":
			m.focusContent = true
			tab, cmd := m.tabs[m.activeTab].SetFocused(true)
			m.tabs[m.activeTab] = tab
			return m, cmd
		default:
			// Number keys: "1" -> index 0, "2" -> index 1, etc.
			s := msg.String()
			if len(s) == 1 && s[0] >= '1' && s[0] <= '9' {
				idx := int(s[0] - '1')
				if idx < len(m.tabs) {
					cmd := m.switchTab(idx)
					return m, cmd
				}
			}
		}
		return m, nil
	}

	// Non-key messages.
	var cmds []tea.Cmd
	if m.fsOpen {
		fsm, fsCmd := m.fsview.Update(msg)
		m.fsview = fsm
		if fsCmd != nil {
			cmds = append(cmds, fsCmd)
		}
	}
	// Broadcast to all tabs. Each tab ignores messages it doesn't own.
	for i, t := range m.tabs {
		updated, cmd := t.Update(msg)
		m.tabs[i] = updated
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return m, tea.Batch(cmds...)
}

func (m model) View() tea.View {
	if m.width == 0 {
		return tea.NewView("")
	}

	if m.fsOpen {
		v := tea.NewView(m.fsview.View(m.width, m.height))
		v.AltScreen = true
		return v
	}

	contentWidth := m.width - 2

	var renderedTabs []string
	for i, t := range m.tabs {
		if i == m.activeTab {
			renderedTabs = append(renderedTabs, m.theme.TabActive.Render(t.Name()))
		} else {
			renderedTabs = append(renderedTabs, m.theme.TabInactive.Render(t.Name()))
		}
	}

	tabRow := lipgloss.JoinHorizontal(lipgloss.Top, renderedTabs...)
	tabRowWidth := lipgloss.Width(tabRow)

	gapWidth := contentWidth - tabRowWidth
	var gap string
	if gapWidth > 1 {
		gap = m.theme.Border.Render(strings.Repeat("─", gapWidth-1) + "╮")
	} else if gapWidth == 1 {
		gap = m.theme.Border.Render("╮")
	}

	tabBar := lipgloss.JoinHorizontal(lipgloss.Bottom, tabRow, gap)

	// Tab bar is 3 lines (top border + text + gap line).
	// Content border: top padding(1) + bottom padding(1) + bottom border(1) = 3.
	const tabBarHeight = 3
	const contentBorderHeight = 3
	contentHeight := m.height - tabBarHeight - contentBorderHeight
	if contentHeight < 1 {
		contentHeight = 1
	}

	// Inner width = contentWidth minus left and right borders.
	innerWidth := contentWidth - 2
	content := m.tabs[m.activeTab].View(innerWidth, contentHeight)

	contentStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder(), false, true, true, true).
		BorderForeground(m.theme.Active).
		Padding(1, 0).
		Width(contentWidth).
		Height(contentHeight)

	renderedContent := contentStyle.Render(content)

	full := lipgloss.JoinVertical(lipgloss.Left, tabBar, renderedContent)

	v := tea.NewView(full)
	v.AltScreen = true
	return v
}
