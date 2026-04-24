package p2p

import (
	"encoding/json"

	"github.com/pion/interceptor"
	"github.com/pion/webrtc/v4"
)

func (manager *MeshManager) initWebRTC() {
	manager.webrtcConfig = webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}
	interceptors := interceptor.Registry{}
	/* interceptors.Add(&EncryptInterceptor{
		username:    manager.username,
		channelName: manager.channelName,
	}) */
	manager.webrtcAPI = webrtc.NewAPI(webrtc.WithInterceptorRegistry(&interceptors))
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

// decides which user will stand down in case of offer collision (politeness)
// assuming signaling server handles username uniqueness per channel,
// politness is just alphabetically priority
func (manager *MeshManager) shouldSendOffer(member *MeshMember) bool {
	return manager.username < member.username
}
