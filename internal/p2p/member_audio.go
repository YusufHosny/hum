package p2p

import (
	"crypto/rand"
	"encoding/binary"
	"log"
	"time"

	"github.com/YusufHosny/hum/internal/audio"
	"github.com/pion/interceptor/pkg/jitterbuffer"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
)

func (member *MeshMember) setupOutboundAudio() error {
	audioCapability := webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus}

	trackLocal, err := webrtc.NewTrackLocalStaticRTP(audioCapability, "audio", member.username)
	if err != nil {
		return err
	}
	member.sendTrack = trackLocal
	member.sequencer = rtp.NewRandomSequencer()

	err = binary.Read(rand.Reader, binary.LittleEndian, &member.sendSSRC)
	if err != nil {
		return err
	}

	rtpSender, err := member.connection.AddTrack(trackLocal)
	if err != nil {
		return err
	}

	go member.rtcpSenderLoop(rtpSender)
	return nil
}

func (member *MeshMember) rtcpSenderLoop(rtpSender *webrtc.RTPSender) {
	buf := make([]byte, 1500)
	for {
		// TODO: maybe handle the rtcp packets for stuff interceptors dont do:
		// - "signal/connection quality/ping" type ui?
		// - adaptive bitrate?
		if _, _, err := rtpSender.Read(buf); err != nil {
			log.Printf("RTCP read error: %v", err)
			return
		}
	}
}

func (member *MeshMember) rtcpReceiverLoop(rtpReceiver *webrtc.RTPReceiver) {
	buf := make([]byte, 1500)
	for {
		// TODO: maybe handle the rtcp packets if needed here?
		if _, _, err := rtpReceiver.Read(buf); err != nil {
			log.Printf("RTCP read error: %v", err)
			return
		}
	}
}

func (member *MeshMember) rtpReceiverLoop(remoteTrack *webrtc.TrackRemote) {
	jb := jitterbuffer.New()
	go func() {
		for {
			rtpPacket, _, err := remoteTrack.ReadRTP()
			if err != nil {
				return
			}
			jb.Push(rtpPacket)
		}
	}()
	go func() {
		for {
			rtpPacket, err := jb.Pop()
			if err != nil {
				return
			}
			if rtpPacket == nil {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			ae := audio.MakeAudioEnvelope(rtpPacket.Payload)
			member.meshContext.acceptAudio(ae)
		}
	}()
}

func (member *MeshMember) sendAudioEnvelope(ae *audio.AudioEnvelope) {
	seq := member.sequencer.NextSequenceNumber()

	pkt := &rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			PayloadType:    111, // opus payloadtype
			SequenceNumber: seq,
			Timestamp:      uint32(time.Now().UnixNano()),
			SSRC:           member.sendSSRC,
		},
		Payload: ae.Content,
	}
	err := member.sendTrack.WriteRTP(pkt)
	if err != nil {
		log.Printf("Failed to send audio packet: %v", err)
	}
}
