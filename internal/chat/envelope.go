package chat

import (
	"encoding/json"
	"fmt"
	"time"
)

type ChatEnvelope struct {
	Type      string // "message", "metadata"
	From      string
	Content   []byte
	CreatedAt time.Time
}

type metadataPayload struct {
	Type string `json:"type"` // "typing", "join", "audio"

	// join event
	Chat bool `json:"chat,omitempty"`
	Call bool `json:"call,omitempty"`

	// audio event
	Muted    bool `json:"muted,omitempty"`
	Deafened bool `json:"deafened,omitempty"`
}

func newMessageEnvelope(from string, content []byte) *ChatEnvelope {
	return &ChatEnvelope{
		Type:      "message",
		From:      from,
		Content:   content,
		CreatedAt: time.Now(),
	}
}

func isValidMessageType(value string) bool {
	return value == "message" || value == "metadata"
}

// public version for recv only
func NewRecvEnvelope(msgType string, from string, content []byte) (*ChatEnvelope, error) {
	if !isValidMessageType(msgType) {
		err := fmt.Errorf("Unexpected message type: %v", msgType)
		return nil, err
	}

	envelope := &ChatEnvelope{
		Type:      msgType,
		From:      from,
		Content:   content,
		CreatedAt: time.Now(),
	}
	return envelope, nil
}

func newMetadataEnvelope(from string, content []byte) *ChatEnvelope {
	return &ChatEnvelope{
		Type:      "metadata",
		From:      from,
		Content:   content,
		CreatedAt: time.Now(),
	}
}

func newTypingMetadataEnvelope(from string) *ChatEnvelope {
	payload, err := json.Marshal(metadataPayload{Type: "typing"})
	if err != nil {
		panic(err)
	}

	return newMetadataEnvelope(from, payload)
}

func newJoinMetadataEnvelope(from string, chat bool, call bool) *ChatEnvelope {
	payload, err := json.Marshal(metadataPayload{
		Type: "join",
		Chat: chat,
		Call: call,
	})
	if err != nil {
		panic(err)
	}

	return newMetadataEnvelope(from, payload)
}

func newAudioMetadataEnvelope(from string, muted bool, deafened bool) *ChatEnvelope {
	payload, err := json.Marshal(metadataPayload{
		Type:     "audio",
		Muted:    muted,
		Deafened: deafened,
	})
	if err != nil {
		panic(err)
	}

	return newMetadataEnvelope(from, payload)
}
