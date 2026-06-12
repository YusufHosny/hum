package ui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/YusufHosny/hum/internal/client"
	"github.com/YusufHosny/hum/internal/config"
	"github.com/YusufHosny/hum/internal/logger"
)

type ViewState int

const (
	ViewIntro ViewState = iota
	ViewHome
	ViewSettings
	ViewChannel
)

type UIModel struct {
	width  int
	height int

	state ViewState

	client *client.Client
	config *config.AppConfig
	logger logger.Logger

	// Sub-models
	introModel    IntroModel
	homeModel     HomeModel
	settingsModel SettingsModel
	channelModel  ChannelModel
}

func InitialModel(appConfig *config.AppConfig, appLogger logger.Logger, humClient *client.Client) UIModel {
	state := ViewHome
	if appConfig.Username == "" {
		state = ViewIntro
	}

	return UIModel{
		state:         state,
		client:        humClient,
		config:        appConfig,
		logger:        appLogger,
		introModel:    NewIntroModel(appConfig),
		homeModel:     NewHomeModel(appConfig),
		settingsModel: NewSettingsModel(appConfig),
		channelModel:  NewChannelModel(humClient, appConfig),
	}
}

func (m UIModel) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		m.listenForClientEvents(),
	)
}

// listenForClientEvents reads from the client's event channel and returns it as a tea.Msg
func (m UIModel) listenForClientEvents() tea.Cmd {
	return func() tea.Msg {
		if m.client != nil {
			event := <-m.client.Events()
			return event
		}
		return nil
	}
}

func (m UIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			if m.client != nil {
				m.client.Disconnect()
			}
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		
		m.introModel.Resize(msg.Width, msg.Height)
		m.homeModel.Resize(msg.Width, msg.Height)
		m.settingsModel.Resize(msg.Width, msg.Height)
		m.channelModel.Resize(msg.Width, msg.Height)
		return m, nil

	case client.Event:
		// Received an event from the Hum client, pass it to the channel model
		m.channelModel, cmd = m.channelModel.Update(msg)
		cmds = append(cmds, cmd)
		// Keep listening for the next event
		cmds = append(cmds, m.listenForClientEvents())
		return m, tea.Batch(cmds...)

	// Navigation Msgs
	case NavigateMsg:
		m.state = msg.View
		
		// Run init for new view if needed
		if msg.View == ViewHome {
			// Refresh home model config just in case it changed
			m.homeModel.config = m.config
		}
		return m, nil
	case ConnectMsg:
		err := m.client.Connect(msg.Channel, msg.Passkey)
		if err != nil {
			// In a real app we'd display this error, for now log it
			m.logger.Printf("Failed to connect: %v", err)
			return m, nil
		}
		
		m.channelModel.Reset(msg.Channel)
		m.state = ViewChannel
		
		// Reset triggers a tick for typing indicator
		return m, m.channelModel.getInitCmd()
	case DisconnectMsg:
		m.client.Disconnect()
		m.state = ViewHome
		return m, nil
	}

	// Route to sub-models
	switch m.state {
	case ViewIntro:
		m.introModel, cmd = m.introModel.Update(msg)
		cmds = append(cmds, cmd)
	case ViewHome:
		m.homeModel, cmd = m.homeModel.Update(msg)
		cmds = append(cmds, cmd)
	case ViewSettings:
		m.settingsModel, cmd = m.settingsModel.Update(msg)
		cmds = append(cmds, cmd)
	case ViewChannel:
		m.channelModel, cmd = m.channelModel.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m UIModel) View() string {
	switch m.state {
	case ViewIntro:
		return m.introModel.View()
	case ViewHome:
		return m.homeModel.View()
	case ViewSettings:
		return m.settingsModel.View()
	case ViewChannel:
		return m.channelModel.View()
	default:
		return "Unknown State"
	}
}

// Custom Messages
type NavigateMsg struct {
	View ViewState
}

type ConnectMsg struct {
	Channel string
	Passkey string
}

type DisconnectMsg struct{}

// Helper to center text
func placeCentered(w, h int, content string) string {
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, content)
}
