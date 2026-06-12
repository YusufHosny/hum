package ui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/YusufHosny/hum/internal/config"
)

type IntroModel struct {
	config *config.AppConfig
	width  int
	height int

	input textinput.Model
}

func NewIntroModel(appConfig *config.AppConfig) IntroModel {
	ti := textinput.New()
	ti.Placeholder = "Username"
	ti.Focus()
	ti.CharLimit = 20
	ti.Width = 30

	return IntroModel{
		config: appConfig,
		input:  ti,
	}
}

func (m *IntroModel) Resize(w, h int) {
	m.width = w
	m.height = h
}

func (m IntroModel) Update(msg tea.Msg) (IntroModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			if m.input.Value() != "" {
				m.config.Username = m.input.Value()
				_ = config.SaveConfig(m.config)
				return m, func() tea.Msg { return NavigateMsg{ViewHome} }
			}
		}
	}

	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m IntroModel) View() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).Render("Hum")
	desc := "Hum is a pre-shared key encrypted peer to peer chat application written in go."
	
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(1, 2)

	content := lipgloss.JoinVertical(lipgloss.Center,
		title,
		desc,
		"",
		"Please enter a username to start:",
		m.input.View(),
	)

	return placeCentered(m.width, m.height, boxStyle.Render(content))
}