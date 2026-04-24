package chat

import (
	"context"
	"slices"
	"sync"
)

type ChatManager struct {
	ctx context.Context

	historyMux sync.RWMutex
	history    []*ChatEnvelope

	subscribersMux sync.RWMutex
	subscribers    []chan *ChatEnvelope

	inbox  chan *ChatEnvelope
	outbox chan *ChatEnvelope
}

func NewChatManager(ctx context.Context) *ChatManager {
	manager := &ChatManager{
		ctx:         ctx,
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

func (manager *ChatManager) broadcast(ce *ChatEnvelope) {
	manager.subscribersMux.RLock()
	defer manager.subscribersMux.RUnlock()

	for _, sub := range manager.subscribers {
		select {
		case sub <- ce:
		default:
			// Subscriber is full or blocked, drop message for this subscriber
			// to avoid blocking the entire inbox processing goroutine.
			// (In a real TUI you might want a larger buffer or a dropping strategy)
		}
	}
}

func (manager *ChatManager) Subscribe() <-chan *ChatEnvelope {
	manager.subscribersMux.Lock()
	defer manager.subscribersMux.Unlock()

	// Use a small buffer to handle brief bursts
	ch := make(chan *ChatEnvelope, 100)
	manager.subscribers = append(manager.subscribers, ch)
	return ch
}

func (manager *ChatManager) addToHistory(ce *ChatEnvelope) {
	manager.historyMux.Lock()
	defer manager.historyMux.Unlock()
	manager.history = append(manager.history, ce)
}

func (manager *ChatManager) GetInboxOutbox() (chan<- *ChatEnvelope, <-chan *ChatEnvelope) {
	return manager.inbox, manager.outbox
}

func (manager *ChatManager) SendMessage(from string, content string) {
	envelope := newMessageEnvelope(from, []byte(content))
	manager.addToHistory(envelope)
	manager.outbox <- envelope
}

func (manager *ChatManager) NotifyTyping(from string) {
	manager.outbox <- newTypingMetadataEnvelope(from)
}

func (manager *ChatManager) GetHistory() []*ChatEnvelope {
	manager.historyMux.RLock()
	defer manager.historyMux.Unlock()
	return slices.Clone(manager.history)
}
