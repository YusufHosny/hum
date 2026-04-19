package network

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/pion/randutil"
	"github.com/pion/webrtc/v4"
)

// ---------------  types ---------------
type MeshContext interface {
	removeMember(member *MeshMember)

	sendOffer(member *MeshMember) error
	sendAnswer(member *MeshMember) error
	sendCandidate(member *MeshMember, candidate *webrtc.ICECandidate) error
}

// a single p2p connection to another user
// handles sending SDP/ICE, politeness, and communication
type MeshMember struct {
	context MeshContext

	username   string // peer username
	connection *webrtc.PeerConnection

	candidatesMux     sync.Mutex
	pendingCandidates []*webrtc.ICECandidate

	remoteCandidatesMux     sync.Mutex
	pendingRemoteCandidates []webrtc.ICECandidateInit
}

// ---------------  functions ---------------

func (member *MeshMember) Close() error {
	member.context.removeMember(member)

	if member.connection.ConnectionState() != webrtc.PeerConnectionStateClosed {
		err := member.connection.Close()
		if err != nil {
			log.Printf("Failed to close connection: %v\n", err)
			return err
		}
	}

	return nil
}

func (member *MeshMember) sendPendingCandidates() {
	member.candidatesMux.Lock()
	defer member.candidatesMux.Unlock()

	for i, candidate := range member.pendingCandidates {
		if err := member.context.sendCandidate(member, candidate); err != nil {
			log.Printf("Failed to send pending candidate: %v\n", err)
			member.pendingCandidates = member.pendingCandidates[i:]
			return
		}
	}
	member.pendingCandidates = nil
}

func (member *MeshMember) processPendingRemoteCandidates() {
	member.remoteCandidatesMux.Lock()
	defer member.remoteCandidatesMux.Unlock()

	for _, candidate := range member.pendingRemoteCandidates {
		if err := member.connection.AddICECandidate(candidate); err != nil {
			log.Printf("Failed to add pending remote ICE Candidate: %v\n", err)
		}
	}
	member.pendingRemoteCandidates = nil
}

// ---------------  callbacks/handlers ---------------

func (member *MeshMember) handleOffer(sdp string) {
	offer := webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: sdp}
	if err := member.connection.SetRemoteDescription(offer); err != nil {
		log.Printf("Failed to handle SDP offer: %v\n", err)
		return
	}
	member.processPendingRemoteCandidates()
	member.sendPendingCandidates()
	member.context.sendAnswer(member)
}

func (member *MeshMember) handleAnswer(sdp string) {
	answer := webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: sdp}
	if err := member.connection.SetRemoteDescription(answer); err != nil {
		log.Printf("Failed to handle SDP Answer: %v\n", err)
		return
	}
	member.processPendingRemoteCandidates()
	member.sendPendingCandidates()
}

func (member *MeshMember) handleCandidate(candidateValue []byte) {
	var candidate webrtc.ICECandidateInit
	err := json.Unmarshal(candidateValue, &candidate)
	if err != nil {
		log.Printf("Failed to parse ICE candidate: %v\n", err)
		return
	}

	if member.connection.RemoteDescription() == nil {
		member.remoteCandidatesMux.Lock()
		member.pendingRemoteCandidates = append(member.pendingRemoteCandidates, candidate)
		member.remoteCandidatesMux.Unlock()
		return
	}

	err = member.connection.AddICECandidate(candidate)
	if err != nil {
		log.Printf("Failed to handle ICE Candidate: %v\n", err)
	}
}

func (member *MeshMember) onICECandidate(candidate *webrtc.ICECandidate) {
	if candidate == nil {
		return
	}
	member.candidatesMux.Lock()
	defer member.candidatesMux.Unlock()

	desc := member.connection.RemoteDescription()
	if desc == nil {
		member.pendingCandidates = append(member.pendingCandidates, candidate)
		return
	}

	err := member.context.sendCandidate(member, candidate)
	if err != nil {
		log.Printf("OnICECandidate Error: %v\n", err)
		member.pendingCandidates = append(member.pendingCandidates, candidate)
	}
}

func (member *MeshMember) onConnectionStateChange(state webrtc.PeerConnectionState) {
	if state == webrtc.PeerConnectionStateFailed || state == webrtc.PeerConnectionStateClosed {
		go member.Close()
		log.Println("Peer Connection closed or failed")
	}
}

func (member *MeshMember) onDataChannel(dataChannel *webrtc.DataChannel) {
	log.Printf("New DataChannel %s %d\n", dataChannel.Label(), dataChannel.ID())
	dataChannel.OnOpen(func() {
		log.Printf(
			"Data channel '%s'-'%d' open. Random messages will now be sent to any connected DataChannels every 5 seconds\n",
			dataChannel.Label(), dataChannel.ID(),
		)

		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			message, sendTextErr := randutil.GenerateCryptoRandomString(
				15, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ",
			)
			if sendTextErr != nil {
				log.Printf("Failed to send data: %v\n", sendTextErr)
				continue
			}

			log.Printf("Sending '%s'\n", message)
			if sendTextErr = dataChannel.SendText(message); sendTextErr != nil {
				log.Printf("Failed to send data: %v\n", sendTextErr)
				continue
			}
		}
	})

	dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
		log.Printf("Message from DataChannel '%s': '%s'\n", dataChannel.Label(), string(msg.Data))
	})
}
