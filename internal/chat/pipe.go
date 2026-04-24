package chat

import (
	"context"
	"encoding/json"
	"log"
	"time"
)

type ChatEnvelope struct {
	Type      string // "message", "metadata"
	From      string
	Content   []byte
	CreatedAt time.Time
}

type MetadataPayload struct {
	Type string `json:"type"` // "typing"
}

type ChatPipe struct {
	username string

	sendChannel chan ChatEnvelope
	recvChannel chan ChatEnvelope

	sendHandler func(ChatEnvelope)
	recvHandler func(ChatEnvelope)
}

func NewChatPipe(username string) *ChatPipe {
	return &ChatPipe{
		username:    username,
		sendChannel: make(chan ChatEnvelope, 100),
		recvChannel: make(chan ChatEnvelope, 100),
	}
}

func (pipe *ChatPipe) SetSendHandler(handler func(ChatEnvelope)) {
	pipe.sendHandler = handler
}

func (pipe *ChatPipe) SetRecvHandler(handler func(ChatEnvelope)) {
	pipe.recvHandler = handler
}

func (pipe *ChatPipe) PassSendEnvelope(ce ChatEnvelope) {
	pipe.sendChannel <- ce
}

func (pipe *ChatPipe) PassRecvEnvelope(ce ChatEnvelope) {
	pipe.recvChannel <- ce
}

func (pipe *ChatPipe) SendMessage(message string) {
	pipe.sendChannel <- ChatEnvelope{
		Type:      "message",
		From:      pipe.username,
		Content:   []byte(message),
		CreatedAt: time.Now(),
	}
}

func (pipe *ChatPipe) NotifyTyping() {
	payload, err := json.Marshal(MetadataPayload{Type: "typing"})
	if err != nil {
		log.Printf("Failed to marshal metadata: %v\n", err)
		return
	}
	pipe.sendChannel <- ChatEnvelope{
		Type:      "metadata",
		From:      pipe.username,
		Content:   payload,
		CreatedAt: time.Now(),
	}
}

func (pipe *ChatPipe) Receive(content []byte, msgType string) {
	pipe.recvChannel <- ChatEnvelope{
		Type:      msgType,
		From:      pipe.username,
		Content:   content,
		CreatedAt: time.Now(),
	}
}

func (pipe *ChatPipe) Process(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case env := <-pipe.sendChannel:
			if pipe.sendHandler != nil {
				pipe.sendHandler(env)
			}
		case env := <-pipe.recvChannel:
			if pipe.recvHandler != nil {
				pipe.recvHandler(env)
			}
		}
	}
}
