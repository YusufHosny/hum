package p2p

import (
	"encoding/json"
	"log"

	"github.com/pion/interceptor"
	"github.com/pion/webrtc/v4"
)

func (manager *MeshManager) initWebRTC() error {
	manager.webrtcConfig = webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	interceptors := &interceptor.Registry{}
	if err := webrtc.RegisterDefaultInterceptors(&webrtc.MediaEngine{}, interceptors); err != nil {
		return err
	}
	manager.webrtcAPI = webrtc.NewAPI(webrtc.WithInterceptorRegistry(interceptors))
	return nil
}

func (manager *MeshManager) setupDatachannels(member *MeshMember) error {
	_, err := member.createDataChannel("chat-text", nil)
	if err != nil {
		return err
	}

	_, err = member.createDataChannel("chat-metadata", &webrtc.DataChannelInit{Ordered: new(false)})
	if err != nil {
		log.Printf("Failed to create datachannel: %v\n", err)
		return err
	}

	err = manager.sendOffer(member)
	if err != nil {
		log.Printf("Failed to send offer: %v\n", err)
		member.Close()
		return err
	}

	return nil
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
