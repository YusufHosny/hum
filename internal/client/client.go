package client

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/YusufHosny/hum/internal/audio"
	"github.com/YusufHosny/hum/internal/chat"
	"github.com/YusufHosny/hum/internal/config"
	"github.com/YusufHosny/hum/internal/crypto"
	"github.com/YusufHosny/hum/internal/logger"
	"github.com/YusufHosny/hum/internal/p2p"
)

type EventType int

const (
	EventMessage EventType = iota
	EventPeerJoined
	EventPeerLeft
	EventSpeaking
	EventDisconnected
	EventMetadata
)

type Event struct {
	Type     EventType
	Username string
	Payload  interface{}
}

type Client struct {
	ctx    context.Context
	cancel context.CancelFunc

	Config *config.AppConfig
	Logger logger.Logger

	ChatManager  *chat.ChatManager
	AudioManager *audio.AudioManager
	MeshManager  *p2p.MeshManager

	events chan Event

	mux sync.Mutex
}

func NewClient(appConfig *config.AppConfig, appLogger logger.Logger) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		ctx:    ctx,
		cancel: cancel,
		Config: appConfig,
		Logger: appLogger,
		events: make(chan Event, 100),
	}
}

func (c *Client) Events() <-chan Event {
	return c.events
}

func (c *Client) Connect(channelName, passkey string) error {
	c.mux.Lock()
	defer c.mux.Unlock()

	c.Logger.Printf("Client connecting to %s...", channelName)

	rawURL := fmt.Sprintf("%s/%s?usr=%s", c.Config.SignalingURL, channelName, c.Config.Username)
	signalingURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("failed to parse signaling URL: %w", err)
	}

	cryptor, err := crypto.NewCryptor(channelName, passkey)
	if err != nil {
		return fmt.Errorf("failed to initialize cryptor: %w", err)
	}

	c.ChatManager = chat.NewChatManager(c.ctx, c.Config.Username, cryptor)

	audioConfig := audio.NewDefaultAudioConfig()
	audioConfig.InputVolume = c.Config.InputVolume
	audioConfig.OutputVolume = c.Config.OutputVolume
	audioConfig.VoiceThreshold = c.Config.VoiceThreshold

	c.AudioManager, err = audio.NewAudioManager(c.ctx, audioConfig, cryptor)
	if err != nil {
		return fmt.Errorf("failed to initialize audio manager: %w", err)
	}

	// We don't Start() the audio manager yet. Let the user JoinCall() later.
	c.AudioManager.OnSpeaking = func(speaking bool) {
		_ = c.ChatManager.NotifySpeaking(speaking)
		// Emitting to local client
		c.events <- Event{
			Type:     EventSpeaking,
			Username: c.Config.Username, // It's us
			Payload:  speaking,
		}
	}

	meshConfig := p2p.MeshConfig{
		SignalingServerURL: *signalingURL,
		STUNServers:        c.Config.STUNServers,
		Username:           c.Config.Username,
		ChannelName:        channelName,
		Logger:             c.Logger,
	}

	c.MeshManager, err = p2p.NewMeshManager(c.ctx, meshConfig, c.ChatManager, c.AudioManager)
	if err != nil {
		return fmt.Errorf("failed to initialize mesh manager: %w", err)
	}

	c.MeshManager.OnMemberJoin = func(username string) {
		c.events <- Event{
			Type:     EventPeerJoined,
			Username: username,
		}
	}

	c.MeshManager.OnMemberLeave = func(username string) {
		c.events <- Event{
			Type:     EventPeerLeft,
			Username: username,
		}
	}

	go c.listenChat()

	// Update recent channels
	config.AddRecentChannel(c.Config, channelName)
	_ = config.SaveConfig(c.Config)

	return nil
}

func (c *Client) listenChat() {
	sub := c.ChatManager.Subscribe()
	for {
		select {
		case <-c.ctx.Done():
			return
		case env, ok := <-sub:
			if !ok {
				return
			}
			if env.Type == "message" {
				c.events <- Event{
					Type:     EventMessage,
					Username: env.From,
					Payload:  string(env.Content),
				}
			} else if env.Type == "metadata" {
				c.events <- Event{
					Type:     EventMetadata,
					Username: env.From,
					Payload:  env.Content,
				}
			}
		}
	}
}

func (c *Client) Disconnect() {
	c.mux.Lock()
	defer c.mux.Unlock()

	c.Logger.Println("Client disconnecting...")

	if c.MeshManager != nil {
		_ = c.MeshManager.Close()
	}
	if c.AudioManager != nil {
		c.AudioManager.Close()
	}
	
	// Create a new context for the next connection
	c.cancel()
	c.ctx, c.cancel = context.WithCancel(context.Background())

	c.events <- Event{
		Type: EventDisconnected,
	}
}

func (c *Client) JoinCall() error {
	if c.AudioManager == nil {
		return fmt.Errorf("audio manager not initialized")
	}
	err := c.AudioManager.JoinCall()
	if err == nil {
		_ = c.ChatManager.NotifyJoin(true, true)
	}
	return err
}

func (c *Client) LeaveCall() {
	if c.AudioManager != nil {
		c.AudioManager.LeaveCall()
		_ = c.ChatManager.NotifyJoin(true, false)
	}
}

func (c *Client) SendMessage(text string) error {
	if c.ChatManager == nil {
		return fmt.Errorf("chat manager not initialized")
	}
	return c.ChatManager.SendMessage(text)
}

func (c *Client) NotifyTyping() {
	if c.ChatManager != nil {
		// Basic throttle
		go func() {
			time.Sleep(100 * time.Millisecond)
			_ = c.ChatManager.NotifyTyping()
		}()
	}
}

func (c *Client) SetMute(muted bool) {
	if c.AudioManager != nil {
		c.AudioManager.SetMute(muted)
		if c.ChatManager != nil {
			_ = c.ChatManager.NotifyAudio(muted, c.AudioManager.IsDeafened())
		}
	}
}

func (c *Client) SetDeafen(deafened bool) {
	if c.AudioManager != nil {
		c.AudioManager.SetDeafen(deafened)
		if c.ChatManager != nil {
			_ = c.ChatManager.NotifyAudio(c.AudioManager.IsMuted(), deafened)
		}
	}
}
