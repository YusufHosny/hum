package chat

import (
	"context"
	"encoding/json"
	"log"
)

type ChatEnvelope struct {
	Type    string // "message", "metadata"
	From    string
	Content []byte
}

type MetadataPayload struct {
	Type string `json:"type"` // "typing"
}

type ChatPipe struct {
	sendChannel chan ChatEnvelope
	recvChannel chan ChatEnvelope

	sendHandler func(ChatEnvelope)
	recvHandler func(ChatEnvelope)
}

func NewChatPipe() *ChatPipe {
	return &ChatPipe{
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

func (pipe *ChatPipe) SendEnvelope(ce ChatEnvelope) {
	pipe.sendChannel <- ce
}

func (pipe *ChatPipe) ReceiveEnvelope(ce ChatEnvelope) {
	pipe.recvChannel <- ce
}

func (pipe *ChatPipe) SendMessage(message string) {
	pipe.sendChannel <- ChatEnvelope{Type: "message", Content: []byte(message)}
}

func (pipe *ChatPipe) NotifyTyping() {
	payload, err := json.Marshal(MetadataPayload{Type: "typing"})
	if err != nil {
		log.Printf("Failed to marshal metadata: %v\n", err)
		return
	}
	pipe.sendChannel <- ChatEnvelope{Type: "metadata", Content: payload}
}

func (pipe *ChatPipe) Receive(content []byte, msgType string) {
	pipe.recvChannel <- ChatEnvelope{Type: msgType, Content: content}
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
