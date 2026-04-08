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

	usbContent = "USB device management coming soon..."

	activeColor   = lipgloss.Color("#04B575")
	inactiveColor = lipgloss.Color("#888888")
)

type model struct {
	activeTab    int
	focusContent bool
	width        int
	height       int

	bluetoothTab bluetoothModel
}

func (m model) Init() tea.Cmd {
	return m.bluetoothTab.Init()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyPressMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		if m.focusContent {
			if msg.String() == "esc" {
				var cmd tea.Cmd
				m.bluetoothTab, cmd = m.bluetoothTab.SetFocused(false)
				m.focusContent = false
				return m, cmd
			}
			// Delegate to the active tab
			if m.activeTab == 1 {
				var cmd tea.Cmd
				m.bluetoothTab, cmd = m.bluetoothTab.Update(msg)
				return m, cmd
			}
			return m, nil
		}

		// Tab bar navigation
		switch msg.String() {
		case "tab", "right", "l":
			m.activeTab = (m.activeTab + 1) % len(tabNames)
		case "shift+tab", "left", "h":
			m.activeTab = (m.activeTab - 1 + len(tabNames)) % len(tabNames)
		case "1":
			m.activeTab = 0
		case "2":
			m.activeTab = 1
		case "enter":
			m.focusContent = true
			if m.activeTab == 1 {
				var cmd tea.Cmd
				m.bluetoothTab, cmd = m.bluetoothTab.SetFocused(true)
				return m, cmd
			}
		}
		return m, nil
	}

	// Always let bluetooth model handle its own async messages
	var cmd tea.Cmd
	m.bluetoothTab, cmd = m.bluetoothTab.Update(msg)
	return m, cmd
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

	// Content
	var content string
	switch m.activeTab {
	case 0:
		content = usbContent
	case 1:
		content = m.bluetoothTab.View(contentWidth)
	}

	contentStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder(), false, true, true, true).
		BorderForeground(activeColor).
		Padding(1, 0).
		Width(contentWidth)

	renderedContent := contentStyle.Render(content)

	full := lipgloss.JoinVertical(lipgloss.Left, tabBar, renderedContent)

	v := tea.NewView(full)
	v.AltScreen = true
	return v
}

func main() {
	m := model{
		bluetoothTab: newBluetoothModel(),
	}
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
