package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/YusufHosny/hum/internal/config"
)

type SettingsModel struct {
	config *config.AppConfig
	width  int
	height int

	inputs []textinput.Model
	focusIndex int
}

func NewSettingsModel(appConfig *config.AppConfig) SettingsModel {
	m := SettingsModel{
		config: appConfig,
		inputs: make([]textinput.Model, 6),
	}

	for i := range m.inputs {
		t := textinput.New()
		t.CursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
		t.CharLimit = 128

		switch i {
		case 0:
			t.Placeholder = "Username"
			t.Focus()
			t.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
			t.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
		case 1:
			t.Placeholder = "Signaling URL"
		case 2:
			t.Placeholder = "STUN Servers (comma separated)"
		case 3:
			t.Placeholder = "Input Volume (0.0-1.0)"
		case 4:
			t.Placeholder = "Output Volume (0.0-1.0)"
		case 5:
			t.Placeholder = "Voice Threshold (RMS ~0.05)"
		}

		m.inputs[i] = t
	}

	m.loadValues()

	return m
}

func (m *SettingsModel) loadValues() {
	m.inputs[0].SetValue(m.config.Username)
	m.inputs[1].SetValue(m.config.SignalingURL)
	m.inputs[2].SetValue(strings.Join(m.config.STUNServers, ","))
	m.inputs[3].SetValue(fmt.Sprintf("%v", m.config.InputVolume))
	m.inputs[4].SetValue(fmt.Sprintf("%v", m.config.OutputVolume))
	m.inputs[5].SetValue(fmt.Sprintf("%v", m.config.VoiceThreshold))
}

func (m *SettingsModel) Resize(w, h int) {
	m.width = w
	m.height = h
}

func (m SettingsModel) Update(msg tea.Msg) (SettingsModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "shift+tab", "enter", "up", "down":
			s := msg.String()

			// Check if we submit
			if s == "enter" && m.focusIndex == len(m.inputs) {
				m.saveValues()
				return m, func() tea.Msg { return NavigateMsg{ViewHome} }
			} else if s == "enter" && m.focusIndex == len(m.inputs)+1 {
				return m, func() tea.Msg { return NavigateMsg{ViewHome} }
			}

			// Cycle through fields
			if s == "up" || s == "shift+tab" {
				m.focusIndex--
			} else {
				m.focusIndex++
			}

			if m.focusIndex > len(m.inputs)+1 {
				m.focusIndex = 0
			} else if m.focusIndex < 0 {
				m.focusIndex = len(m.inputs)+1
			}

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
			return m, func() tea.Msg { return NavigateMsg{ViewHome} }
		}
	}

	cmd := m.updateInputs(msg)
	return m, cmd
}

func (m *SettingsModel) updateInputs(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd

	for i := range m.inputs {
		var cmd tea.Cmd
		m.inputs[i], cmd = m.inputs[i].Update(msg)
		cmds = append(cmds, cmd)
	}

	return tea.Batch(cmds...)
}

func (m *SettingsModel) saveValues() {
	m.config.Username = m.inputs[0].Value()
	m.config.SignalingURL = m.inputs[1].Value()
	
	stunServers := strings.Split(m.inputs[2].Value(), ",")
	var cleaned []string
	for _, s := range stunServers {
		trimmed := strings.TrimSpace(s)
		if trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	m.config.STUNServers = cleaned

	fmt.Sscanf(m.inputs[3].Value(), "%f", &m.config.InputVolume)
	fmt.Sscanf(m.inputs[4].Value(), "%f", &m.config.OutputVolume)
	fmt.Sscanf(m.inputs[5].Value(), "%f", &m.config.VoiceThreshold)

	_ = config.SaveConfig(m.config)
}

func (m SettingsModel) View() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).Render("Settings")
	
	var inputs []string
	labels := []string{
		"Username",
		"Signaling URL",
		"STUN Servers",
		"Input Volume",
		"Output Volume",
		"Voice Threshold",
	}

	for i := range m.inputs {
		inputs = append(inputs, fmt.Sprintf("%-20s %s", labels[i], m.inputs[i].View()))
	}

	saveBtn := "[ Save ]"
	cancelBtn := "[ Cancel ]"
	if m.focusIndex == len(m.inputs) {
		saveBtn = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true).Render(saveBtn)
	} else if m.focusIndex == len(m.inputs)+1 {
		cancelBtn = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true).Render(cancelBtn)
	}

	buttons := lipgloss.JoinHorizontal(lipgloss.Center, saveBtn, "  ", cancelBtn)

	centerContent := lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		strings.Join(inputs, "\n\n"),
		"",
		buttons,
	)
	
	return placeCentered(m.width, m.height, centerContent)
}