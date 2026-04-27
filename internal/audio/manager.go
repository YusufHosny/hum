package audio

import (
	"context"
	"fmt"
	"sync"
	
	"github.com/YusufHosny/hum/internal/crypto"
)

type AudioEnvelope struct {
	/* TODO: for now i wont use seqNumber, but with a rollover counter it could be used as encryption nonce
	which would drop payload size by abit, as the nonce is sent inside the packet currently */
	seqNumber *uint32
	Content   []byte
}

type AudioManager struct {
	ctx    context.Context
	cancel context.CancelFunc

	config  *AudioConfig
	cryptor *crypto.Cryptor
	
	recorder AudioRecorder
	player   AudioPlayer
	encoder  AudioEncoder

	// Channels for the network layer
	inbox  chan *AudioEnvelope
	outbox chan *AudioEnvelope

	// For local pub/sub (e.g. visualizing audio in TUI)
	subscribersMux sync.RWMutex
	subscribers    []chan *AudioEnvelope
}

func NewAudioManager(ctx context.Context, config *AudioConfig, cryptor *crypto.Cryptor) (*AudioManager, error) {
	ctx, cancel := context.WithCancel(ctx)
	
	manager := &AudioManager{
		ctx:         ctx,
		cancel:      cancel,
		config:      config,
		cryptor:     cryptor,
		inbox:       make(chan *AudioEnvelope, 100),
		outbox:      make(chan *AudioEnvelope, 100),
		subscribers: make([]chan *AudioEnvelope, 0),
	}

	// Initialize components
	var err error
	
	manager.encoder, err = NewOpusEncoder(config)
	if err != nil {
		return nil, fmt.Errorf("failed to init encoder: %w", err)
	}

	manager.recorder, err = NewMalgoRecorder(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to init recorder: %w", err)
	}

	manager.player, err = NewMalgoPlayer(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to init player: %w", err)
	}

	return manager, nil
}

func (m *AudioManager) Start() error {
	if err := m.player.Start(); err != nil {
		return fmt.Errorf("failed to start player: %w", err)
	}

	if err := m.recorder.Start(); err != nil {
		return fmt.Errorf("failed to start recorder: %w", err)
	}

	go m.captureLoop()
	go m.playbackLoop()

	return nil
}

func (m *AudioManager) Stop() {
	m.cancel()
	m.recorder.Stop()
	m.player.Stop()

	m.subscribersMux.Lock()
	for _, sub := range m.subscribers {
		close(sub)
	}
	m.subscribers = nil
	m.subscribersMux.Unlock()
}

// captureLoop continuously reads from the mic, encodes, encrypts, and sends to outbox
func (m *AudioManager) captureLoop() {
	for {
		select {
		case <-m.ctx.Done():
			return
		default:
			pcm, err := m.recorder.Read()
			if err != nil {
				// Log or handle error, for now we continue
				continue
			}

			encoded, err := m.encoder.Encode(pcm)
			if err != nil {
				continue
			}

			// Encrypt (nil nonce means cryptor generates a random one and prepends it)
			encrypted, err := m.cryptor.Encrypt(encoded, nil)
			if err != nil {
				continue
			}

			envelope := &AudioEnvelope{Content: encrypted}

			select {
			case m.outbox <- envelope:
			default:
				// Dropped frame if network is too slow
			}
		}
	}
}

// playbackLoop continuously reads from inbox, decrypts, decodes, and plays
func (m *AudioManager) playbackLoop() {
	for {
		select {
		case <-m.ctx.Done():
			return
		case received := <-m.inbox:
			// 1. Broadcast to local subscribers (TUI visualizations etc)
			m.broadcast(received)

			// 2. Decrypt
			decrypted, err := m.cryptor.Decrypt(received.Content, nil)
			if err != nil {
				// Failed to decrypt (wrong channel, corruption, etc)
				continue
			}

			// 3. Decode Opus
			pcm, err := m.encoder.Decode(decrypted)
			if err != nil {
				continue
			}

			// 4. Play
			// TODO: A Jitter Buffer should be placed here in the future
			// to handle out-of-order packets before writing to the player.
			_ = m.player.Write(pcm)
		}
	}
}

func (m *AudioManager) broadcast(ae *AudioEnvelope) {
	m.subscribersMux.RLock()
	defer m.subscribersMux.RUnlock()

	for _, sub := range m.subscribers {
		select {
		case sub <- ae:
		default:
		}
	}
}

func (m *AudioManager) Subscribe() <-chan *AudioEnvelope {
	m.subscribersMux.Lock()
	defer m.subscribersMux.Unlock()

	ch := make(chan *AudioEnvelope, 100)
	m.subscribers = append(m.subscribers, ch)
	return ch
}

func (m *AudioManager) GetInbox() chan<- *AudioEnvelope {
	return m.inbox
}

func (m *AudioManager) GetOutbox() <-chan *AudioEnvelope {
	return m.outbox
}

func MakeAudioEnvelope(content []byte) *AudioEnvelope {
	return &AudioEnvelope{Content: content}
}

// --- Dynamic Control API ---

func (m *AudioManager) SetInputVolume(vol float64) {
	m.recorder.SetVolume(vol)
}

func (m *AudioManager) SetOutputVolume(vol float64) {
	m.player.SetVolume(vol)
}

func (m *AudioManager) SetMute(muted bool) {
	m.recorder.SetMute(muted)
}

func (m *AudioManager) SetDeafen(deafened bool) {
	m.player.SetDeafen(deafened)
}

func (m *AudioManager) SetBitrate(bitrate int) error {
	return m.encoder.SetBitrate(bitrate)
}
