package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/YusufHosny/hum/internal/client"
	"github.com/YusufHosny/hum/internal/config"
)

type ParticipantState struct {
	Username string
	InChat   bool
	InCall   bool
	Muted    bool
	Deafened bool
	Speaking bool
}

type ChannelModel struct {
	client *client.Client
	config *config.AppConfig
	width  int
	height int

	channelName string

	input textinput.Model
	vp    viewport.Model

	messages     []string
	participants map[string]*ParticipantState

	sidebarWidth int
	rightSidebar int

	// User state
	inCall   bool
	muted    bool
	deafened bool

	// Action buttons (focus indices)
	focusIndex int // 0 = leave, 1 = call, 2 = mute, 3 = deafen, 4 = input

	typingUsers map[string]time.Time
}

func NewChannelModel(humClient *client.Client, appConfig *config.AppConfig) ChannelModel {
	ti := textinput.New()
	ti.Placeholder = "Type a message..."
	ti.Focus()
	ti.CharLimit = 256

	vp := viewport.New(0, 0)
	vp.SetContent("Welcome to the channel!")

	return ChannelModel{
		client:       humClient,
		config:       appConfig,
		sidebarWidth: 25,
		rightSidebar: 25,
		input:        ti,
		vp:           vp,
		messages:     make([]string, 0),
		participants: make(map[string]*ParticipantState),
		focusIndex:   4, // input by default
		typingUsers:  make(map[string]time.Time),
	}
}

func tickTyping() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return TypingTickMsg(t)
	})
}

type TypingTickMsg time.Time

func (m *ChannelModel) getInitCmd() tea.Cmd {
	return tickTyping()
}

func (m *ChannelModel) Reset(channelName string) {
	m.channelName = channelName
	m.messages = []string{}
	m.participants = make(map[string]*ParticipantState)
	m.inCall = false
	m.muted = false
	m.deafened = false
	m.focusIndex = 4
	m.typingUsers = make(map[string]time.Time)
	
	m.vp.SetContent("Joined " + channelName)
	m.vp.GotoBottom()
}

func (m *ChannelModel) Resize(w, h int) {
	m.width = w
	m.height = h

	// Layout math
	centerWidth := w - m.sidebarWidth - m.rightSidebar - 4
	if centerWidth < 10 {
		centerWidth = 10
	}

	m.input.Width = centerWidth - 4
	m.vp.Width = centerWidth
	
	// Total height minus header (1) and footer (3)
	m.vp.Height = h - 4
}

