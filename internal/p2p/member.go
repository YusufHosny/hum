package p2p

import (
	"context"
	"log"
	"sync"

	"github.com/YusufHosny/hum/internal/chat"
	"github.com/pion/webrtc/v4"
)

// ---------------  types ---------------
type MeshContext interface {
	removeMember(member *MeshMember)

	sendOffer(member *MeshMember) error
	sendAnswer(member *MeshMember) error
	sendCandidate(member *MeshMember, candidate *webrtc.ICECandidate) error

	getInbox() chan<- *chat.ChatEnvelope
}

// a single p2p connection to another user
// handles sending SDP/ICE, politeness, and communication
type MeshMember struct {
	meshContext MeshContext
	ctx         context.Context
	cancel      context.CancelFunc

	username   string // peer username
	connection *webrtc.PeerConnection

	dataChannelsMux sync.Mutex
	dataChannels    []*webrtc.DataChannel

	candidatesMux     sync.Mutex
	pendingCandidates []*webrtc.ICECandidate

	remoteCandidatesMux     sync.Mutex
	pendingRemoteCandidates []webrtc.ICECandidateInit

	done      chan struct{}
	closeOnce sync.Once
}

// ---------------  functions ---------------

func (member *MeshMember) Close() error {
	member.closeOnce.Do(func() {
		if member.cancel != nil {
			member.cancel()
		}
		close(member.done)
	})

	member.meshContext.removeMember(member)

	if member.connection.ConnectionState() != webrtc.PeerConnectionStateClosed {
		err := member.connection.Close()
		if err != nil {
			log.Printf("Failed to close connection: %v\n", err)
			return err
		}
	}

	return nil
}
