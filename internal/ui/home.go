package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/YusufHosny/hum/internal/config"
)

type HomeModel struct {
	config *config.AppConfig
	width  int
	height int

	inputs []textinput.Model
	focusIndex int

	sidebarWidth int
}

func NewHomeModel(appConfig *config.AppConfig) HomeModel {
	m := HomeModel{
		config:       appConfig,
		sidebarWidth: 25,
		inputs:       make([]textinput.Model, 2),
	}

	for i := range m.inputs {
		t := textinput.New()
		t.CursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
		t.CharLimit = 32

		switch i {
		case 0:
			t.Placeholder = "Channel Name"
			t.Focus()
			t.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
			t.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
		case 1:
			t.Placeholder = "Passkey"
			t.EchoMode = textinput.EchoPassword
			t.EchoCharacter = '•'
		}

		m.inputs[i] = t
	}

	return m
}

func (m *HomeModel) Resize(w, h int) {
	m.width = w
	m.height = h
}

func (m HomeModel) Update(msg tea.Msg) (HomeModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "shift+tab", "enter", "up", "down":
			s := msg.String()

			// Check if we submit
			if s == "enter" {
				if m.focusIndex == len(m.inputs) { // Connect
					return m, func() tea.Msg {
						return ConnectMsg{Channel: m.inputs[0].Value(), Passkey: m.inputs[1].Value()}
					}
				} else if m.focusIndex == len(m.inputs)+1+len(m.config.RecentChannels) { // Settings
					return m, func() tea.Msg { return NavigateMsg{ViewSettings} }
				} else if m.focusIndex > len(m.inputs) { // Recent channel
					idx := m.focusIndex - len(m.inputs) - 1
					if idx >= 0 && idx < len(m.config.RecentChannels) {
						m.inputs[0].SetValue(m.config.RecentChannels[idx])
						m.focusIndex = 1 // jump to passkey
						cmds = append(cmds, m.inputs[1].Focus())
					}
				}
			}

			// Cycle through fields
			if s == "up" || s == "shift+tab" {
				m.focusIndex--
			} else {
				m.focusIndex++
			}

			totalFocusable := len(m.inputs) + 1 + len(m.config.RecentChannels) + 1 // inputs + connect btn + recents + settings btn
			if m.focusIndex >= totalFocusable {
				m.focusIndex = 0
			} else if m.focusIndex < 0 {
				m.focusIndex = totalFocusable - 1
			}

			// Focus management
			for i := 0; i <= len(m.inputs)-1; i++ {
				if i == m.focusIndex {
					// Set focused state
					cmds = append(cmds, m.inputs[i].Focus())
					m.inputs[i].PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
					m.inputs[i].TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
					continue
				}
				// Remove focused state
				m.inputs[i].Blur()
				m.inputs[i].PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
				m.inputs[i].TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
			}

			return m, tea.Batch(cmds...)
		case "esc":
			return m, tea.Quit
		case "d":
			if m.focusIndex > len(m.inputs) && m.focusIndex < len(m.inputs)+1+len(m.config.RecentChannels) {
				idx := m.focusIndex - len(m.inputs) - 1
				if idx >= 0 && idx < len(m.config.RecentChannels) {
					// Remove the channel
					m.config.RecentChannels = append(m.config.RecentChannels[:idx], m.config.RecentChannels[idx+1:]...)
					_ = config.SaveConfig(m.config)
					// Adjust focus
					if m.focusIndex >= len(m.inputs)+1+len(m.config.RecentChannels) {
						m.focusIndex--
					}
				}
			}
		}
	}

	cmd := m.updateInputs(msg)
	return m, cmd
}

func (m *HomeModel) updateInputs(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd

	for i := range m.inputs {
		var cmd tea.Cmd
		m.inputs[i], cmd = m.inputs[i].Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return tea.Batch(cmds...)
}

func (m HomeModel) View() string {
	// Left Sidebar
	sidebarStyle := lipgloss.NewStyle().
		Width(m.sidebarWidth).
		Height(m.height).
		Border(lipgloss.NormalBorder(), false, true, false, false).
		BorderForeground(lipgloss.Color("240")).
		Padding(1, 2)

	var sb []string
	sb = append(sb, lipgloss.NewStyle().Bold(true).Render("Recent Channels"))
	sb = append(sb, lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("(Press 'd' to delete)"))
	sb = append(sb, "")

	if len(m.config.RecentChannels) == 0 {
		sb = append(sb, lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("No recent channels"))
	} else {
		for i, c := range m.config.RecentChannels {
			// Offset: 0=channel, 1=passkey, 2=connect, 3...N=recents
			if m.focusIndex == len(m.inputs)+1+i {
				sb = append(sb, lipgloss.NewStyle().Background(lipgloss.Color("63")).Foreground(lipgloss.Color("255")).Render(fmt.Sprintf("> # %s", c)))
			} else {
				sb = append(sb, fmt.Sprintf("  # %s", c))
			}
		}
	}

	// Bottom of sidebar
	settingsText := "[⚙ Settings]"
	if m.focusIndex == len(m.inputs)+1+len(m.config.RecentChannels) {
		settingsText = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true).Render(settingsText)
	}

	userSection := lipgloss.NewStyle().
		MarginTop(m.height - len(sb) - 5).
		Render(fmt.Sprintf("%s\n%s", m.config.Username, settingsText))
	
	sb = append(sb, userSection)

	leftCol := sidebarStyle.Render(strings.Join(sb, "\n"))

	// Center pane
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).Render("Hum")
	desc := "Enter a channel name and passkey to start"
	
	var inputs []string
	for i := range m.inputs {
		inputs = append(inputs, m.inputs[i].View())
	}

	submitBtn := "[ Connect ]"
	if m.focusIndex == len(m.inputs) {
		submitBtn = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true).Render(submitBtn)
	}

	centerContent := lipgloss.JoinVertical(lipgloss.Center,
		title,
		desc,
		"",
		inputs[0],
		inputs[1],
		"",
		submitBtn,
	)
	
	centerCol := placeCentered(m.width-m.sidebarWidth-2, m.height, centerContent)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftCol, centerCol)
}