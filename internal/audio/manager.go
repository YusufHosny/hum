package audio

import (
	"context"
	"sync"
)

type AudioEnvelope struct {
	/* TODO: for now i wont use seqNumber, but with a rollover counter it could be used as encryption nonce
	which would drop payload size by abit, as the nonce is sent inside the packet currently */
	seqNumber *uint32
	Content   []byte
}

type AudioManager struct {
	ctx context.Context

	inbox  chan *AudioEnvelope
	outbox chan *AudioEnvelope

	subscribersMux sync.RWMutex
	subscribers    []chan *AudioEnvelope
}

func NewAudioManager(ctx context.Context) (*AudioManager, error) {
	manager := &AudioManager{
		ctx:         ctx,
		inbox:       make(chan *AudioEnvelope),
		outbox:      make(chan *AudioEnvelope),
		subscribers: make([]chan *AudioEnvelope, 0),
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
				manager.broadcast(received)
			}
		}
	}()

	return manager, nil
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
