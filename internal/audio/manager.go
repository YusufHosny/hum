package audio

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/YusufHosny/hum/internal/crypto"
)

type AudioEnvelope struct {
	Content []byte
}

type AudioManager struct {
	ctx    context.Context
	cancel context.CancelFunc

	callCtx    context.Context
	callCancel context.CancelFunc
	callMux    sync.Mutex

	config  *AudioConfig
	cryptor *crypto.Cryptor

	recorder AudioRecorder
	player   AudioPlayer
	encoder  AudioEncoder

	inbox  chan *AudioEnvelope
	outbox chan *AudioEnvelope

	subscribersMux sync.RWMutex
	subscribers    []chan *AudioEnvelope

	OnSpeaking func(bool)
	isSpeaking bool
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
		OnSpeaking:  func(bool) {},
	}

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

func (manager *AudioManager) JoinCall() error {
	manager.callMux.Lock()
	defer manager.callMux.Unlock()

	if manager.callCtx != nil {
		return nil // already in call
	}

	callCtx, callCancel := context.WithCancel(manager.ctx)
	manager.callCtx = callCtx
	manager.callCancel = callCancel

	if err := manager.player.Start(); err != nil {
		manager.callCancel()
		manager.callCtx = nil
		return fmt.Errorf("failed to start player: %w", err)
	}

	if err := manager.recorder.Start(); err != nil {
		manager.player.Stop()
		manager.callCancel()
		manager.callCtx = nil
		return fmt.Errorf("failed to start recorder: %w", err)
	}

	go manager.captureLoop(callCtx)
	go manager.playbackLoop(callCtx)

	return nil
}

func (manager *AudioManager) LeaveCall() {
	manager.callMux.Lock()
	defer manager.callMux.Unlock()

	if manager.callCtx != nil {
		manager.callCancel()
		manager.callCtx = nil
		manager.callCancel = nil
		
		manager.recorder.Stop()
		manager.player.Stop()
	}
}

func (manager *AudioManager) Close() {
	manager.LeaveCall()
	manager.cancel()

	manager.subscribersMux.Lock()
	for _, sub := range manager.subscribers {
		close(sub)
	}
	manager.subscribers = nil
	manager.subscribersMux.Unlock()
}

func (manager *AudioManager) captureLoop(callCtx context.Context) {
	for {
		select {
		case <-callCtx.Done():
			return
		default:
			pcm, err := manager.recorder.Read()
			if err != nil {
				log.Printf("failed to read mic: %v\n", err)
				continue
			}

			if manager.config.Muted {
				if manager.isSpeaking {
					manager.isSpeaking = false
					manager.OnSpeaking(false)
				}
				continue
			}

			isAboveThreshold := IsAboveThreshold(pcm, manager.config.VoiceThreshold)
			if isAboveThreshold != manager.isSpeaking {
				manager.isSpeaking = isAboveThreshold
				manager.OnSpeaking(isAboveThreshold)
			}

			if !isAboveThreshold {
				continue
			}

			encoded, err := manager.encoder.Encode(pcm)
			if err != nil {
				log.Printf("failed to encode frame: %v\n", err)
				continue
			}

			encrypted, err := manager.cryptor.Encrypt(encoded, nil)
			if err != nil {
				log.Printf("failed to encrypt frame: %v\n", err)
				continue
			}

			envelope := &AudioEnvelope{Content: encrypted}

			select {
			case manager.outbox <- envelope:
			default:
			}
		}
	}
}

func (manager *AudioManager) playbackLoop(callCtx context.Context) {
	for {
		select {
		case <-callCtx.Done():
			return
		case <-manager.ctx.Done():
			return
		case received := <-manager.inbox:
			// only broadcast to subscribers if they need to analyze the incoming audio
			manager.broadcast(received)

			if manager.config.Deafened {
				continue
			}

			decrypted, err := manager.cryptor.Decrypt(received.Content, nil)
			if err != nil {
				log.Printf("failed to decrypt frame: %v\n", err)
				continue
			}

			pcm, err := manager.encoder.Decode(decrypted)
			if err != nil {
				log.Printf("failed to decode frame: %v\n", err)
				continue
			}

			err = manager.player.Write(pcm)
			if err != nil {
				log.Printf("failed to play frame: %v\n", err)
			}
		}
	}
}

func (manager *AudioManager) broadcast(ae *AudioEnvelope) {
	manager.subscribersMux.RLock()
	defer manager.subscribersMux.RUnlock()

	for _, sub := range manager.subscribers {
		select {
		case sub <- ae:
		default:
		}
	}
}

func (manager *AudioManager) Subscribe() <-chan *AudioEnvelope {
	manager.subscribersMux.Lock()
	defer manager.subscribersMux.Unlock()

	ch := make(chan *AudioEnvelope, 100)
	manager.subscribers = append(manager.subscribers, ch)
	return ch
}

func (manager *AudioManager) GetInbox() chan<- *AudioEnvelope {
	return manager.inbox
}

func (manager *AudioManager) GetOutbox() <-chan *AudioEnvelope {
	return manager.outbox
}

func MakeAudioEnvelope(content []byte) *AudioEnvelope {
	return &AudioEnvelope{Content: content}
}

func (manager *AudioManager) SetInputVolume(vol float64) {
	manager.config.InputVolume = vol
	manager.recorder.SetVolume(vol)
}

func (manager *AudioManager) SetOutputVolume(vol float64) {
	manager.config.OutputVolume = vol
	manager.player.SetVolume(vol)
}

func (manager *AudioManager) SetMute(muted bool) {
	manager.config.Muted = muted
	manager.recorder.SetMute(muted)
}

func (manager *AudioManager) SetDeafen(deafened bool) {
	manager.config.Deafened = deafened
	manager.player.SetDeafen(deafened)
}

func (manager *AudioManager) IsMuted() bool {
	return manager.config.Muted
}

func (manager *AudioManager) IsDeafened() bool {
	return manager.config.Deafened
}

func (manager *AudioManager) SetBitrate(bitrate int) error {
	return manager.encoder.SetBitrate(bitrate)
}
