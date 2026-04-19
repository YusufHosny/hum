package network

import (
	"encoding/json"
	"errors"
	"log"
	"net/url"
	"slices"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/pion/webrtc/v4"
)

// ---------------  types ---------------

// manages and orchestrates all communication
// handles signaling and creating individual member p2p connections
type MeshManager struct {
	signalingServerUrl url.URL
	wsMux              sync.Mutex
	ws                 *websocket.Conn

	username    string // self username
	channelName string

	webrtcConfig webrtc.Configuration
	membersMux   sync.Mutex
	members      []*MeshMember
}

type SignalingMessage struct {
	Type string `json:"type"` // "offer", "answer", "candidate", "peer-joined", "peer-left", "broadcast", channel-list", "error"
	From string `json:"from"` // from username attached locally, then checked on signaling server to ensure authenticity
	To   string `json:"to"`

	SDP        string   `json:"sdp,omitempty"`
	Candidate  []byte   `json:"candidate,omitempty"`
	MemberList []string `json:"memberList,omitempty"`
	Error      string   `json:"error,omitempty"`
}

// ---------------  MeshManager Functions ---------------

func NewMeshManager(
	signalingServerUrl url.URL,
	username string,
	channelName string,
) (*MeshManager, error) {
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	manager := &MeshManager{
		signalingServerUrl: signalingServerUrl,
		ws:                 nil,

		username:    username,
		channelName: channelName,

		webrtcConfig: config,
		members:      make([]*MeshMember, 0),
	}

	go manager.wsListener()

	return manager, nil
}

// TODO audit if this leaks/cant be stopped
func (manager *MeshManager) wsListener() {
	backoff := time.Second

	for {
		err := manager.connectAndListen()
		if err != nil {
			log.Printf("Connection lost: %v. Retrying in %v...", err, backoff)
			time.Sleep(backoff)

			backoff *= 2
			if backoff > 1*time.Minute {
				backoff = 1 * time.Minute
			}
			continue
		}
		break
	}
}

func (manager *MeshManager) connectAndListen() error {
	conn, _, err := websocket.DefaultDialer.Dial(manager.signalingServerUrl.String(), nil)
	if err != nil {
		return err
	}

	manager.ws = conn
	defer manager.ws.Close()

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
		log.Printf("Failed to parse signaling message: %v\n", err)
		return
	}

	if msg.From == manager.username {
		return // ignore own messages
	}

	switch msg.Type {
	case "error":
		log.Printf("Signaling Server Error: %v\n", msg.Error)

	case "member-list":
		manager.handleMemberList(msg.MemberList)

	case "peer-joined":
		_, err := manager.getOrCreateMemberByName(msg.From)
		if err != nil {
			log.Printf("Failed to handle %s: %v\n", msg.Type, err)
			return
		}

	case "peer-left":
		if m, found := manager.getMemberByName(msg.From); found {
			m.Close()
		} else {
			log.Printf("Peer left but no member found for %s\n", msg.From)
		}

	case "offer", "answer", "candidate":
		member, err := manager.getOrCreateMemberByName(msg.From)
		if err != nil {
			log.Printf("Failed to handle %s: %v\n", msg.Type, err)
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
		log.Printf("Unknown message type: %s\n", msg.Type)
	}
}

func (manager *MeshManager) newMember(username string) (*MeshMember, error) {
	peerConnection, err := webrtc.NewPeerConnection(manager.webrtcConfig)
	if err != nil {
		return nil, err
	}

	member := &MeshMember{
		context:           manager,
		username:          username,
		connection:        peerConnection,
		pendingCandidates: make([]*webrtc.ICECandidate, 0),
	}

	manager.membersMux.Lock()
	defer manager.membersMux.Unlock()
	manager.members = append(manager.members, member)

	peerConnection.OnICECandidate(member.onICECandidate)
	peerConnection.OnConnectionStateChange(member.onConnectionStateChange)
	peerConnection.OnDataChannel(member.onDataChannel)

	if manager.shouldSendOffer(member) {
		dc, err := member.connection.CreateDataChannel("default", nil)
		if err != nil {
			log.Printf("Failed to create datachannel: %v\n", err)
			member.Close()
			return nil, err
		}
		member.onDataChannel(dc)

		err = manager.sendOffer(member)
		if err != nil {
			log.Printf("Failed to send offer: %v\n", err)
			member.Close()
			return nil, err
		}
	}

	return member, nil
}

func (manager *MeshManager) Close() error {
	manager.membersMux.Lock()
	currentMembers := append([]*MeshMember(nil), manager.members...)
	manager.membersMux.Unlock()

	errOccurred := false
	for _, member := range currentMembers {
		err := member.Close()
		if err != nil {
			log.Printf("Couldn't close peerConnection: %v\n", err)
			errOccurred = true
		}
	}

	manager.membersMux.Lock()
	manager.members = nil
	manager.membersMux.Unlock()

	if errOccurred {
		return errors.New("At least one peerConnection failed to close")
	}
	return nil
}

func (manager *MeshManager) removeMember(member *MeshMember) {
	manager.membersMux.Lock()
	defer manager.membersMux.Unlock()

	if idx := slices.Index(manager.members, member); idx != -1 {
		manager.members = slices.Delete(manager.members, idx, idx+1)
	}
}

func (manager *MeshManager) getMemberByName(name string) (*MeshMember, bool) {
	manager.membersMux.Lock()
	defer manager.membersMux.Unlock()

	for _, m := range manager.members {
		if m.username == name {
			return m, true
		}
	}
	return nil, false
}

func (manager *MeshManager) getOrCreateMemberByName(name string) (*MeshMember, error) {
	m, found := manager.getMemberByName(name)
	if !found {
		return manager.newMember(name)
	}
	return m, nil
}

func (manager *MeshManager) sendOffer(member *MeshMember) error {
	offer, err := member.connection.CreateOffer(nil)
	if err != nil {
		return err
	}

	if err = member.connection.SetLocalDescription(offer); err != nil {
		return err
	}

	return manager.sendSignalingMessage(SignalingMessage{
		Type: "offer",
		From: manager.username,
		To:   member.username,
		SDP:  offer.SDP,
	})
}

func (manager *MeshManager) sendAnswer(member *MeshMember) error {
	answer, err := member.connection.CreateAnswer(nil)
	if err != nil {
		return err
	}

	if err = member.connection.SetLocalDescription(answer); err != nil {
		return err
	}

	return manager.sendSignalingMessage(SignalingMessage{
		Type: "answer",
		From: manager.username,
		To:   member.username,
		SDP:  answer.SDP,
	})
}

func (manager *MeshManager) sendCandidate(member *MeshMember, candidate *webrtc.ICECandidate) error {
	candidateValue, err := json.Marshal(candidate.ToJSON())
	if err != nil {
		return err
	}

	return manager.sendSignalingMessage(SignalingMessage{
		Type:      "candidate",
		From:      manager.username,
		To:        member.username,
		Candidate: candidateValue,
	})
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

	return manager.ws.WriteMessage(
		websocket.TextMessage,
		payload,
	)
}

// decides which user will stand down in case of offer collision (politeness)
// assuming signaling server handles username uniqueness per channel,
// politness is just alphabetically priority
func (manager *MeshManager) shouldSendOffer(member *MeshMember) bool {
	return manager.username < member.username
}