func (m ChannelModel) Update(msg tea.Msg) (ChannelModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case TypingTickMsg:
		now := time.Now()
		for user, t := range m.typingUsers {
			if now.Sub(t) > 3*time.Second {
				delete(m.typingUsers, user)
			}
		}
		return m, tickTyping()

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return DisconnectMsg{} }
		case "tab":
			m.focusIndex++
			if m.focusIndex > 4 {
				m.focusIndex = 0
			}
			if m.focusIndex == 4 {
				m.input.Focus()
			} else {
				m.input.Blur()
			}
			return m, nil
		case "shift+tab":
			m.focusIndex--
			if m.focusIndex < 0 {
				m.focusIndex = 4
			}
			if m.focusIndex == 4 {
				m.input.Focus()
			} else {
				m.input.Blur()
			}
			return m, nil
		case "enter":
			if m.focusIndex == 0 { // leave
				return m, func() tea.Msg { return DisconnectMsg{} }
			} else if m.focusIndex == 1 { // call
				m.inCall = !m.inCall
				if m.inCall {
					_ = m.client.JoinCall()
				} else {
					m.client.LeaveCall()
				}
				return m, nil
			} else if m.focusIndex == 2 { // mute
				m.muted = !m.muted
				m.client.SetMute(m.muted)
				return m, nil
			} else if m.focusIndex == 3 { // deafen
				m.deafened = !m.deafened
				m.client.SetDeafen(m.deafened)
				return m, nil
			} else if m.focusIndex == 4 { // input
				val := m.input.Value()
				if val != "" {
					m.client.SendMessage(val)
					m.input.SetValue("")
				}
				return m, nil
			}
		default:
			// Let input handle it, but also notify typing if it's a character
			if m.focusIndex == 4 && len(msg.String()) == 1 {
				m.client.NotifyTyping()
			}
		}

	case client.Event:
		switch msg.Type {
		case client.EventMessage:
			style := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
			if msg.Username == m.config.Username {
				style = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))
			}
			m.messages = append(m.messages, fmt.Sprintf("%s: %s", style.Render(msg.Username), msg.Payload.(string)))
			m.vp.SetContent(strings.Join(m.messages, "\n"))
			m.vp.GotoBottom()
		case client.EventPeerJoined:
			m.messages = append(m.messages, lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(fmt.Sprintf("%s joined the channel", msg.Username)))
			m.vp.SetContent(strings.Join(m.messages, "\n"))
			m.vp.GotoBottom()
			if _, ok := m.participants[msg.Username]; !ok {
				m.participants[msg.Username] = &ParticipantState{Username: msg.Username, InChat: true}
			}
		case client.EventPeerLeft:
			m.messages = append(m.messages, lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(fmt.Sprintf("%s left the channel", msg.Username)))
			m.vp.SetContent(strings.Join(m.messages, "\n"))
			m.vp.GotoBottom()
			delete(m.participants, msg.Username)
		case client.EventSpeaking:
			if p, ok := m.participants[msg.Username]; ok {
				p.Speaking = msg.Payload.(bool)
			}
		case client.EventMetadata:
			content := msg.Payload.([]byte)
			if strings.Contains(string(content), `"type":"typing"`) {
				m.typingUsers[msg.Username] = time.Now()
			} else if strings.Contains(string(content), `"type":"join"`) {
				if p, ok := m.participants[msg.Username]; ok {
					p.InCall = strings.Contains(string(content), `"call":true`)
				}
			} else if strings.Contains(string(content), `"type":"audio"`) {
				if p, ok := m.participants[msg.Username]; ok {
					p.Muted = strings.Contains(string(content), `"muted":true`)
					p.Deafened = strings.Contains(string(content), `"deafened":true`)
				}
			}
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)

	m.vp, cmd = m.vp.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m ChannelModel) View() string {
	// Header
	btnStyle := lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("240"))
	focusStyle := lipgloss.NewStyle().Padding(0, 1).Bold(true).Foreground(lipgloss.Color("205"))
	activeStyle := lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("10")) // green

	renderBtn := func(idx int, text string, active bool) string {
		if m.focusIndex == idx {
			return focusStyle.Render("[" + text + "]")
		} else if active {
			return activeStyle.Render("[" + text + "]")
		}
		return btnStyle.Render("[" + text + "]")
	}

	leaveBtn := renderBtn(0, "Leave Chat (Esc)", false)
	callBtn := renderBtn(1, "Join Call", m.inCall)
	muteBtn := renderBtn(2, "Mute", m.muted)
	deafenBtn := renderBtn(3, "Deafen", m.deafened)

	header := lipgloss.JoinHorizontal(lipgloss.Top, leaveBtn, " | ", callBtn, " ", muteBtn, " ", deafenBtn)
	headerStyle := lipgloss.NewStyle().Border(lipgloss.NormalBorder(), false, false, true, false).PaddingBottom(1)
	
	// Left Sidebar (placeholder for now)
	leftSidebar := lipgloss.NewStyle().
		Width(m.sidebarWidth).
		Height(m.height).
		Border(lipgloss.NormalBorder(), false, true, false, false).
		Render("Recents go here...")

	// Right Sidebar
	var parts []string
	parts = append(parts, lipgloss.NewStyle().Bold(true).Render("Participants"))
	for _, p := range m.participants {
		stateStr := ""
		if p.InCall {
			stateStr += " 📞"
		}
		if p.Deafened {
			stateStr += " 🎧"
		} else if p.Muted {
			stateStr += " 🔇"
		}
		
		name := p.Username
		if p.Speaking {
			name = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render(name)
		}
		parts = append(parts, name+stateStr)
	}

	rightSidebar := lipgloss.NewStyle().
		Width(m.rightSidebar).
		Height(m.height).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		Render(strings.Join(parts, "\n"))

	var typingNames []string
	for user := range m.typingUsers {
		typingNames = append(typingNames, user)
	}

	typingStr := " "
	if len(typingNames) > 0 {
		typingStr = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(strings.Join(typingNames, ", ") + " is typing...")
	}

	// Center
	centerContent := lipgloss.JoinVertical(lipgloss.Left,
		headerStyle.Render(header),
		m.vp.View(),
		typingStr,
		lipgloss.NewStyle().Border(lipgloss.NormalBorder(), true, false, false, false).Render(m.input.View()),
	)
	centerStyle := lipgloss.NewStyle().Width(m.width - m.sidebarWidth - m.rightSidebar - 2)
	centerCol := centerStyle.Render(centerContent)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftSidebar, centerCol, rightSidebar)
}