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

	"github.com/YusufHosny/hum/internal/audio"
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

	chatManager  *chat.ChatManager
	audioManager *audio.AudioManager

	username    string // self username
	channelName string

	webrtcConfig webrtc.Configuration
	webrtcAPI    *webrtc.API

	membersMux sync.Mutex
	members    []*MeshMember
	closeOnce  sync.Once
}

func NewMeshManager(
	ctx context.Context,
	signalingServerUrl url.URL,
	username string,
	channelName string,
	chatManager *chat.ChatManager,
	audioManager *audio.AudioManager,
) (*MeshManager, error) {
	ctx, cancel := context.WithCancel(ctx)

	manager := &MeshManager{
		ctx:                ctx,
		cancel:             cancel,
		signalingServerUrl: signalingServerUrl,

		chatManager:  chatManager,
		audioManager: audioManager,

		username:    username,
		channelName: channelName,
		members:     make([]*MeshMember, 0),
	}
	err := manager.initWebRTC()
	if err != nil {
		log.Printf("Failed to initialize mesh manager webrtc: %v", err)
		return nil, err
	}

	go manager.websocketLoop()
	go manager.senderLoop()
	go func() {
		<-ctx.Done()
		manager.Close()
	}()

	return manager, nil
}

func (manager *MeshManager) senderLoop() {
	for {
		manager.membersMux.Lock()
		currentMembers := slices.Clone(manager.members)
		manager.membersMux.Unlock()

		select {
		case <-manager.ctx.Done():
			return
		case envelope := <-manager.chatManager.GetOutbox():
			for _, member := range currentMembers {
				go member.sendChatEnvelope(envelope) // TODO: retry on errors?
			}
		case envelope := <-manager.audioManager.GetOutbox():
			for _, member := range currentMembers {
				go member.sendAudioEnvelope(envelope) // TODO: retry on errors?
			}
		}
	}
}

func (manager *MeshManager) newMember(username string) (*MeshMember, error) {
	peerConnection, err := manager.webrtcAPI.NewPeerConnection(manager.webrtcConfig)
	if err != nil {
		return nil, err
	}

	memberCtx, memberCancel := context.WithCancel(manager.ctx)

	member := &MeshMember{
		meshContext: manager,
		ctx:         memberCtx,
		cancel:      memberCancel,
		done:        make(chan struct{}),

		username:   username,
		connection: peerConnection,

		dataChannels:      make([]*webrtc.DataChannel, 0),
		pendingCandidates: make([]*webrtc.ICECandidate, 0),
	}

	manager.membersMux.Lock()
	manager.members = append(manager.members, member)
	manager.membersMux.Unlock()

	err = member.initWebRTC()
	if err != nil {
		log.Printf("Failed to initialize webrtc: %v\n", err)
		return nil, err
	}

	if manager.shouldSendOffer(member) {
		err := manager.setupDatachannels(member)
		if err != nil {
			log.Printf("Failed to setup datachannels: %v\n", err)
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

func (manager *MeshManager) acceptChat(ce *chat.ChatEnvelope) {
	manager.chatManager.GetInbox() <- ce
}

func (manager *MeshManager) acceptAudio(ae *audio.AudioEnvelope) {
	manager.audioManager.GetInbox() <- ae
}
