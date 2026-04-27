package p2p

import (
	"context"
	"sync"

	"github.com/YusufHosny/hum/internal/audio"
	"github.com/YusufHosny/hum/internal/chat"
	"github.com/YusufHosny/hum/internal/logger"
	"github.com/pion/webrtc/v4"
)

type MeshContext interface {
	removeMember(member *MeshMember)

	sendOffer(member *MeshMember) error
	sendAnswer(member *MeshMember) error
	sendCandidate(member *MeshMember, candidate *webrtc.ICECandidate) error

	acceptChat(ce *chat.ChatEnvelope)
	acceptAudio(ae *audio.AudioEnvelope)

	Logger() logger.Logger
}

// single p2p peer connection
type MeshMember struct {
	meshContext MeshContext
	ctx         context.Context
	cancel      context.CancelFunc

	username   string // peer username
	connection *webrtc.PeerConnection

	dataChannelsMux sync.Mutex
	dataChannels    []*webrtc.DataChannel

	sendTrack *webrtc.TrackLocalStaticSample

	candidatesMux     sync.Mutex
	pendingCandidates []*webrtc.ICECandidate

	remoteCandidatesMux     sync.Mutex
	pendingRemoteCandidates []webrtc.ICECandidateInit

	done      chan struct{}
	closeOnce sync.Once
}

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
			member.meshContext.Logger().Printf("Failed to close connection: %v\n", err)
			return err
		}
	}

	return nil
}
