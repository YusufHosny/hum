package chat

import (
	"context"
	"sync"
)

type ChatManager struct {
	ctx        context.Context
	historyMux sync.RWMutex
	history    []*ChatEnvelope
}

func NewChatManager(ctx context.Context) *ChatManager {
	return &ChatManager{
		ctx:     ctx,
		history: make([]*ChatEnvelope, 0),
	}
}

func (manager *ChatManager) SendMessage(author string, content string) {
	manager.historyMux.Lock()
	defer manager.historyMux.Unlock()
	manager.history = append(manager.history, &ChatEnvelope{})
}

func (manager *ChatManager) GetHistory() []*ChatEnvelope {
	manager.historyMux.RLock()
	defer manager.historyMux.Unlock()

	return append([]*ChatEnvelope{}, manager.history...)
}

func (manager *ChatManager) Attach(pipe ChatPipe) {
	go func() {
		for {
			select {
			case <-manager.ctx.Done():
				return
			case received := <-pipe.recvChannel:
				manager.historyMux.Lock()
				manager.history = append(manager.history, &received)
				manager.historyMux.Unlock()
			}
		}
	}()
}
