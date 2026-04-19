package p2p

import (
	"fmt"
	"log"
	"slices"

	"github.com/pion/webrtc/v4"
)

// handles when a candidate is produced/found by pion locally
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

	err := member.meshContext.sendCandidate(member, candidate)
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
	label := dataChannel.Label()
	if label != "chat-text" && label != "chat-metadata" {
		panic(fmt.Errorf("Unexpected data channel label: %v\n", label))
	}

	dataChannel.OnOpen(func() {
		if index := slices.IndexFunc(member.dataChannels, func(dc *webrtc.DataChannel) bool { return dc.Label() == label }); index != -1 {
			member.dataChannels[index] = dataChannel
		} else {
			member.dataChannels = append(member.dataChannels, dataChannel)
		}
	})

	dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
		member.chatPipe.Receive(msg.Data, "message")
	})
}

var DataChannelLabelMap = map[string]string{
	"message":  "chat-text",
	"metadata": "chat-metadata",
}

func (member *MeshMember) findDataChannel(label string) (*webrtc.DataChannel, bool) {
	for _, dc := range member.dataChannels {
		if dc.Label() == label {
			return dc, true
		}
	}
	return nil, false
}

func (member *MeshMember) createDataChannel(label string, options *webrtc.DataChannelInit) (*webrtc.DataChannel, error) {
	dc, err := member.connection.CreateDataChannel(label, options)
	if err != nil {
		return nil, err
	}
	member.onDataChannel(dc)
	return dc, nil
}
