package p2p

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/gorilla/websocket"
)

type SignalingMessage struct {
	Type string `json:"type"`
	From string `json:"from"`
	To   string `json:"to"`

	SDP        string   `json:"sdp,omitempty"`
	Candidate  []byte   `json:"candidate,omitempty"`
	MemberList []string `json:"memberList,omitempty"`
	Error      string   `json:"error,omitempty"`
}

func (manager *MeshManager) websocketLoop() {
	backoff := time.Second

	for {
		select {
		case <-manager.ctx.Done():
			manager.logger.Println("wsListener stopped: context canceled")
			return
		default:
			err := manager.connectAndListen()
			if err != nil {
				if errors.Is(manager.ctx.Err(), context.Canceled) {
					return
				}
				manager.logger.Printf("WS Connection lost: %v. Retrying in %v...", err, backoff)

				timer := time.NewTimer(backoff)
				select {
				case <-manager.ctx.Done():
					timer.Stop()
					return
				case <-timer.C:
				}

				backoff *= 2
				if backoff > 1*time.Minute {
					backoff = 1 * time.Minute
				}
				continue
			}
		}
	}
}

func (manager *MeshManager) connectAndListen() error {
	conn, _, err := websocket.DefaultDialer.DialContext(manager.ctx, manager.signalingServerUrl.String(), nil)
	if err != nil {
		return err
	}

	manager.wsMux.Lock()
	if manager.ctx.Err() != nil {
		manager.wsMux.Unlock()
		conn.Close()
		return manager.ctx.Err()
	}
	manager.ws = conn
	manager.wsMux.Unlock()

	defer func() {
		manager.wsMux.Lock()
		manager.ws.Close()
		manager.wsMux.Unlock()
	}()

	for {
		messageType, messageContent, err := manager.ws.ReadMessage()
		if err != nil {
			return err
		}
		manager.handleWsMessage(messageType, messageContent)
	}
}

func (manager *MeshManager) handleWsMessage(messageType int, messageContent []byte) {
	if messageType != websocket.TextMessage {
		return
	}

	var msg SignalingMessage
	if err := json.Unmarshal(messageContent, &msg); err != nil {
		manager.logger.Printf("Failed to parse signaling message: %v\n", err)
		return
	}

	if msg.From == manager.username {
		return
	}

	switch msg.Type {
	case "error":
		manager.logger.Printf("Signaling Server Error: %v\n", msg.Error)

	case "member-list":
		manager.handleMemberList(msg.MemberList)

	case "peer-joined":
		_, err := manager.getOrCreateMemberByName(msg.From)
		if err != nil {
			manager.logger.Printf("Failed to handle %s: %v\n", msg.Type, err)
			return
		}

	case "peer-left":
		if m, found := manager.getMemberByName(msg.From); found {
			m.Close()
		} else {
			manager.logger.Printf("Peer left but no member found for %s\n", msg.From)
		}

	case "offer", "answer", "candidate":
		member, err := manager.getOrCreateMemberByName(msg.From)
		if err != nil {
			manager.logger.Printf("Failed to handle %s: %v\n", msg.Type, err)
			return
		}

		if msg.Type == "offer" {
			member.handleOffer(msg.SDP)
		} else if msg.Type == "answer" {
			member.handleAnswer(msg.SDP)
		} else {
			member.handleCandidate(msg.Candidate)
		}

	default:
		manager.logger.Printf("Unknown signaling message type: %s\n", msg.Type)
	}
}

func (manager *MeshManager) handleMemberList(memberList []string) {
	for _, memberName := range memberList {
		if memberName != manager.username {
			manager.getOrCreateMemberByName(memberName)
		}
	}
}

func (manager *MeshManager) sendSignalingMessage(message SignalingMessage) error {
	payload, err := json.Marshal(message)
	if err != nil {
		return err
	}

	manager.wsMux.Lock()
	defer manager.wsMux.Unlock()

	if manager.ws == nil {
		return errors.New("websocket is not connected")
	}

	err = manager.ws.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err != nil {
		return err
	}
	err = manager.ws.WriteMessage(websocket.TextMessage, payload)

	manager.ws.SetWriteDeadline(time.Time{})
	return err
}
