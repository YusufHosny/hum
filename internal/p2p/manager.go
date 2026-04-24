package p2p

import (
	"context"
	"errors"
	"log"
	"net/url"
	"slices"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"

	"github.com/YusufHosny/hum/internal/chat"
)

// manages and orchestrates all communication
// handles signaling and creating individual member p2p connections
type MeshManager struct {
	ctx    context.Context
	cancel context.CancelFunc

	signalingServerUrl url.URL
	wsMux              sync.Mutex
	ws                 *websocket.Conn

	inbox  chan<- *chat.ChatEnvelope
	outbox <-chan *chat.ChatEnvelope

	username    string // self username
	channelName string

	webrtcConfig webrtc.Configuration
	membersMux   sync.Mutex
	members      []*MeshMember
	closeOnce    sync.Once
}

func NewMeshManager(
	ctx context.Context,
	signalingServerUrl url.URL,
	username string,
	channelName string,
	chatManager *chat.ChatManager,
) (*MeshManager, error) {
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	ctx, cancel := context.WithCancel(ctx)
	inbox, outbox := chatManager.GetInboxOutbox()

	manager := &MeshManager{
		ctx:                ctx,
		cancel:             cancel,
		signalingServerUrl: signalingServerUrl,
		ws:                 nil,
		inbox:              inbox,
		outbox:             outbox,
		username:           username,
		channelName:        channelName,
		webrtcConfig:       config,
		members:            make([]*MeshMember, 0),
	}

	go manager.wsListener()
	go manager.senderLoop()
	go func() {
		<-ctx.Done()
		manager.Close()
	}()

	return manager, nil
}

func (manager *MeshManager) senderLoop() {
	for {
		select {
		case <-manager.ctx.Done():
			return
		case envelope := <-manager.outbox:
			manager.membersMux.Lock()
			currentMembers := slices.Clone(manager.members)
			manager.membersMux.Unlock()

			for _, member := range currentMembers {
				go member.sendChatEnvelope(envelope) // TODO: retry on errors?
			}
		}
	}
}

func (manager *MeshManager) newMember(username string) (*MeshMember, error) {
	peerConnection, err := webrtc.NewPeerConnection(manager.webrtcConfig)
	if err != nil {
		return nil, err
	}

	memberCtx, memberCancel := context.WithCancel(manager.ctx)

	member := &MeshMember{
		meshContext:       manager,
		ctx:               memberCtx,
		cancel:            memberCancel,
		username:          username,
		connection:        peerConnection,
		dataChannels:      make([]*webrtc.DataChannel, 0),
		pendingCandidates: make([]*webrtc.ICECandidate, 0),
		done:              make(chan struct{}),
	}

	manager.membersMux.Lock()
	defer manager.membersMux.Unlock()
	manager.members = append(manager.members, member)

	peerConnection.OnICECandidate(member.onICECandidate)
	peerConnection.OnConnectionStateChange(member.onConnectionStateChange)
	peerConnection.OnDataChannel(member.onDataChannel)

	if manager.shouldSendOffer(member) {
		_, err := member.createDataChannel("chat-text", nil)
		if err != nil {
			log.Printf("Failed to create datachannel: %v\n", err)
			return nil, err
		}

		_, err = member.createDataChannel("chat-metadata", &webrtc.DataChannelInit{Ordered: new(false)})
		if err != nil {
			log.Printf("Failed to create datachannel: %v\n", err)
			return nil, err
		}

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
	var returnErr error
	manager.closeOnce.Do(func() {
		manager.cancel()

		manager.membersMux.Lock()
		currentMembers := slices.Clone(manager.members)
		manager.membersMux.Unlock()

		errOccurred := false
		for _, member := range currentMembers {
			err := member.Close()
			if err != nil {
				log.Printf("Failed to close peerConnection: %v\n", err)
				errOccurred = true
			}
		}

		manager.membersMux.Lock()
		manager.members = nil
		manager.membersMux.Unlock()

		manager.wsMux.Lock()
		if manager.ws != nil {
			err := manager.ws.Close()
			if err != nil {
				log.Printf("Failed to close websocket: %v\n", err)
			}
		}
		manager.wsMux.Unlock()

		if errOccurred {
			returnErr = errors.New("Failed to close at least one peerConnection")
		}
	})
	return returnErr
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

func (manager *MeshManager) getInbox() chan<- *chat.ChatEnvelope {
	return manager.inbox
}
