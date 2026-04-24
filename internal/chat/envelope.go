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
	Type string `json:"type"` // "typing"
}

func newMessageEnvelope(from string, content []byte) *ChatEnvelope {
	return &ChatEnvelope{
		Type:      "message",
		From:      from,
		Content:   content,
		CreatedAt: time.Now(),
	}
}

func isValidMessageType(datachannel string) bool {
	return datachannel == "message" || datachannel == "metadata"
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
