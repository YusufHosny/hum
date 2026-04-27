package chat

import (
	"context"
	"slices"
	"sync"

	"github.com/YusufHosny/hum/internal/crypto"
)

type ChatManager struct {
	ctx      context.Context
	username string
	cryptor  *crypto.Cryptor

	historyMux sync.RWMutex
	history    []*ChatEnvelope

	subscribersMux sync.RWMutex
	subscribers    []chan *ChatEnvelope

	inbox  chan *ChatEnvelope
	outbox chan *ChatEnvelope
}

func NewChatManager(
	ctx context.Context,
	username string,
	cryptor *crypto.Cryptor,
) *ChatManager {
	manager := &ChatManager{
		ctx:         ctx,
		username:    username,
		cryptor:     cryptor,
		history:     make([]*ChatEnvelope, 0),
		subscribers: make([]chan *ChatEnvelope, 0),
		inbox:       make(chan *ChatEnvelope, 100),
		outbox:      make(chan *ChatEnvelope, 100),
	}

	go func() {
		for {
			select {
			case <-manager.ctx.Done():
				manager.subscribersMux.Lock()
				for _, sub := range manager.subscribers {
					close(sub)
				}
				manager.subscribers = nil
				manager.subscribersMux.Unlock()
				return
			case received := <-manager.inbox:
				manager.addToHistory(received)
				manager.broadcast(received)
			}
		}
	}()

	return manager
}

func (manager *ChatManager) GetInbox() chan<- *ChatEnvelope {
	return manager.inbox
}

func (manager *ChatManager) GetOutbox() <-chan *ChatEnvelope {
	return manager.outbox
}

func (manager *ChatManager) broadcast(ce *ChatEnvelope) {
	manager.subscribersMux.RLock()
	defer manager.subscribersMux.RUnlock()

	for _, sub := range manager.subscribers {
		select {
		case sub <- ce:
		default:
		}
	}
}

func (manager *ChatManager) Subscribe() <-chan *ChatEnvelope {
	manager.subscribersMux.Lock()
	defer manager.subscribersMux.Unlock()

	ch := make(chan *ChatEnvelope, 100)
	manager.subscribers = append(manager.subscribers, ch)
	return ch
}

func (manager *ChatManager) addToHistory(ce *ChatEnvelope) {
	manager.historyMux.Lock()
	defer manager.historyMux.Unlock()
	manager.history = append(manager.history, ce)
}

func (manager *ChatManager) encryptAndSend(ce *ChatEnvelope) error {
	encrypted, err := manager.cryptor.Encrypt(ce.Content, nil)
	if err != nil {
		return err
	}
	manager.addToHistory(ce)
	ce.Content = encrypted
	manager.outbox <- ce
	return nil
}

func (manager *ChatManager) SendMessage(content string) error {
	envelope := newMessageEnvelope(manager.username, []byte(content))
	return manager.encryptAndSend(envelope)
}

func (manager *ChatManager) NotifyTyping() error {
	envelope := newTypingMetadataEnvelope(manager.username)
	return manager.encryptAndSend(envelope)
}

func (manager *ChatManager) NotifyJoin(chat bool, call bool) error {
	envelope := newJoinMetadataEnvelope(manager.username, chat, call)
	return manager.encryptAndSend(envelope)
}

func (manager *ChatManager) NotifyAudio(muted bool, deafened bool) error {
	envelope := newAudioMetadataEnvelope(manager.username, muted, deafened)
	return manager.encryptAndSend(envelope)
}

func (manager *ChatManager) GetHistory() []*ChatEnvelope {
	manager.historyMux.RLock()
	defer manager.historyMux.Unlock()
	return slices.Clone(manager.history)
}
