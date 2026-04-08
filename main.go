package main

import (
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var (
	tabNames = []string{"USB", "Bluetooth"}

	tabContents = []string{
		"Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod\ntempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim\nveniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea\ncommodo consequat. Duis aute irure dolor in reprehenderit in voluptate\nvelit esse cillum dolore eu fugiat nulla pariatur.",
		"Sed ut perspiciatis unde omnis iste natus error sit voluptatem\naccusantium doloremque laudantium, totam rem aperiam, eaque ipsa quae\nab illo inventore veritatis et quasi architecto beatae vitae dicta sunt\nexplicabo. Nemo enim ipsam voluptatem quia voluptas sit aspernatur aut\nodit aut fugit, sed quia consequuntur magni dolores eos.",
	}

	activeColor   = lipgloss.Color("#04B575")
	inactiveColor = lipgloss.Color("#888888")
)

type model struct {
	activeTab int
	width     int
	height    int
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab", "right", "l":
			m.activeTab = (m.activeTab + 1) % len(tabNames)
		case "shift+tab", "left", "h":
			m.activeTab = (m.activeTab - 1 + len(tabNames)) % len(tabNames)
		case "1":
			m.activeTab = 0
		case "2":
			m.activeTab = 1
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

func (m model) View() tea.View {
	if m.width == 0 {
		return tea.NewView("")
	}

	contentWidth := m.width - 2

	activeTabStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(activeColor).
		Border(lipgloss.RoundedBorder(), true, true, false, true).
		BorderForeground(activeColor).
		Padding(0, 1)

	inactiveTabStyle := lipgloss.NewStyle().
		Foreground(inactiveColor).
		Border(lipgloss.RoundedBorder(), true, true, true, true).
		BorderForeground(inactiveColor).
		Padding(0, 1)

	var renderedTabs []string
	for i, name := range tabNames {
		if i == m.activeTab {
			renderedTabs = append(renderedTabs, activeTabStyle.Render(name))
		} else {
			renderedTabs = append(renderedTabs, inactiveTabStyle.Render(name))
		}
	}

	tabRow := lipgloss.JoinHorizontal(lipgloss.Top, renderedTabs...)
	tabRowWidth := lipgloss.Width(tabRow)

	gapWidth := contentWidth - tabRowWidth
	var gap string
	if gapWidth > 1 {
		gapStyle := lipgloss.NewStyle().Foreground(activeColor)
		gap = gapStyle.Render(strings.Repeat("─", gapWidth-1) + "╮")
	} else if gapWidth == 1 {
		gapStyle := lipgloss.NewStyle().Foreground(activeColor)
		gap = gapStyle.Render("╮")
	}

	tabBar := lipgloss.JoinHorizontal(lipgloss.Bottom, tabRow, gap)

	contentStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder(), false, true, true, true).
		BorderForeground(activeColor).
		Padding(1, 2).
		Width(contentWidth)

	content := contentStyle.Render(tabContents[m.activeTab])

	full := lipgloss.JoinVertical(lipgloss.Left, tabBar, content)

	v := tea.NewView(full)
	v.AltScreen = true
	return v
}

func main() {
	p := tea.NewProgram(model{})
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
