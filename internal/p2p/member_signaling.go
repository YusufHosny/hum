package p2p

import (
	"encoding/json"
	"log"

	"github.com/pion/webrtc/v4"
)

func (member *MeshMember) handleOffer(sdp string) {
	offer := webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: sdp}
	if err := member.connection.SetRemoteDescription(offer); err != nil {
		log.Printf("Failed to handle SDP offer: %v\n", err)
		return
	}
	member.processPendingRemoteCandidates()
	member.sendPendingCandidates()
	member.meshContext.sendAnswer(member)
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

func (member *MeshMember) sendPendingCandidates() {
	member.candidatesMux.Lock()
	defer member.candidatesMux.Unlock()

	for i, candidate := range member.pendingCandidates {
		if err := member.meshContext.sendCandidate(member, candidate); err != nil {
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
